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

type attachmentTestFiles struct {
	*attachments.LocalFiles
	readErr, writeErr error
	reads, writes     int
}

func (f *attachmentTestFiles) Read(ctx context.Context, path string) (domain.AttachmentFile, error) {
	f.reads++
	if f.readErr != nil {
		return domain.AttachmentFile{}, f.readErr
	}
	return f.LocalFiles.Read(ctx, path)
}

func (f *attachmentTestFiles) Write(ctx context.Context, file domain.AttachmentFile) (string, error) {
	f.writes++
	if f.writeErr != nil {
		return "", f.writeErr
	}
	return f.LocalFiles.Write(ctx, file)
}

func TestTutorAttachesPDFAndKeepsFileAvailableWhenViewerFails(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	files := &attachmentTestFiles{LocalFiles: attachments.NewLocalFiles(filepath.Join(dir, "exports")), readErr: errors.New("read failed")}
	data := []byte("%PDF-1.4\nORIGINAL_PDF_BYTES\n%%EOF\n")
	source := filepath.Join(dir, "пример #1.pdf")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	opens := 0
	var exportedPath string
	service := app.NewHomeworkService(store).WithAttachments(files, func(_ context.Context, path string) error {
		opens++
		exportedPath = path
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, data) {
			t.Fatalf("viewer received altered content: %v", err)
		}
		return errors.New("viewer unavailable")
	})
	homework, err := service.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.AddTask(ctx, student.ID, homework.ID, domain.HomeworkTaskInput{Content: "Задача с PDF", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetCompleted(ctx, student.ID, homework.ID, task.ID, true); err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, students, nil, app.NewRouteService(store)).WithHomework(service)
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, load = updateApp(model, keypress('w'))
	model, _ = updateApp(model, load())
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, load = updateApp(model, keypress('a'))
	if load == nil {
		t.Fatal("a should open attachments for selected task")
	}
	oldLoad := load
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, load = updateApp(model, keypress('a'))
	model, _ = updateApp(model, oldLoad())
	if !strings.Contains(model.View().Content, "Загрузка") {
		t.Fatal("old attachment view response replaced the reopened view")
	}
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('n'))
	model = typeHomeworkText(model, "unsaved.pdf")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(model.View().Content, "Файлов пока нет") || files.reads != 0 {
		t.Fatal("cancel closed attachment list or imported file")
	}
	model, _ = updateApp(model, keypress('n'))
	model = typeHomeworkText(model, source)
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model = typeHomeworkText(model, "2-4")
	saveKey := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	model, save := updateApp(model, saveKey)
	if save == nil || files.reads != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("file import must wait for Cmd")
	}
	model, duplicate := updateApp(model, saveKey)
	if duplicate != nil {
		t.Fatal("duplicate attachment import")
	}
	model, _ = updateApp(model, save())
	if model.screenInputMode() != inputText || !strings.Contains(model.View().Content, "Не удалось прикрепить файл") || !strings.Contains(model.View().Content, "2-4") {
		t.Fatal("failed import lost file form")
	}
	files.readErr = nil
	model, save = updateApp(model, saveKey)
	model, _ = updateApp(model, save())
	if !strings.Contains(model.View().Content, "пример #1.pdf") || !strings.Contains(model.View().Content, "стр. 2-4") {
		t.Fatalf("attachment metadata missing:\n%s", model.View().Content)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	files.writeErr = errors.New("export directory unavailable")
	model, export := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if export == nil || files.writes != 0 || opens != 0 {
		t.Fatal("file preparation should run through Cmd")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.screenInputMode() != inputBusy {
		t.Fatal("left attachment screen while preparing file")
	}
	model, _ = updateApp(model, export())
	if model.screenInputMode() != inputNavigation || !strings.Contains(model.View().Content, "Не удалось подготовить файл") {
		t.Fatal("export failure not recoverable")
	}
	files.writeErr = nil
	model, export = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, open := updateApp(model, export())
	if open == nil || opens != 0 || model.screenInputMode() != inputNavigation || !strings.Contains(model.View().Content, "Файл сохранён:") {
		t.Fatal("viewer must open separately after path is available")
	}
	model, _ = updateApp(model, open())
	if opens != 1 || !strings.Contains(model.View().Content, exportedPath) || !strings.Contains(model.View().Content, "Не удалось открыть просмотрщик") {
		t.Fatal("viewer failure lost saved file path")
	}
	if files.reads != 2 {
		t.Fatal("opening attachment reread the removed source file")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(model.View().Content, "1/1 выполнено") {
		t.Fatal("attachments changed completion or Esc left task view")
	}
}
