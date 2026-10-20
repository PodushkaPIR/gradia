package tui

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

type bankTestStore struct {
	*sqlite.Store
	saveErr error
}

func TestTaskBankDetailWrapsAndScrollsLongContentAfterResize(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "wrap.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bank := app.NewTaskBankService(store)
	task, err := bank.Create(ctx, domain.TaskBankInput{Content: "宽字符 https://example.org/" + strings.Repeat("long", 60) + "ENDMARK", Subject: "Информатика", Topics: []string{"Тема"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, app.NewStudentService(store), nil, nil).WithTaskBank(bank)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 28, Height: 16})
	model, load := updateApp(model, keypress('b'))
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	checkWidth := func(width int) {
		t.Helper()
		for _, line := range strings.Split(model.taskBank.View(), "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("detail line would be truncated at %d cells: %q", width, line)
			}
		}
	}
	checkWidth(28)
	for i := 0; i < 100 && !strings.Contains(strings.ReplaceAll(model.taskBank.View(), "\n", ""), "ENDMARK"); i++ {
		model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if !strings.Contains(strings.ReplaceAll(model.taskBank.View(), "\n", ""), "ENDMARK") {
		t.Fatal("long URL suffix cannot be reached by scrolling")
	}
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 80, Height: 24})
	checkWidth(80)
	if !strings.Contains(strings.ReplaceAll(model.taskBank.View(), "\n", ""), "ENDMARK") {
		t.Fatal("resize left detail scrolled beyond its content")
	}
	if page, err := bank.Browse(ctx, domain.TaskBankFilter{}); err != nil || !reflect.DeepEqual(page.Tasks[0], task) {
		t.Fatalf("wrapping changed stored content: %+v %v", page, err)
	}
}

