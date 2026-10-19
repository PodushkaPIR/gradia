package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) RenameRoute(ctx context.Context, id domain.RouteID, name string, at time.Time) (domain.Route, error) {
	return scanRoute(s.db.QueryRowContext(ctx, `UPDATE student_routes SET name = ?, updated_at = ? WHERE id = ? RETURNING id, student_id, source_template_id, name, description_md, created_at, updated_at`, name, at.UnixMilli(), id))
}

func (s *Store) SetRouteDescription(ctx context.Context, id domain.RouteID, markdown string, at time.Time) (domain.Route, error) {
	return scanRoute(s.db.QueryRowContext(ctx, `UPDATE student_routes SET description_md = ?, updated_at = ? WHERE id = ? RETURNING id, student_id, source_template_id, name, description_md, created_at, updated_at`, markdown, at.UnixMilli(), id))
}

func scanRoute(row interface{ Scan(...any) error }) (domain.Route, error) {
	var route domain.Route
	var source sql.NullInt64
	var created, updated int64
	if err := row.Scan(&route.ID, &route.StudentID, &source, &route.Name, &route.Description, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Route{}, domain.ErrRouteNotFound
		}
		return domain.Route{}, err
	}
	if source.Valid {
		id := domain.TemplateID(source.Int64)
		route.SourceTemplateID = &id
	}
	route.CreatedAt, route.UpdatedAt = time.UnixMilli(created).UTC(), time.UnixMilli(updated).UTC()
	return route, nil
}
