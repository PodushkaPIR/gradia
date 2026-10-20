package app

import (
	"context"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *HomeworkService) WithAttachments(files HomeworkAttachmentFiles, open func(context.Context, string) error) *HomeworkService {
	configured := *s
	configured.files, configured.openFile = files, open
	return &configured
}

func (s *HomeworkService) AttachmentsEnabled() bool { return s.files != nil }

func (s *HomeworkService) Attachments(ctx context.Context, student domain.StudentID, homework domain.HomeworkID, task domain.HomeworkTaskID) ([]domain.HomeworkAttachment, error) {
	if err := validateHomeworkTaskIDs(student, homework, task); err != nil {
		return nil, err
	}
	return s.repo.ListHomeworkAttachments(ctx, student, homework, task)
}

func (s *HomeworkService) AttachFile(ctx context.Context, student domain.StudentID, homework domain.HomeworkID, task domain.HomeworkTaskID, path, pages string) (domain.HomeworkAttachment, error) {
	if err := validateHomeworkTaskIDs(student, homework, task); err != nil {
		return domain.HomeworkAttachment{}, err
	}
	pages, err := validateAttachmentRequest(s.files, path, pages)
	if err != nil {
		return domain.HomeworkAttachment{}, err
	}
	// Reject foreign or missing tasks before reading a file from the caller's path.
	if _, err := s.Attachments(ctx, student, homework, task); err != nil {
		return domain.HomeworkAttachment{}, err
	}
	file, mediaType, err := readAttachment(ctx, s.files, path, pages)
	if err != nil {
		return domain.HomeworkAttachment{}, err
	}
	metadata := domain.HomeworkAttachment{TaskID: task, Name: file.Name, MediaType: mediaType, Pages: pages, Size: int64(len(file.Data)), CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
	return s.repo.AddHomeworkAttachment(ctx, student, homework, metadata, file.Data)
}

func (s *HomeworkService) ExportAttachment(ctx context.Context, student domain.StudentID, homework domain.HomeworkID, task domain.HomeworkTaskID, id domain.HomeworkAttachmentID) (string, error) {
	if err := validateHomeworkTaskIDs(student, homework, task); err != nil {
		return "", err
	}
	if id <= 0 {
		return "", domain.ErrHomeworkAttachmentNotFound
	}
	if s.files == nil {
		return "", ErrAttachmentFilesUnavailable
	}
	metadata, data, err := s.repo.GetHomeworkAttachment(ctx, student, homework, task, id)
	if err != nil {
		return "", err
	}
	return s.files.Write(ctx, domain.AttachmentFile{Name: metadata.Name, Data: data})
}

func (s *HomeworkService) OpenAttachment(ctx context.Context, path string) error {
	if s.openFile == nil {
		return errors.New("не настроен локальный просмотрщик; откройте файл вручную")
	}
	return s.openFile(ctx, path)
}

func validateHomeworkTaskIDs(student domain.StudentID, homework domain.HomeworkID, task domain.HomeworkTaskID) error {
	if err := validateHomeworkIDs(student, homework); err != nil {
		return err
	}
	if task <= 0 {
		return domain.ErrHomeworkTaskNotFound
	}
	return nil
}
