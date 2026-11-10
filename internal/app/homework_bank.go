package app

import (
	"context"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

var ErrBankSelectionRequired = errors.New("отметьте задачи или выберите текущую задачу банка")

func (s *HomeworkService) AddFromBank(
	ctx context.Context,
	student domain.StudentID,
	id domain.HomeworkID,
	source domain.TaskBankTaskID,
) (domain.HomeworkTask, error) {
	tasks, err := s.AddManyFromBank(ctx, student, id, []domain.TaskBankTaskID{source})
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	return tasks[0], nil
}

// AddManyFromBank copies independent snapshots atomically, preserving selection order.
func (s *HomeworkService) AddManyFromBank(
	ctx context.Context,
	student domain.StudentID,
	id domain.HomeworkID,
	sources []domain.TaskBankTaskID,
) ([]domain.HomeworkTask, error) {
	if err := validateHomeworkIDs(student, id); err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, ErrBankSelectionRequired
	}
	ids := make([]domain.TaskBankTaskID, 0, len(sources))
	seen := make(map[domain.TaskBankTaskID]bool)
	for _, source := range sources {
		if source <= 0 {
			return nil, domain.ErrTaskBankTaskNotFound
		}
		if !seen[source] {
			ids = append(ids, source)
			seen[source] = true
		}
	}
	return s.repo.SnapshotBankTasksToHomework(ctx, student, id, ids, time.Now().UTC().Truncate(time.Millisecond))
}
