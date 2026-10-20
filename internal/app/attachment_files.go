package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/shesh/gradia/internal/domain"
)

type AttachmentFiles interface {
	Read(context.Context, string) (domain.AttachmentFile, error)
	Write(context.Context, domain.AttachmentFile) (string, error)
}

type HomeworkAttachmentFiles = AttachmentFiles

var ErrAttachmentFilesUnavailable = errors.New("работа с файлами не настроена")
var ErrAttachmentPathRequired = errors.New("укажите путь к файлу")
var ErrAttachmentRegularFileRequired = errors.New("выберите обычный файл изображения или PDF")

func validateAttachmentRequest(files AttachmentFiles, path, pages string) (string, error) {
	pages, err := domain.NormalizeAttachmentPages(pages)
	if err != nil {
		return "", err
	}
	if files == nil {
		return "", ErrAttachmentFilesUnavailable
	}
	if strings.TrimSpace(path) == "" {
		return "", ErrAttachmentPathRequired
	}
	return pages, nil
}

func readAttachment(ctx context.Context, files AttachmentFiles, path, pages string) (domain.AttachmentFile, string, error) {
	file, err := files.Read(ctx, path)
	if err != nil {
		return domain.AttachmentFile{}, "", err
	}
	mediaType := http.DetectContentType(file.Data)
	if len(file.Data) >= 4 && (bytes.Equal(file.Data[:4], []byte{'I', 'I', 42, 0}) || bytes.Equal(file.Data[:4], []byte{'M', 'M', 0, 42})) {
		mediaType = "image/tiff"
	}
	if mediaType != "application/pdf" && !strings.HasPrefix(mediaType, "image/") {
		return domain.AttachmentFile{}, "", domain.ErrAttachmentType
	}
	if pages != "" && mediaType != "application/pdf" {
		return domain.AttachmentFile{}, "", domain.ErrAttachmentPagesOnlyPDF
	}
	return file, mediaType, nil
}
