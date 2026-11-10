package app

import (
	"context"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *HomeworkService) SwapTasks(
	ctx context.Context,
	student domain.StudentID,
	homework domain.HomeworkID,
	first, second domain.HomeworkTaskID,
) ([]domain.HomeworkTaskPosition, error) {
	if err := validateHomeworkTaskIDs(student, homework, first); err != nil {
		return nil, err
	}
	if second <= 0 {
		return nil, domain.ErrHomeworkTaskNotFound
	}
	return s.repo.SwapHomeworkTasks(ctx, student, homework, first, second, time.Now().UTC().Truncate(time.Millisecond))
}
