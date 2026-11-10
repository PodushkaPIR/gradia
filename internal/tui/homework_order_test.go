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
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

type homeworkOrderStore struct {
	*sqlite.Store
	err   error
	moves int
}

func (s *homeworkOrderStore) SwapHomeworkTasks(
	ctx context.Context,
	student domain.StudentID,
	homework domain.HomeworkID,
	first, second domain.HomeworkTaskID,
	at time.Time,
) ([]domain.HomeworkTaskPosition, error) {
	s.moves++
	if s.err != nil {
		return nil, s.err
	}
	return s.Store.SwapHomeworkTasks(ctx, student, homework, first, second, at)
}

func TestHomeworkReorderWorkflow(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "order.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	repo := &homeworkOrderStore{Store: store, err: errors.New("write failed")}
	homework := app.NewHomeworkService(repo)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"Первая", "Вторая"} {
		if _, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{
			Content: content, Topics: []string{"Геометрия"}, Difficulty: domain.DifficultyEasy,
		}); err != nil {
			t.Fatal(err)
		}
	}
	model := NewApp(ctx, students, nil, app.NewRouteService(store)).WithHomework(homework)
	model, _ = updateApp(model, model.Init()())
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: tea.KeyEnter}, keypress('w')} {
		var cmd tea.Cmd
		model, cmd = updateApp(model, msg)
		model, _ = updateApp(model, cmd())
	}
	if strings.Contains(model.students.ShortHelp(), "выше") {
		t.Fatal("task order controls should not appear in Homework list")
	}
	model, cmd := updateApp(model, keypress('K'))
	if cmd != nil || repo.moves != 0 {
		t.Fatal("cannot reorder without selecting Homework")
	}
	model, cmd = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, cmd())
	state := func() *homeworkModel { return model.students.details.homework }
	if !strings.Contains(model.students.ShortHelp(), "K / Alt+↑ — выше") ||
		!strings.Contains(model.contextHelp().View(), "Переставить задачу ниже") {
		t.Fatal("reorder controls missing from help")
	}
	model, cmd = updateApp(model, keypress('K'))
	if cmd != nil || repo.moves != 0 {
		t.Fatal("moving first task up should be a no-op")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyDown})
	before := *state()
	model, cmd = updateApp(model, keypress('K'))
	if cmd == nil || repo.moves != 0 || model.screenInputMode() != inputBusy || state().cursor != 1 ||
		!reflect.DeepEqual(state().tasks, before.tasks) {
		t.Fatal("order must not change before Cmd succeeds")
	}
	for _, msg := range []tea.Msg{
		keypress('K'), keypress('J'), keypress(' '), keypress('n'), keypress('r'),
		keypress('q'), keypress('p'), tea.KeyPressMsg{Code: tea.KeyEscape},
	} {
		var duplicate tea.Cmd
		model, duplicate = updateApp(model, msg)
		if duplicate != nil || model.screenInputMode() != inputBusy || state().selected == nil {
			t.Fatal("busy move accepted duplicate input or navigation")
		}
	}
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 80, Height: 24})
	model, _ = updateApp(model, cmd())
	if repo.moves != 1 || model.screenInputMode() != inputNavigation || state().cursor != 1 ||
		!reflect.DeepEqual(state().tasks, before.tasks) || !strings.Contains(model.View().Content, "Не удалось сохранить изменение") {
		t.Fatal("failed move changed visible order or lost retry message")
	}
	repo.err = nil
	model, cmd = updateApp(model, tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModAlt})
	if cmd == nil || repo.moves != 1 {
		t.Fatal("Alt+Up must schedule retry, not write immediately")
	}
	model, _ = updateApp(model, cmd())
	if state().cursor != 0 || state().tasks[0].ID != before.tasks[1].ID || state().saveErr != nil ||
		before.tasks[0].Content != "Первая" {
		t.Fatal("successful move lost selection, retained error, or mutated old model")
	}
	persisted, err := homework.Tasks(ctx, student.ID, assignment.ID)
	if err != nil || persisted[0].ID != before.tasks[1].ID {
		t.Fatalf("visible order was not persisted: %+v %v", persisted, err)
	}
	model, cmd = updateApp(model, tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModAlt})
	if cmd == nil {
		t.Fatal("Alt+Down should move selected task down")
	}
	model, _ = updateApp(model, cmd())
	if state().cursor != 1 || state().tasks[1].ID != before.tasks[1].ID {
		t.Fatal("moving down should follow the same selected task")
	}
	model, cmd = updateApp(model, keypress('J'))
	if cmd != nil {
		t.Fatal("moving last task down should be a no-op")
	}
	model, cmd = updateApp(model, keypress('K'))
	model, _ = updateApp(model, cmd())
	state().keys.MoveDown.SetKeys("x")
	state().keys.MoveDown.SetHelp("x", state().keys.MoveDown.Help().Desc)
	model, cmd = updateApp(model, keypress('J'))
	if cmd != nil || !strings.Contains(model.students.ShortHelp(), "x — ниже") {
		t.Fatal("actions and help must use the same configured binding")
	}
	model, cmd = updateApp(model, keypress('x'))
	if cmd == nil {
		t.Fatal("rebound move key should work")
	}
	model, _ = updateApp(model, cmd())
	if state().cursor != 1 || state().tasks[1].ID != before.tasks[1].ID {
		t.Fatal("rebound key lost selection")
	}
}

func TestHomeworkIgnoresStaleOrder(t *testing.T) {
	model := homeworkModel{
		selected: &domain.Homework{ID: 1}, saving: true,
		tasks: []domain.HomeworkTask{{ID: 10, Position: 0}, {ID: 20, Position: 1}},
	}
	after := model.applyTaskOrder(homeworkTasksMovedMsg{
		homework: 2, selected: 20,
		positions: []domain.HomeworkTaskPosition{{TaskID: 20, Position: 0}, {TaskID: 10, Position: 1}},
	})
	if !reflect.DeepEqual(after, model) {
		t.Fatal("stale response changed another Homework")
	}
	model.selected = nil
	after = model.applyTaskOrder(homeworkTasksMovedMsg{homework: 1})
	if !reflect.DeepEqual(after, model) {
		t.Fatal("stale response changed Homework list")
	}
}
