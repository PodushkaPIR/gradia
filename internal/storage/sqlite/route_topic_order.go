package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) SwapRouteTopics(ctx context.Context, routeID domain.RouteID, first, second domain.RouteTopicID, at time.Time) ([]domain.RouteTopicPosition, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var firstParent, secondParent sql.NullInt64
	var firstPosition, secondPosition int
	for _, item := range []struct {
		id       domain.RouteTopicID
		parent   *sql.NullInt64
		position *int
	}{{first, &firstParent, &firstPosition}, {second, &secondParent, &secondPosition}} {
		if err := tx.QueryRowContext(ctx, `SELECT parent_id, position FROM route_topics WHERE route_id = ? AND id = ?`, routeID, item.id).Scan(item.parent, item.position); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, domain.ErrRouteTopicNotFound
			}
			return nil, err
		}
	}
	if firstParent != secondParent {
		return nil, domain.ErrTopicsNotSiblings
	}
	if first == second {
		return []domain.RouteTopicPosition{{TopicID: first, Position: firstPosition}}, tx.Commit()
	}
	// Use a free nonnegative slot so sibling-position uniqueness holds after every UPDATE.
	var temporaryPosition int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) + 1 FROM route_topics WHERE route_id = ? AND parent_id IS ?`, routeID, firstParent).Scan(&temporaryPosition); err != nil {
		return nil, err
	}
	for _, move := range []struct {
		id       domain.RouteTopicID
		position int
	}{{first, temporaryPosition}, {second, firstPosition}, {first, secondPosition}} {
		if _, err := tx.ExecContext(ctx, `UPDATE route_topics SET position = ?, updated_at = ? WHERE route_id = ? AND id = ?`, move.position, at.UnixMilli(), routeID, move.id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return []domain.RouteTopicPosition{{TopicID: first, Position: secondPosition}, {TopicID: second, Position: firstPosition}}, nil
}
