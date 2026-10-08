package sqlite

import (
	"context"
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
		var template domain.Template
		var created, updated int64
		if err := rows.Scan(&template.ID, &template.Name, &template.Description, &created, &updated); err != nil {
			return nil, err
		}
		template.CreatedAt = time.UnixMilli(created).UTC()
		template.UpdatedAt = time.UnixMilli(updated).UTC()
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
