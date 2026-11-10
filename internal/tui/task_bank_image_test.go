package tui

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/attachments"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

func TestTutorCreatesImageTaskAndRetriesWithoutLosingDraft(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "image.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	files := &attachmentTestFiles{LocalFiles: attachments.NewLocalFiles(dir), readErr: errors.New("read failed")}
	bank := app.NewTaskBankService(store).WithAttachments(files, nil)
	collection, err := bank.CreateCollection(ctx, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "условие.png")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, app.NewStudentService(store), nil, nil).WithTaskBank(bank)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 80, Height: 32})
	model, load := updateApp(model, keypress('b'))
	model, _ = updateApp(model, load())
	if !strings.Contains(model.taskBank.ShortHelp(), "N — из картинки") {
		t.Fatal("image entry is not discoverable")
	}
	model, _ = updateApp(model, keypress('N'))
	if !strings.Contains(model.View().Content, "Задача из картинки") || model.taskBank.taskForm.focus != 5 {
		t.Fatal("N did not focus image path")
	}
	model, save := updateApp(model, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model, _ = updateApp(model, save())
	if !strings.Contains(model.View().Content, "Введите короткое название") || files.reads != 0 {
		t.Fatal("empty image title did not produce a useful validation error")
	}
	model = typeHomeworkText(model, path)
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model = typeHomeworkText(model, "Уравнение 1")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model = typeHomeworkText(model, "Математика")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model = typeHomeworkText(model, "Уравнения")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model, _ = updateApp(model, keypress('2'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model, _ = updateApp(model, keypress(' '))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	if model.taskBank.taskForm.focus != 5 {
		t.Fatal("Tab did not cycle back to image path")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if model.taskBank.taskForm.focus != 4 {
		t.Fatal("Shift+Tab did not return to collections")
	}
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 80, Height: 24})
	if lines := strings.Count(model.View().Content, "\n") + 1; lines > 24 {
		t.Fatalf("image form overflows 80x24: %d lines\n%s", lines, model.View().Content)
	}
	saveKey := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	model, save = updateApp(model, saveKey)
	if save == nil || files.reads != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("file read must execute inside Cmd")
	}
	model, duplicate := updateApp(model, saveKey)
	if duplicate != nil {
		t.Fatal("duplicate save while busy")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.screenInputMode() != inputBusy {
		t.Fatal("escaped during save")
	}
	model, _ = updateApp(model, save())
	form := model.taskBank.taskForm
	if form == nil || form.saving || form.content.Value() != "Уравнение 1" || form.imagePath.Value() != path || !strings.Contains(form.err, "Не удалось сохранить") {
		t.Fatal("read failure lost draft or retry error")
	}
	page, err := bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks) != 0 {
		t.Fatalf("failed read created task: %+v %v", page, err)
	}
	files.readErr = nil
	model, save = updateApp(model, saveKey)
	model, load = updateApp(model, save())
	model, _ = updateApp(model, load())
	page, err = bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks) != 1 {
		t.Fatalf("retry failed: %+v %v", page, err)
	}
	task := page.Tasks[0]
	if task.Content != "Уравнение 1" || task.Subject != "Математика" || task.Difficulty != domain.DifficultyMedium || len(task.Topics) != 1 || task.Topics[0] != "Уравнения" || len(task.CollectionIDs) != 1 || task.CollectionIDs[0] != collection.ID {
		t.Fatalf("metadata lost: %+v", task)
	}
	items, err := bank.Attachments(ctx, task.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("image missing: %+v %v", items, err)
	}
	model, _ = updateApp(model, keypress('N'))
	model = typeHomeworkText(model, path)
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.taskBank.taskForm != nil || files.reads != 2 {
		t.Fatal("cancel read a file or left form open")
	}
}

func TestImageTaskCreationDisabledWithoutFileAdapter(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "disabled.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	model := NewApp(ctx, app.NewStudentService(store), nil, nil).WithTaskBank(app.NewTaskBankService(store))
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, keypress('b'))
	model, _ = updateApp(model, load())
	model, cmd := updateApp(model, keypress('N'))
	if cmd != nil || model.taskBank.taskForm != nil || strings.Contains(model.taskBank.ShortHelp(), "из картинки") {
		t.Fatal("image entry enabled without files")
	}
}
