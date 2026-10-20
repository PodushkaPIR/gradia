package app

import (
	"context"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *TaskBankService) WithAttachments(files AttachmentFiles, open func(context.Context, string) error) *TaskBankService {
	configured := *s
	configured.files, configured.openFile = files, open
	return &configured
}

func (s *TaskBankService) AttachmentsEnabled() bool { return s.files != nil }

func (s *TaskBankService) Attachments(ctx context.Context, task domain.TaskBankTaskID) ([]domain.TaskBankAttachment, error) {
	if task <= 0 {
		return nil, domain.ErrTaskBankTaskNotFound
	}
	return s.repo.ListBankAttachments(ctx, task)
}

func (s *TaskBankService) AttachFile(ctx context.Context, task domain.TaskBankTaskID, path, pages string) (domain.TaskBankAttachment, error) {
	if task <= 0 {
		return domain.TaskBankAttachment{}, domain.ErrTaskBankTaskNotFound
	}
	pages, err := validateAttachmentRequest(s.files, path, pages)
	if err != nil {
		return domain.TaskBankAttachment{}, err
	}
	if _, err := s.Attachments(ctx, task); err != nil {
		return domain.TaskBankAttachment{}, err
	}
	file, mediaType, err := readAttachment(ctx, s.files, path, pages)
	if err != nil {
		return domain.TaskBankAttachment{}, err
	}
	metadata := domain.TaskBankAttachment{TaskID: task, Name: file.Name, MediaType: mediaType, Pages: pages, Size: int64(len(file.Data)), CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
	return s.repo.AddBankAttachment(ctx, metadata, file.Data)
}

// Attachment exposes original bytes for portable Task Pack export, without filesystem copies.
func (s *TaskBankService) Attachment(ctx context.Context, task domain.TaskBankTaskID, id domain.TaskBankAttachmentID) (domain.TaskBankAttachment, []byte, error) {
	if task <= 0 {
		return domain.TaskBankAttachment{}, nil, domain.ErrTaskBankTaskNotFound
	}
	if id <= 0 {
		return domain.TaskBankAttachment{}, nil, domain.ErrTaskBankAttachmentNotFound
	}
	return s.repo.GetBankAttachment(ctx, task, id)
}

func (s *TaskBankService) ExportAttachment(ctx context.Context, task domain.TaskBankTaskID, id domain.TaskBankAttachmentID) (string, error) {
	if s.files == nil {
		return "", ErrAttachmentFilesUnavailable
	}
	metadata, data, err := s.Attachment(ctx, task, id)
	if err != nil {
		return "", err
	}
	return s.files.Write(ctx, domain.AttachmentFile{Name: metadata.Name, Data: data})
}

func (s *TaskBankService) OpenAttachment(ctx context.Context, path string) error {
	if s.openFile == nil {
		return errors.New("не настроен локальный просмотрщик; откройте файл вручную")
	}
	return s.openFile(ctx, path)
}
