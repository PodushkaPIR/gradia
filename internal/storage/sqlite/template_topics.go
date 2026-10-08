package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	driver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) ListTemplateTopics(ctx context.Context, id domain.TemplateID) ([]domain.TemplateTopic, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, template_id, parent_id, name, position, created_at, updated_at
		FROM template_topics WHERE template_id = ? ORDER BY parent_id, position, id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	topics := make([]domain.TemplateTopic, 0)
	for rows.Next() {
		var topic domain.TemplateTopic
		var parent sql.NullInt64
		var created, updated int64
		if err := rows.Scan(&topic.ID, &topic.TemplateID, &parent, &topic.Name, &topic.Position, &created, &updated); err != nil {
			return nil, err
		}
		if parent.Valid {
			id := domain.TemplateTopicID(parent.Int64)
			topic.ParentID = &id
		}
		topic.CreatedAt = time.UnixMilli(created).UTC()
		topic.UpdatedAt = time.UnixMilli(updated).UTC()
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}

func (s *Store) CreateTemplateTopic(ctx context.Context, topic domain.TemplateTopic) (domain.TemplateTopic, error) {
	var parent any
	if topic.ParentID != nil {
		parent = int64(*topic.ParentID)
	}
	// Выбор следующей позиции и вставка — один атомарный запрос.
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO template_topics (template_id, parent_id, name, position, created_at, updated_at)
		SELECT ?, ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?
		FROM template_topics WHERE template_id = ? AND parent_id IS ?
		RETURNING id, position
	`, topic.TemplateID, parent, topic.Name, topic.CreatedAt.UnixMilli(), topic.UpdatedAt.UnixMilli(), topic.TemplateID, parent).Scan(&topic.ID, &topic.Position)
	if err != nil {
		var sqliteErr *driver.Error
		if errors.As(err, &sqliteErr) {
			switch sqliteErr.Code() {
			case sqlite3.SQLITE_CONSTRAINT_TRIGGER:
				return domain.TemplateTopic{}, domain.ErrTopicDepthExceeded
			case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
				if topic.ParentID != nil {
					return domain.TemplateTopic{}, domain.ErrTopicNotFound
				}
				return domain.TemplateTopic{}, domain.ErrTemplateNotFound
			}
		}
		return domain.TemplateTopic{}, err
	}
	return topic, nil
}
