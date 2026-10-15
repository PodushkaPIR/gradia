package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) SetRouteTopicStatus(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, status domain.ProgressStatus, at time.Time) error {
	if !status.Valid() {
		return domain.ErrInvalidProgressStatus
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM route_topics WHERE id = ? AND route_id = ?`, topicID, routeID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrRouteTopicNotFound
		}
		return err
	}
	var children int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM route_topics WHERE route_id = ? AND parent_id = ?`, routeID, topicID).Scan(&children); err != nil {
		return err
	}
	if children > 0 {
		return domain.ErrTopicHasSubtopics
	}
	if status == domain.ProgressNotStarted {
		_, err = tx.ExecContext(ctx, `DELETE FROM route_topic_progress WHERE topic_id = ?`, topicID)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO route_topic_progress (topic_id, status, updated_at) VALUES (?, ?, ?) ON CONFLICT(topic_id) DO UPDATE SET status = excluded.status, updated_at = excluded.updated_at`, topicID, status, at.UnixMilli())
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
