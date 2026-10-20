package app

import (
	"context"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type TaskBankCatalog struct {
	Subjects, Topics []string
	Collections      []domain.TaskCollection
}

type TaskBankPage struct {
	Tasks []domain.TaskBankTask
	TaskBankCatalog
}

type TaskBankRepository interface {
	ListBankTasks(context.Context, domain.TaskBankFilter) ([]domain.TaskBankTask, error)
	BankTaskCatalog(context.Context, string) (TaskBankCatalog, error)
	SaveBankTask(context.Context, domain.TaskBankTaskID, domain.TaskBankInput, time.Time) (domain.TaskBankTask, error)
	CreateTaskCollection(context.Context, string, time.Time) (domain.TaskCollection, error)
	RenameTaskCollection(context.Context, domain.TaskCollectionID, string, time.Time) (domain.TaskCollection, error)
	ListBankAttachments(context.Context, domain.TaskBankTaskID) ([]domain.TaskBankAttachment, error)
	AddBankAttachment(context.Context, domain.TaskBankAttachment, []byte) (domain.TaskBankAttachment, error)
	GetBankAttachment(context.Context, domain.TaskBankTaskID, domain.TaskBankAttachmentID) (domain.TaskBankAttachment, []byte, error)
	TaskPackSnapshot(context.Context, []domain.TaskBankTaskID) (TaskPackSource, error)
	ImportTaskPack(context.Context, TaskPackData, time.Time) (TaskPackImportResult, error)
}

type TaskBankService struct {
	repo       TaskBankRepository
	files      AttachmentFiles
	openFile   func(context.Context, string) error
	packWriter TaskPackWriter
	packReader TaskPackReader
}

func NewTaskBankService(repo TaskBankRepository) *TaskBankService {
	return &TaskBankService{repo: repo}
}

func (s *TaskBankService) Browse(ctx context.Context, filter domain.TaskBankFilter) (TaskBankPage, error) {
	filter, err := domain.NormalizeTaskBankFilter(filter)
	if err != nil {
		return TaskBankPage{}, err
	}
	tasks, err := s.repo.ListBankTasks(ctx, filter)
	if err != nil {
		return TaskBankPage{}, err
	}
	catalog, err := s.repo.BankTaskCatalog(ctx, filter.Subject)
	if err != nil {
		return TaskBankPage{}, err
	}
	return TaskBankPage{Tasks: tasks, TaskBankCatalog: catalog}, nil
}

func (s *TaskBankService) Create(ctx context.Context, input domain.TaskBankInput) (domain.TaskBankTask, error) {
	return s.save(ctx, 0, input)
}

func (s *TaskBankService) Edit(ctx context.Context, id domain.TaskBankTaskID, input domain.TaskBankInput) (domain.TaskBankTask, error) {
	if id <= 0 {
		return domain.TaskBankTask{}, domain.ErrTaskBankTaskNotFound
	}
	return s.save(ctx, id, input)
}

func (s *TaskBankService) save(ctx context.Context, id domain.TaskBankTaskID, input domain.TaskBankInput) (domain.TaskBankTask, error) {
	input, err := domain.NormalizeTaskBankInput(input)
	if err != nil {
		return domain.TaskBankTask{}, err
	}
	return s.repo.SaveBankTask(ctx, id, input, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *TaskBankService) CreateCollection(ctx context.Context, name string) (domain.TaskCollection, error) {
	name, err := domain.NormalizeTaskCollectionName(name)
	if err != nil {
		return domain.TaskCollection{}, err
	}
	return s.repo.CreateTaskCollection(ctx, name, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *TaskBankService) RenameCollection(ctx context.Context, id domain.TaskCollectionID, name string) (domain.TaskCollection, error) {
	if id <= 0 {
		return domain.TaskCollection{}, domain.ErrTaskCollectionNotFound
	}
	name, err := domain.NormalizeTaskCollectionName(name)
	if err != nil {
		return domain.TaskCollection{}, err
	}
	return s.repo.RenameTaskCollection(ctx, id, name, time.Now().UTC().Truncate(time.Millisecond))
}
