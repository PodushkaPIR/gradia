package app

import (
	"context"
	"errors"
	"slices"

	"github.com/shesh/gradia/internal/domain"
)

var ErrTaskPackSelectionRequired = errors.New("отметьте хотя бы одну задачу для Task Pack")

type TaskPackSource struct{ Tasks []TaskPackSourceTask }

type TaskPackSourceTask struct {
	Key         string
	Task        domain.TaskBankTask
	Collections []string
	Attachments []domain.TaskBankAttachment
}

type TaskPackWriter interface {
	Write(context.Context, TaskPackSource, func(context.Context, domain.TaskBankAttachment) ([]byte, error)) (string, error)
}

func (s *TaskBankService) WithTaskPackWriter(writer TaskPackWriter) *TaskBankService {
	configured := *s
	configured.packWriter = writer
	return &configured
}

func (s *TaskBankService) TaskPackExportEnabled() bool { return s.packWriter != nil }

func (s *TaskBankService) ExportPack(ctx context.Context, selection []domain.TaskBankTaskID) (string, error) {
	if len(selection) == 0 {
		return "", ErrTaskPackSelectionRequired
	}
	ids := slices.Clone(selection)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if ids[0] <= 0 {
		return "", domain.ErrTaskBankTaskNotFound
	}
	if s.packWriter == nil {
		return "", errors.New("экспорт Task Pack не настроен")
	}
	source, err := s.repo.TaskPackSnapshot(ctx, ids)
	if err != nil {
		return "", err
	}
	load := func(ctx context.Context, expected domain.TaskBankAttachment) ([]byte, error) {
		actual, data, err := s.Attachment(ctx, expected.TaskID, expected.ID)
		if err != nil {
			return nil, err
		}
		if actual != expected {
			return nil, errors.New("вложение изменилось во время экспорта; повторите экспорт")
		}
		return data, nil
	}
	return s.packWriter.Write(ctx, source, load)
}
