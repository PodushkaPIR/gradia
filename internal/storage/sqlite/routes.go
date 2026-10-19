package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	driver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) ListStudentRoutes(ctx context.Context, id domain.StudentID) ([]domain.Route, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, student_id, source_template_id, name, description_md, created_at, updated_at FROM student_routes WHERE student_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes := make([]domain.Route, 0)
	for rows.Next() {
		route, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (s *Store) ListRouteTopics(ctx context.Context, id domain.RouteID) ([]domain.RouteTopic, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM student_routes WHERE id = ?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRouteNotFound
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.route_id, t.parent_id, t.name, t.position, t.created_at, t.updated_at, COALESCE(p.status, 'not_started'), t.notes_md, COALESCE(p.revision, 0), t.excluded FROM route_topics t LEFT JOIN route_topic_progress p ON p.topic_id = t.id WHERE t.route_id = ? ORDER BY t.parent_id, t.position, t.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	topics := make([]domain.RouteTopic, 0)
	for rows.Next() {
		var topic domain.RouteTopic
		var parent sql.NullInt64
		var created, updated int64
		if err := rows.Scan(&topic.ID, &topic.RouteID, &parent, &topic.Name, &topic.Position, &created, &updated, &topic.Status, &topic.Notes, &topic.Revision, &topic.Excluded); err != nil {
			return nil, err
		}
		if parent.Valid {
			id := domain.RouteTopicID(parent.Int64)
			topic.ParentID = &id
		}
		topic.CreatedAt, topic.UpdatedAt = time.UnixMilli(created).UTC(), time.UnixMilli(updated).UTC()
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}

func (s *Store) CreateRouteTopic(ctx context.Context, topic domain.RouteTopic) (domain.RouteTopic, error) {
	var parent any
	if topic.ParentID != nil {
		parent = int64(*topic.ParentID)
	}
	var sqliteErr *driver.Error
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO route_topics (route_id, parent_id, name, position, created_at, updated_at)
		SELECT ?, ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?
		FROM route_topics WHERE route_id = ? AND parent_id IS ?
		RETURNING id, position
	`, topic.RouteID, parent, topic.Name, topic.CreatedAt.UnixMilli(), topic.UpdatedAt.UnixMilli(), topic.RouteID, parent).Scan(&topic.ID, &topic.Position)
	if err != nil {
		if errors.As(err, &sqliteErr) {
			switch sqliteErr.Code() {
			case sqlite3.SQLITE_CONSTRAINT_TRIGGER:
				return domain.RouteTopic{}, domain.ErrTopicDepthExceeded
			case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
				if topic.ParentID != nil {
					return domain.RouteTopic{}, domain.ErrRouteTopicNotFound
				}
				return domain.RouteTopic{}, domain.ErrRouteNotFound
			}
		}
		return domain.RouteTopic{}, err
	}
	return topic, nil
}

func (s *Store) RenameRouteTopic(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, name string, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE route_topics SET name = ?, updated_at = ? WHERE route_id = ? AND id = ?`, name, at.UnixMilli(), routeID, topicID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrRouteTopicNotFound
	}
	return nil
}

func (s *Store) CreateRouteFromTemplate(ctx context.Context, student domain.StudentID, templateID domain.TemplateID, at time.Time) (domain.Route, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Route{}, err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM students WHERE id = ?`, student).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Route{}, domain.ErrStudentNotFound
		}
		return domain.Route{}, err
	}
	template, err := scanTemplate(tx.QueryRowContext(ctx, `SELECT id, name, description_md, created_at, updated_at FROM templates WHERE id = ?`, templateID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Route{}, domain.ErrTemplateNotFound
		}
		return domain.Route{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, template_id, parent_id, name, position, created_at, updated_at FROM template_topics WHERE template_id = ? ORDER BY id`, templateID)
	if err != nil {
		return domain.Route{}, err
	}
	var topics []domain.TemplateTopic
	for rows.Next() {
		topic, err := scanTemplateTopic(rows)
		if err != nil {
			rows.Close()
			return domain.Route{}, err
		}
		topics = append(topics, topic)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return domain.Route{}, err
	}
	if closeErr != nil {
		return domain.Route{}, closeErr
	}
	// Сначала родители: ID исходных тем не определяют их положение в дереве.
	depths := make(map[domain.TemplateTopicID]int, len(topics))
	for _, topic := range topics {
		path, err := domain.TemplateTopicPath(topics, topic.ID)
		if err != nil {
			return domain.Route{}, err
		}
		depths[topic.ID] = len(path)
	}
	sort.Slice(topics, func(i, j int) bool {
		if depths[topics[i].ID] != depths[topics[j].ID] {
			return depths[topics[i].ID] < depths[topics[j].ID]
		}
		return topics[i].ID < topics[j].ID
	})
	result, err := tx.ExecContext(ctx, `INSERT INTO student_routes (student_id, source_template_id, name, description_md, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, student, templateID, template.Name, template.Description, at.UnixMilli(), at.UnixMilli())
	if err != nil {
		return domain.Route{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Route{}, err
	}
	route := domain.Route{ID: domain.RouteID(id), StudentID: student, SourceTemplateID: &templateID, Name: template.Name, Description: template.Description, CreatedAt: at, UpdatedAt: at}
	ids := make(map[domain.TemplateTopicID]domain.RouteTopicID, len(topics))
	for _, topic := range topics {
		var parent any
		if topic.ParentID != nil {
			parent = int64(ids[*topic.ParentID])
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO route_topics (route_id, parent_id, name, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, route.ID, parent, topic.Name, topic.Position, at.UnixMilli(), at.UnixMilli())
		if err != nil {
			return domain.Route{}, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return domain.Route{}, err
		}
		ids[topic.ID] = domain.RouteTopicID(id)
	}
	if err := tx.Commit(); err != nil {
		return domain.Route{}, err
	}
	return route, nil
}
