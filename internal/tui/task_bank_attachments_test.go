package tui

import (
	"bytes"
	"context"
	"errors"
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

func TestTutorAttachesAndOpensBankFilesWithSharedAttachmentScreen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "bank-files.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	files := &attachmentTestFiles{LocalFiles: attachments.NewLocalFiles(filepath.Join(dir, "exports")), readErr: errors.New("read failed")}
	data := []byte("%PDF-1.4\nBANK_PDF_BYTES\n%%EOF\n")
	source := filepath.Join(dir, "bank #1.pdf")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	opens := 0
	var exported string
	bank := app.NewTaskBankService(store).WithAttachments(files, func(_ context.Context, path string) error {
		opens++
		exported = path
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, data) {
			t.Fatalf("viewer received changed bytes: %v", err)
		}
		return errors.New("viewer unavailable")
	})
	task, err := bank.Create(ctx, domain.TaskBankInput{Content: "Задача банка с файлом", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyMedium})
	if err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, app.NewStudentService(store), nil, nil).WithTaskBank(bank)
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, keypress('b'))
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, load = updateApp(model, keypress('a'))
	if load == nil {
		t.Fatal("a should open bank-task attachments from detail")
	}
	oldLoad := load
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, load = updateApp(model, keypress('a'))
	model, _ = updateApp(model, oldLoad())
	if !strings.Contains(model.View().Content, "Загрузка") {
		t.Fatal("old file-screen reply replaced reopened bank attachments")
	}
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('n'))
	model = typeHomeworkText(model, source)
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model = typeHomeworkText(model, "3-7")
	saveKey := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	model, save := updateApp(model, saveKey)
	if files.reads != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("bank file read occurred before Cmd")
	}
	model, duplicate := updateApp(model, saveKey)
	if duplicate != nil {
		t.Fatal("duplicate bank file import")
	}
	model, _ = updateApp(model, save())
	if model.screenInputMode() != inputText || !strings.Contains(model.View().Content, "Не удалось прикрепить файл") || !strings.Contains(model.View().Content, "3-7") {
		t.Fatal("failed bank import lost draft")
	}
	files.readErr = nil
	model, save = updateApp(model, saveKey)
	model, _ = updateApp(model, save())
	if !strings.Contains(model.View().Content, "bank #1.pdf") || !strings.Contains(model.View().Content, "стр. 3-7") {
		t.Fatal("bank attachment metadata missing")
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	model, prepare := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if files.writes != 0 || opens != 0 {
		t.Fatal("file preparation occurred before Cmd")
	}
	model, open := updateApp(model, prepare())
	if opens != 0 || model.screenInputMode() != inputNavigation || !strings.Contains(model.View().Content, "Файл сохранён:") {
		t.Fatal("viewer must run separately from file preparation")
	}
	model, _ = updateApp(model, open())
	if opens != 1 || !strings.Contains(model.View().Content, exported) || !strings.Contains(model.View().Content, "Не удалось открыть просмотрщик") {
		t.Fatal("viewer error lost bank file path")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(model.View().Content, task.Content) {
		t.Fatal("closing files should return to bank-task detail")
	}
	if items, err := bank.Attachments(ctx, task.ID); err != nil || len(items) != 1 {
		t.Fatalf("bank attachment not saved: %+v %v", items, err)
	}
}
