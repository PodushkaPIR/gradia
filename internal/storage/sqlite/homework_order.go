package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) SwapHomeworkTasks(
	ctx context.Context,
	student domain.StudentID,
	homework domain.HomeworkID,
	first, second domain.HomeworkTaskID,
	at time.Time,
) ([]domain.HomeworkTaskPosition, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := checkHomework(ctx, tx, student, homework); err != nil {
		return nil, err
	}
	var firstPosition, secondPosition int
	for _, item := range []struct {
		id       domain.HomeworkTaskID
		position *int
	}{{first, &firstPosition}, {second, &secondPosition}} {
		if err := tx.QueryRowContext(ctx,
			`SELECT position FROM homework_tasks WHERE homework_id = ? AND id = ?`,
			homework, item.id,
		).Scan(item.position); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, domain.ErrHomeworkTaskNotFound
			}
			return nil, err
		}
	}
	if first == second {
		return []domain.HomeworkTaskPosition{{TaskID: first, Position: firstPosition}}, tx.Commit()
	}
	// A free nonnegative slot preserves unique positions throughout the swap.
	var temporaryPosition int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) + 1 FROM homework_tasks WHERE homework_id = ?`, homework,
	).Scan(&temporaryPosition); err != nil {
		return nil, err
	}
	for _, move := range []domain.HomeworkTaskPosition{
		{TaskID: first, Position: temporaryPosition},
		{TaskID: second, Position: firstPosition},
		{TaskID: first, Position: secondPosition},
	} {
		if _, err := tx.ExecContext(ctx,
			`UPDATE homework_tasks SET position = ?, updated_at = ? WHERE homework_id = ? AND id = ?`,
			move.Position, at.UnixMilli(), homework, move.TaskID,
		); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return []domain.HomeworkTaskPosition{
		{TaskID: first, Position: secondPosition},
		{TaskID: second, Position: firstPosition},
	}, nil
}
