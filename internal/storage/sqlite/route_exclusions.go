package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) SetRouteTopicExcluded(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, excluded bool, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before bool
	if err := tx.QueryRowContext(ctx, `SELECT excluded FROM route_topics WHERE id = ? AND route_id = ?`, topicID, routeID).Scan(&before); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrRouteTopicNotFound
		}
		return err
	}
	if before == excluded {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE route_topics SET excluded = ?, updated_at = ? WHERE id = ? AND route_id = ?`, excluded, at.UnixMilli(), topicID, routeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO route_topic_exclusion_history (topic_id, before_excluded, after_excluded, created_at) VALUES (?, ?, ?, ?)`, topicID, before, excluded, at.UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListRouteTopicExclusionHistory(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID) ([]domain.RouteTopicExclusionChange, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM route_topics WHERE id = ? AND route_id = ?`, topicID, routeID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRouteTopicNotFound
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, topic_id, before_excluded, after_excluded, created_at FROM route_topic_exclusion_history WHERE topic_id = ? ORDER BY id`, topicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	changes := make([]domain.RouteTopicExclusionChange, 0)
	for rows.Next() {
		var change domain.RouteTopicExclusionChange
		var at int64
		if err := rows.Scan(&change.ID, &change.TopicID, &change.BeforeExcluded, &change.AfterExcluded, &at); err != nil {
			return nil, err
		}
		change.CreatedAt = time.UnixMilli(at).UTC()
		changes = append(changes, change)
	}
	return changes, rows.Err()
}
