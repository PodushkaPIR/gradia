package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

type homeworkBankTestStore struct {
	*sqlite.Store
	copies  int
	copyErr error
}

func (s *homeworkBankTestStore) SnapshotBankTaskToHomework(ctx context.Context, student domain.StudentID, id domain.HomeworkID, source domain.TaskBankTaskID, at time.Time) (domain.HomeworkTask, error) {
	s.copies++
	if s.copyErr != nil {
		return domain.HomeworkTask{}, s.copyErr
	}
	return s.Store.SnapshotBankTaskToHomework(ctx, student, id, source, at)
}

func TestTutorSelectsBankTaskWhileComposingHomeworkAndRetriesSnapshot(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "picker.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	repo := &homeworkBankTestStore{Store: store, copyErr: errors.New("write failed")}
	homework := app.NewHomeworkService(repo)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	bank := app.NewTaskBankService(store)
	input := domain.TaskBankInput{Content: "Задача из банка", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyHard}
	source, err := bank.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bank.Create(ctx, domain.TaskBankInput{Content: "Другая задача", Subject: "Математика", Topics: []string{"Дроби"}, Difficulty: domain.DifficultyEasy}); err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, students, nil, app.NewRouteService(store)).WithHomework(homework).WithTaskBank(bank)
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, load = updateApp(model, keypress('w'))
	model, _ = updateApp(model, load())
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 80, Height: 32})
	model, stale := updateApp(model, keypress('B'))
	if stale == nil || model.taskBank != nil {
		t.Fatal("B should open nested bank picker, not the global bank")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(model.View().Content, "0/0 выполнено") {
		t.Fatal("cancel should return to the same Homework")
	}
	model, load = updateApp(model, keypress('B'))
	model, _ = updateApp(model, stale())
	if !strings.Contains(model.View().Content, "Загрузка") {
		t.Fatal("stale picker load reached new picker")
	}
	model, _ = updateApp(model, load())
	for _, r := range []rune{'n', 'e', 'E', 'I'} {
		var command tea.Cmd
		model, command = updateApp(model, keypress(r))
		if command != nil || model.screenInputMode() != inputNavigation {
			t.Fatalf("picker accepted editing/export action %q", r)
		}
	}
	model, _ = updateApp(model, keypress('s'))
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	if strings.Contains(model.View().Content, "Другая задача") {
		t.Fatal("subject filtering not applied")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(model.View().Content, "сложная") || !strings.Contains(model.View().Content, "Циклы") {
		t.Fatal("bank preview missing metadata")
	}
	model, copy := updateApp(model, keypress('a'))
	if copy == nil || repo.copies != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("snapshot must execute in Cmd")
	}
	model, duplicate := updateApp(model, keypress('a'))
	if duplicate != nil {
		t.Fatal("duplicate assignment while busy")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.screenInputMode() != inputBusy {
		t.Fatal("escaped while copying")
	}
	model, _ = updateApp(model, copy())
	if !strings.Contains(model.View().Content, "Не удалось добавить") || !strings.Contains(model.View().Content, "Задача из банка") {
		t.Fatal("failure lost picker or retry selection")
	}
	if tasks, err := homework.Tasks(ctx, student.ID, assignment.ID); err != nil || len(tasks) != 0 {
		t.Fatalf("failed assignment changed Homework: %+v %v", tasks, err)
	}
	repo.copyErr = nil
	model, copy = updateApp(model, keypress('a'))
	input.Content = "Актуальная версия"
	if _, err := bank.Edit(ctx, source.ID, input); err != nil {
		t.Fatal(err)
	}
	model, _ = updateApp(model, copy())
	if !strings.Contains(model.View().Content, "Актуальная версия") || !strings.Contains(model.View().Content, "0/1 выполнено") {
		t.Fatal("snapshot not returned to selected Homework")
	}
	model, complete := updateApp(model, keypress(' '))
	model, _ = updateApp(model, complete())
	if !strings.Contains(model.View().Content, "1/1 выполнено") {
		t.Fatal("assigned task is not independently completable")
	}
}
