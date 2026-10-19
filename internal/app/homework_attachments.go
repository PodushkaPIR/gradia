package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type HomeworkAttachmentFiles interface {
	Read(context.Context, string) (domain.AttachmentFile, error)
	Write(context.Context, domain.AttachmentFile) (string, error)
}

var ErrAttachmentFilesUnavailable = errors.New("работа с файлами не настроена")
var ErrAttachmentPathRequired = errors.New("укажите путь к файлу")
var ErrAttachmentRegularFileRequired = errors.New("выберите обычный файл изображения или PDF")

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
	pages, err := domain.NormalizeAttachmentPages(pages)
	if err != nil {
		return domain.HomeworkAttachment{}, err
	}
	if s.files == nil {
		return domain.HomeworkAttachment{}, ErrAttachmentFilesUnavailable
	}
	if strings.TrimSpace(path) == "" {
		return domain.HomeworkAttachment{}, ErrAttachmentPathRequired
	}
	// Reject foreign or missing tasks before reading a file from the caller's path.
	if _, err := s.Attachments(ctx, student, homework, task); err != nil {
		return domain.HomeworkAttachment{}, err
	}
	file, err := s.files.Read(ctx, path)
	if err != nil {
		return domain.HomeworkAttachment{}, err
	}
	mediaType := http.DetectContentType(file.Data)
	if len(file.Data) >= 4 && (bytes.Equal(file.Data[:4], []byte{'I', 'I', 42, 0}) || bytes.Equal(file.Data[:4], []byte{'M', 'M', 0, 42})) {
		mediaType = "image/tiff"
	}
	if mediaType != "application/pdf" && !strings.HasPrefix(mediaType, "image/") {
		return domain.HomeworkAttachment{}, domain.ErrAttachmentType
	}
	if pages != "" && mediaType != "application/pdf" {
		return domain.HomeworkAttachment{}, domain.ErrAttachmentPagesOnlyPDF
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