func TestTaskBankRejectsStaleScreenRepliesAndPreservesEditorMetadata(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bank-routing.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bank := app.NewTaskBankService(store)
	collection, err := bank.CreateCollection(ctx, "Коллекция; \"пример\"")
	if err != nil {
		t.Fatal(err)
	}
	task, err := bank.Create(ctx, domain.TaskBankInput{Content: "Исходный текст\r\nПрактика", Subject: "Информатика\nосновы", Topics: []string{"Циклы; условия", "Тема \"цитата\"", "Класс\r\nи функции"}, Difficulty: domain.DifficultyEasy, CollectionIDs: []domain.TaskCollectionID{collection.ID}})
	if err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, app.NewStudentService(store), nil, nil).WithTaskBank(bank)
	model, _ = updateApp(model, model.Init()())
	model, oldLoad := updateApp(model, keypress('b'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, load := updateApp(model, keypress('b'))
	model, _ = updateApp(model, oldLoad())
	if !strings.Contains(model.View().Content, "Загрузка") {
		t.Fatal("old bank instance reply replaced the reopened screen")
	}
	model, _ = updateApp(model, keypress('?'))
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Выбрать предмет") {
		t.Fatal("help lost bank context while loading completed")
	}
	model, _ = updateApp(model, keypress('?'))
	model, _ = updateApp(model, keypress('e'))
	for _, r := range "qpb?" {
		model, _ = updateApp(model, keypress(r))
	}
	if model.screenInputMode() != inputText || model.help != nil {
		t.Fatal("global keys interrupted the task draft")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if page, err := bank.Browse(ctx, domain.TaskBankFilter{}); err != nil || !reflect.DeepEqual(page.Tasks[0], task) {
		t.Fatalf("canceled draft changed task: %+v %v", page, err)
	}
	model, _ = updateApp(model, keypress('e'))
	for i := 0; i < 3; i++ {
		model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	model, _ = updateApp(model, keypress('3'))
	model, save := updateApp(model, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model, load = updateApp(model, save())
	model, _ = updateApp(model, load())
	page, err := bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil {
		t.Fatal(err)
	}
	after := page.Tasks[0]
	if after.Difficulty != domain.DifficultyHard || after.Content != task.Content || after.Subject != task.Subject || !reflect.DeepEqual(after.Topics, task.Topics) || !reflect.DeepEqual(after.CollectionIDs, task.CollectionIDs) {
		t.Fatalf("editing difficulty changed other data: %+v", after)
	}
}

func (s *bankTestStore) SaveBankTask(ctx context.Context, id domain.TaskBankTaskID, input domain.TaskBankInput, at time.Time) (domain.TaskBankTask, error) {
	if s.saveErr != nil {
		return domain.TaskBankTask{}, s.saveErr
	}
	return s.Store.SaveBankTask(ctx, id, input, at)
}

func TestTutorCreatesAndFiltersTaskBankWithOverlappingCollections(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "bank.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	repo := &bankTestStore{Store: store}
	bank := app.NewTaskBankService(repo)
	students := app.NewStudentService(store)
	model := NewApp(ctx, students, nil, nil).WithTaskBank(bank)
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, keypress('b'))
	if load == nil {
		t.Fatal("b should open Task Bank without creating a student")
	}
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('c'))
	for _, name := range []string{"Экзамен", "Практика"} {
		var save, reload tea.Cmd
		model, _ = updateApp(model, keypress('n'))
		model = typeHomeworkText(model, name)
		model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
		model, reload = updateApp(model, save())
		model, _ = updateApp(model, reload())
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = updateApp(model, keypress('n'))
	model = typeHomeworkText(model, "Задача q/p\nНапишите цикл")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model = typeHomeworkText(model, "Информатика")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model = typeHomeworkText(model, "Циклы; Функции")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model, _ = updateApp(model, keypress('2'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model, _ = updateApp(model, keypress(' '))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyDown})
	model, _ = updateApp(model, keypress(' '))
	repo.saveErr = errors.New("disk unavailable")
	saveKey := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	model, save := updateApp(model, saveKey)
	if save == nil || model.screenInputMode() != inputBusy {
		t.Fatal("bank save must use Cmd")
	}
	if page, err := bank.Browse(ctx, domain.TaskBankFilter{}); err != nil || len(page.Tasks) != 0 {
		t.Fatal("task saved before Cmd")
	}
	model, duplicate := updateApp(model, saveKey)
	if duplicate != nil {
		t.Fatal("duplicate bank save")
	}
	model, _ = updateApp(model, save())
	if model.screenInputMode() != inputText || !strings.Contains(model.View().Content, "Не удалось сохранить задачу банка") || !strings.Contains(model.View().Content, "Информатика") {
		t.Fatal("failed save lost bank editor")
	}
	repo.saveErr = nil
	model, save = updateApp(model, saveKey)
	model, load = updateApp(model, save())
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Задача q/p") || !strings.Contains(model.View().Content, "Информатика") {
		t.Fatal("new bank task missing")
	}
	page, err := bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks) != 1 || len(page.Tasks[0].CollectionIDs) != 2 || page.Tasks[0].Difficulty != domain.DifficultyMedium {
		t.Fatalf("classification/membership not saved: %+v %v", page, err)
	}
	model, _ = updateApp(model, keypress('c'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyDown})
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Коллекция: Экзамен") {
		t.Fatal("collection filter missing")
	}
	model, _ = updateApp(model, keypress('s'))
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('t'))
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Предмет: Информатика") || !strings.Contains(model.View().Content, "Тема: Функции") {
		t.Fatal("subject/topic filters missing")
	}
	model, load = updateApp(model, keypress('x'))
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(model.View().Content, "Напишите цикл") {
		t.Fatal("task details unavailable")
	}
	model, _ = updateApp(model, keypress('e'))
	model = typeHomeworkText(model, "!")
	for i := 0; i < 4; i++ {
		model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	model, _ = updateApp(model, keypress(' '))
	model, save = updateApp(model, saveKey)
	model, load = updateApp(model, save())
	model, _ = updateApp(model, load())
	page, err = bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks[0].CollectionIDs) != 1 || !strings.Contains(page.Tasks[0].Content, "!") {
		t.Fatalf("bank task cannot be edited: %+v %v", page, err)
	}
	if list, err := students.List(ctx); err != nil || len(list) != 0 {
		t.Fatal("Task Bank created or required students")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(model.View().Content, "Ученики") {
		t.Fatal("Esc should leave Task Bank root")
	}
}
