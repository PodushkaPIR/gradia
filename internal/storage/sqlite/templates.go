package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) ListTemplates(ctx context.Context) ([]domain.Template, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description_md, created_at, updated_at
		FROM templates ORDER BY name, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	templates := make([]domain.Template, 0)
	for rows.Next() {
		template, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, rows.Err()
}

func (s *Store) CreateTemplate(ctx context.Context, template domain.Template) (domain.Template, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO templates (name, description_md, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, template.Name, template.Description, template.CreatedAt.UnixMilli(), template.UpdatedAt.UnixMilli())
	if err != nil {
		return domain.Template{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Template{}, err
	}
	template.ID = domain.TemplateID(id)
	return template, nil
}

func (s *Store) RenameTemplate(ctx context.Context, id domain.TemplateID, name string, updatedAt time.Time) (domain.Template, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE templates SET name = ?, updated_at = ? WHERE id = ?
		RETURNING id, name, description_md, created_at, updated_at
	`, name, updatedAt.UnixMilli(), id)
	template, err := scanTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Template{}, domain.ErrTemplateNotFound
	}
	return template, err
}

func scanTemplate(row interface{ Scan(...any) error }) (domain.Template, error) {
	var template domain.Template
	var created, updated int64
	if err := row.Scan(&template.ID, &template.Name, &template.Description, &created, &updated); err != nil {
		return domain.Template{}, err
	}
	template.CreatedAt = time.UnixMilli(created).UTC()
	template.UpdatedAt = time.UnixMilli(updated).UTC()
	return template, nil
}
