package tui

import (
	"context"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

// The UI item ID is opaque within one already-scoped source; domain IDs stay typed in adapters.
type taskAttachmentItem struct {
	ID                     int64
	Name, MediaType, Pages string
	Size                   int64
}

type taskAttachmentSource interface {
	List(context.Context) ([]taskAttachmentItem, error)
	Add(context.Context, string, string) (taskAttachmentItem, error)
	Export(context.Context, int64) (string, error)
	Open(context.Context, string) error
}

type homeworkAttachmentSource struct {
	service  *app.HomeworkService
	student  domain.StudentID
	homework domain.HomeworkID
	task     domain.HomeworkTaskID
}

func (s homeworkAttachmentSource) List(ctx context.Context) ([]taskAttachmentItem, error) {
	items, err := s.service.Attachments(ctx, s.student, s.homework, s.task)
	if err != nil {
		return nil, err
	}
	result := make([]taskAttachmentItem, 0, len(items))
	for _, item := range items {
		result = append(result, homeworkAttachmentItem(item))
	}
	return result, nil
}

func homeworkAttachmentItem(item domain.HomeworkAttachment) taskAttachmentItem {
	return taskAttachmentItem{ID: int64(item.ID), Name: item.Name, MediaType: item.MediaType, Pages: item.Pages, Size: item.Size}
}

func (s homeworkAttachmentSource) Add(ctx context.Context, path, pages string) (taskAttachmentItem, error) {
	item, err := s.service.AttachFile(ctx, s.student, s.homework, s.task, path, pages)
	if err != nil {
		return taskAttachmentItem{}, err
	}
	if item.TaskID != s.task {
		return taskAttachmentItem{}, domain.ErrHomeworkAttachmentNotFound
	}
	return homeworkAttachmentItem(item), nil
}

func (s homeworkAttachmentSource) Export(ctx context.Context, id int64) (string, error) {
	return s.service.ExportAttachment(ctx, s.student, s.homework, s.task, domain.HomeworkAttachmentID(id))
}
func (s homeworkAttachmentSource) Open(ctx context.Context, path string) error {
	return s.service.OpenAttachment(ctx, path)
}

type bankAttachmentSource struct {
	service *app.TaskBankService
	task    domain.TaskBankTaskID
}

func bankAttachmentItem(item domain.TaskBankAttachment) taskAttachmentItem {
	return taskAttachmentItem{ID: int64(item.ID), Name: item.Name, MediaType: item.MediaType, Pages: item.Pages, Size: item.Size}
}

func (s bankAttachmentSource) List(ctx context.Context) ([]taskAttachmentItem, error) {
	items, err := s.service.Attachments(ctx, s.task)
	if err != nil {
		return nil, err
	}
	result := make([]taskAttachmentItem, 0, len(items))
	for _, item := range items {
		result = append(result, bankAttachmentItem(item))
	}
	return result, nil
}

func (s bankAttachmentSource) Add(ctx context.Context, path, pages string) (taskAttachmentItem, error) {
	item, err := s.service.AttachFile(ctx, s.task, path, pages)
	if err != nil {
		return taskAttachmentItem{}, err
	}
	if item.TaskID != s.task {
		return taskAttachmentItem{}, domain.ErrTaskBankAttachmentNotFound
	}
	return bankAttachmentItem(item), nil
}

func (s bankAttachmentSource) Export(ctx context.Context, id int64) (string, error) {
	return s.service.ExportAttachment(ctx, s.task, domain.TaskBankAttachmentID(id))
}
func (s bankAttachmentSource) Open(ctx context.Context, path string) error {
	return s.service.OpenAttachment(ctx, path)
}
