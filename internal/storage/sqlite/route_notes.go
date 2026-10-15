package sqlite

import (
	"context"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) SetRouteTopicNote(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, markdown string, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE route_topics SET notes_md = ?, updated_at = ? WHERE route_id = ? AND id = ?`, markdown, at.UnixMilli(), routeID, topicID)
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
