package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

var ErrInvalidTaskPack = errors.New("некорректный или повреждённый Task Pack")
var ErrTaskPackPathRequired = errors.New("укажите путь к Task Pack")

type TaskPackData struct {
	Tasks  []TaskPackTask
	Assets map[string][]byte
}

type TaskPackImportResult struct{ Imported, Duplicates, Conflicts int }

type TaskPackReader interface {
	Read(context.Context, string) (TaskPackData, error)
}

func (s *TaskBankService) WithTaskPackReader(reader TaskPackReader) *TaskBankService {
	configured := *s
	configured.packReader = reader
	return &configured
}

func (s *TaskBankService) TaskPackImportEnabled() bool { return s.packReader != nil }

// ImportPack always keeps local tasks when their portable identities conflict.
func (s *TaskBankService) ImportPack(ctx context.Context, path string) (TaskPackImportResult, error) {
	if strings.TrimSpace(path) == "" {
		return TaskPackImportResult{}, ErrTaskPackPathRequired
	}
	if s.packReader == nil {
		return TaskPackImportResult{}, errors.New("импорт Task Pack не настроен")
	}
	pack, err := s.packReader.Read(ctx, path)
	if err != nil {
		return TaskPackImportResult{}, err
	}
	if err := validateTaskPack(ctx, pack); err != nil {
		return TaskPackImportResult{}, err
	}
	return s.repo.ImportTaskPack(ctx, pack, time.Now().UTC().Truncate(time.Millisecond))
}

func validateTaskPack(ctx context.Context, pack TaskPackData) error {
	if len(pack.Tasks) == 0 {
		return ErrInvalidTaskPack
	}
	seen := make(map[string]bool)
	for _, task := range pack.Tasks {
		if err := ctx.Err(); err != nil {
			return err
		}
		identity, err := hex.DecodeString(task.ID)
		if err != nil || len(identity) != 16 || strings.ToLower(task.ID) != task.ID || seen[task.ID] {
			return ErrInvalidTaskPack
		}
		seen[task.ID] = true
		input, err := domain.NormalizeTaskBankInput(domain.TaskBankInput{Content: task.Content, Subject: task.Subject, Topics: task.Topics, Difficulty: task.Difficulty})
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidTaskPack, err)
		}
		if input.Content != task.Content || input.Subject != task.Subject || !slices.Equal(input.Topics, task.Topics) {
			return ErrInvalidTaskPack
		}
		collections := make(map[string]bool)
		for _, name := range task.Collections {
			normalized, err := domain.NormalizeTaskCollectionName(name)
			if err != nil || normalized != name || collections[name] {
				return ErrInvalidTaskPack
			}
			collections[name] = true
		}
		for _, attachment := range task.Attachments {
			if err := ctx.Err(); err != nil {
				return err
			}
			if attachment.Name == "" || attachment.Name == "." || attachment.Name == ".." || strings.ContainsAny(attachment.Name, "/\x00") {
				return ErrInvalidTaskPack
			}
			pages, err := domain.NormalizeAttachmentPages(attachment.Pages)
			if err != nil || pages != attachment.Pages {
				return ErrInvalidTaskPack
			}
			data, ok := pack.Assets[attachment.Path]
			if !ok || !strings.HasPrefix(attachment.Path, "assets/") || int64(len(data)) != attachment.Size {
				return ErrInvalidTaskPack
			}
			mediaType, err := validateAttachmentData(data, pages)
			if err != nil || mediaType != attachment.MediaType {
				return ErrInvalidTaskPack
			}
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != attachment.SHA256 {
				return ErrInvalidTaskPack
			}
		}
		if TaskPackFingerprint(task) != task.SHA256 {
			return ErrInvalidTaskPack
		}
	}
	return ctx.Err()
}
