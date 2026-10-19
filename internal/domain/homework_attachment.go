package domain

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

type HomeworkAttachmentID int64

type HomeworkAttachment struct {
	ID                     HomeworkAttachmentID
	TaskID                 HomeworkTaskID
	Name, MediaType, Pages string
	Size                   int64
	CreatedAt              time.Time
}

// AttachmentFile carries the original, unmodified bytes across the local-file boundary.
type AttachmentFile struct {
	Name string
	Data []byte
}

var (
	ErrHomeworkAttachmentNotFound = errors.New("файл не найден у выбранной задачи")
	ErrAttachmentType             = errors.New("поддерживаются PDF и изображения PNG, JPEG, GIF, WebP, BMP, TIFF")
	ErrAttachmentPages            = errors.New("укажите страницу или диапазон: например, 2 или 2-5")
	ErrAttachmentPagesOnlyPDF     = errors.New("страницы можно указывать только для PDF")
)

func NormalizeAttachmentPages(pages string) (string, error) {
	pages = strings.TrimSpace(pages)
	if pages == "" {
		return "", nil
	}
	parts := strings.Split(strings.ReplaceAll(pages, "–", "-"), "-")
	if len(parts) > 2 {
		return "", ErrAttachmentPages
	}
	first, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || first <= 0 {
		return "", ErrAttachmentPages
	}
	if len(parts) == 1 {
		return strconv.Itoa(first), nil
	}
	last, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || last < first {
		return "", ErrAttachmentPages
	}
	if last == first {
		return strconv.Itoa(first), nil
	}
	return strconv.Itoa(first) + "-" + strconv.Itoa(last), nil
}
