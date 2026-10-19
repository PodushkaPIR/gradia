package app

import (
	"context"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type HomeworkRepository interface {
	ListHomework(context.Context, domain.StudentID) ([]domain.Homework, error)
	CreateHomework(context.Context, domain.StudentID, string, time.Time) (domain.Homework, error)
	RenameHomework(context.Context, domain.StudentID, domain.HomeworkID, string, time.Time) (domain.Homework, error)
	ListHomeworkTasks(context.Context, domain.StudentID, domain.HomeworkID) ([]domain.HomeworkTask, error)
	SaveHomeworkTask(context.Context, domain.StudentID, domain.HomeworkID, domain.HomeworkTaskID, domain.HomeworkTaskInput, time.Time) (domain.HomeworkTask, error)
	SetHomeworkTaskCompleted(context.Context, domain.StudentID, domain.HomeworkID, domain.HomeworkTaskID, bool, time.Time) (domain.HomeworkTask, error)
	ListHomeworkAttachments(context.Context, domain.StudentID, domain.HomeworkID, domain.HomeworkTaskID) ([]domain.HomeworkAttachment, error)
	AddHomeworkAttachment(context.Context, domain.StudentID, domain.HomeworkID, domain.HomeworkAttachment, []byte) (domain.HomeworkAttachment, error)
	GetHomeworkAttachment(context.Context, domain.StudentID, domain.HomeworkID, domain.HomeworkTaskID, domain.HomeworkAttachmentID) (domain.HomeworkAttachment, []byte, error)
}

type HomeworkService struct {
	repo     HomeworkRepository
	files    HomeworkAttachmentFiles
	openFile func(context.Context, string) error
}

func NewHomeworkService(repo HomeworkRepository) *HomeworkService {
	return &HomeworkService{repo: repo}
}

func (s *HomeworkService) List(ctx context.Context, student domain.StudentID) ([]domain.Homework, error) {
	if student <= 0 {
		return nil, domain.ErrStudentNotFound
	}
	return s.repo.ListHomework(ctx, student)
}

func (s *HomeworkService) Create(ctx context.Context, student domain.StudentID, name string) (domain.Homework, error) {
	if student <= 0 {
		return domain.Homework{}, domain.ErrStudentNotFound
	}
	name, err := domain.NormalizeHomeworkName(name)
	if err != nil {
		return domain.Homework{}, err
	}
	return s.repo.CreateHomework(ctx, student, name, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *HomeworkService) Rename(ctx context.Context, student domain.StudentID, id domain.HomeworkID, name string) (domain.Homework, error) {
	if err := validateHomeworkIDs(student, id); err != nil {
		return domain.Homework{}, err
	}
	name, err := domain.NormalizeHomeworkName(name)
	if err != nil {
		return domain.Homework{}, err
	}
	return s.repo.RenameHomework(ctx, student, id, name, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *HomeworkService) Tasks(ctx context.Context, student domain.StudentID, id domain.HomeworkID) ([]domain.HomeworkTask, error) {
	if err := validateHomeworkIDs(student, id); err != nil {
		return nil, err
	}
	return s.repo.ListHomeworkTasks(ctx, student, id)
}

func (s *HomeworkService) AddTask(ctx context.Context, student domain.StudentID, id domain.HomeworkID, input domain.HomeworkTaskInput) (domain.HomeworkTask, error) {
	return s.saveTask(ctx, student, id, 0, input)
}

func (s *HomeworkService) EditTask(ctx context.Context, student domain.StudentID, id domain.HomeworkID, task domain.HomeworkTaskID, input domain.HomeworkTaskInput) (domain.HomeworkTask, error) {
	if task <= 0 {
		return domain.HomeworkTask{}, domain.ErrHomeworkTaskNotFound
	}
	return s.saveTask(ctx, student, id, task, input)
}

func (s *HomeworkService) saveTask(ctx context.Context, student domain.StudentID, id domain.HomeworkID, task domain.HomeworkTaskID, input domain.HomeworkTaskInput) (domain.HomeworkTask, error) {
	if err := validateHomeworkIDs(student, id); err != nil {
		return domain.HomeworkTask{}, err
	}
	input, err := domain.NormalizeHomeworkTask(input)
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	return s.repo.SaveHomeworkTask(ctx, student, id, task, input, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *HomeworkService) SetCompleted(ctx context.Context, student domain.StudentID, id domain.HomeworkID, task domain.HomeworkTaskID, completed bool) (domain.HomeworkTask, error) {
	if err := validateHomeworkIDs(student, id); err != nil {
		return domain.HomeworkTask{}, err
	}
	if task <= 0 {
		return domain.HomeworkTask{}, domain.ErrHomeworkTaskNotFound
	}
	return s.repo.SetHomeworkTaskCompleted(ctx, student, id, task, completed, time.Now().UTC().Truncate(time.Millisecond))
}

func validateHomeworkIDs(student domain.StudentID, id domain.HomeworkID) error {
	if student <= 0 {
		return domain.ErrStudentNotFound
	}
	if id <= 0 {
		return domain.ErrHomeworkNotFound
	}
	return nil
}
