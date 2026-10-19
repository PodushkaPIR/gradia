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
	return s.changeRouteTopicProgress(ctx, routeID, topicID, &status, nil, at)
}

func (s *Store) SetRouteTopicRevision(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, revision bool, at time.Time) error {
	return s.changeRouteTopicProgress(ctx, routeID, topicID, nil, &revision, at)
}

func (s *Store) changeRouteTopicProgress(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, requestedStatus *domain.ProgressStatus, requestedRevision *bool, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before domain.ProgressStatus
	var beforeRevision bool
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(p.status, 'not_started'), COALESCE(p.revision, 0) FROM route_topics t LEFT JOIN route_topic_progress p ON p.topic_id = t.id WHERE t.id = ? AND t.route_id = ?`, topicID, routeID).Scan(&before, &beforeRevision); err != nil {
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
	status, revision := before, beforeRevision
	if requestedStatus != nil {
		status = *requestedStatus
		if status != domain.ProgressMastered {
			revision = false
		}
	}
	if requestedRevision != nil {
		revision = *requestedRevision
	}
	if revision && status != domain.ProgressMastered {
		return domain.ErrRevisionRequiresMastered
	}
	if status == before && revision == beforeRevision {
		return tx.Commit()
	}
	if status == domain.ProgressNotStarted {
		_, err = tx.ExecContext(ctx, `DELETE FROM route_topic_progress WHERE topic_id = ?`, topicID)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO route_topic_progress (topic_id, status, revision, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT(topic_id) DO UPDATE SET status = excluded.status, revision = excluded.revision, updated_at = excluded.updated_at`, topicID, status, revision, at.UnixMilli())
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO route_progress_history (topic_id, before_status, after_status, before_revision, after_revision, created_at) VALUES (?, ?, ?, ?, ?, ?)`, topicID, before, status, beforeRevision, revision, at.UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListRouteTopicProgressHistory(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID) ([]domain.RouteProgressChange, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM route_topics WHERE id = ? AND route_id = ?`, topicID, routeID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRouteTopicNotFound
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, topic_id, before_status, after_status, before_revision, after_revision, created_at FROM route_progress_history WHERE topic_id = ? ORDER BY id`, topicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	changes := make([]domain.RouteProgressChange, 0)
	for rows.Next() {
		var change domain.RouteProgressChange
		var at int64
		if err := rows.Scan(&change.ID, &change.TopicID, &change.BeforeStatus, &change.AfterStatus, &change.BeforeRevision, &change.AfterRevision, &at); err != nil {
			return nil, err
		}
		change.CreatedAt = time.UnixMilli(at).UTC()
		changes = append(changes, change)
	}
	return changes, rows.Err()
}
