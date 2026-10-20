package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

type packWriterStub struct {
	writes int
	err    error
	source app.TaskPackSource
}

func (w *packWriterStub) Write(_ context.Context, source app.TaskPackSource, _ func(context.Context, domain.TaskBankAttachment) ([]byte, error)) (string, error) {
	w.writes++
	w.source = source
	if w.err != nil {
		return "", w.err
	}
	return "/tmp/demo-task-pack.zip", nil
}

func TestTutorExportsMarkedTasksAcrossFiltersAndRetriesWithoutLosingSelection(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "packs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	writer := &packWriterStub{err: errors.New("disk unavailable")}
	bank := app.NewTaskBankService(store).WithTaskPackWriter(writer)
	for _, item := range []struct{ content, subject string }{{"Задача А", "Информатика"}, {"Задача Б", "Математика"}} {
		if _, err := bank.Create(ctx, domain.TaskBankInput{Content: item.content, Subject: item.subject, Topics: []string{"Тема"}, Difficulty: domain.DifficultyEasy}); err != nil {
			t.Fatal(err)
		}
	}
	model := NewApp(ctx, app.NewStudentService(store), nil, nil).WithTaskBank(bank)
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, keypress('b'))
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress(' '))
	model, _ = updateApp(model, keypress('s'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyDown})
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress(' '))
	if !strings.Contains(model.View().Content, "Отмечено: 2") {
		t.Fatal("selection was lost when applying a filter")
	}
	model, export := updateApp(model, keypress('E'))
	if export == nil || writer.writes != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("pack export must run through Cmd")
	}
	model, duplicate := updateApp(model, keypress('E'))
	if duplicate != nil {
		t.Fatal("duplicate pack export")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.screenInputMode() != inputBusy {
		t.Fatal("left bank while exporting")
	}
	model, _ = updateApp(model, export())
	if model.screenInputMode() != inputNavigation || !strings.Contains(model.View().Content, "Не удалось экспортировать Task Pack") || !strings.Contains(model.View().Content, "Отмечено: 2") {
		t.Fatal("failed export lost selection or cannot be retried")
	}
	writer.err = nil
	model, export = updateApp(model, keypress('E'))
	model, _ = updateApp(model, export())
	if !strings.Contains(model.View().Content, "/tmp/demo-task-pack.zip") || len(writer.source.Tasks) != 2 || writer.source.Tasks[0].Task.Content != "Задача А" || writer.source.Tasks[1].Task.Content != "Задача Б" {
		t.Fatal("pack did not include the marked hidden task")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, load = updateApp(model, keypress('b'))
	model, _ = updateApp(model, load())
	model, export = updateApp(model, keypress('E'))
	if export != nil || !strings.Contains(model.View().Content, "Отметьте задачи") {
		t.Fatal("export should require explicit marks in the current bank session")
	}
}
