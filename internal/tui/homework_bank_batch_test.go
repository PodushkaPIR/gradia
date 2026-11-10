package tui

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

func TestBankMarksKeepOrder(t *testing.T) {
	first := (taskBankModel{}).toggleMark(2).toggleMark(1)
	after := first.toggleMark(2).toggleMark(2)
	if !reflect.DeepEqual(first.marked, []domain.TaskBankTaskID{2, 1}) {
		t.Fatal("toggle mutated a previous model")
	}
	if !reflect.DeepEqual(after.marked, []domain.TaskBankTaskID{1, 2}) {
		t.Fatalf("reselected task should move to end: %v", after.marked)
	}
}

func TestBankBatchAcrossFilters(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "batch.db"))
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
	manual, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{
		Content: "Ручная задача", Topics: []string{"Повторение"}, Difficulty: domain.DifficultyEasy,
	})
	if err != nil {
		t.Fatal(err)
	}
	bank := app.NewTaskBankService(store)
	collection, err := bank.CreateCollection(ctx, "Алгебра")
	if err != nil {
		t.Fatal(err)
	}
	first, err := bank.Create(ctx, domain.TaskBankInput{
		Content: "Уравнение", Subject: "Математика", Topics: []string{"Уравнения"},
		Difficulty: domain.DifficultyEasy, CollectionIDs: []domain.TaskCollectionID{collection.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := bank.Create(ctx, domain.TaskBankInput{
		Content: "Цикл", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyHard,
	})
	if err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, students, nil, app.NewRouteService(store)).WithHomework(homework).WithTaskBank(bank)
	model, _ = updateApp(model, model.Init()())
	press := func(msg tea.Msg) {
		var cmd tea.Cmd
		model, cmd = updateApp(model, msg)
		if cmd != nil {
			model, _ = updateApp(model, cmd())
		}
	}
	for _, msg := range []tea.Msg{
		tea.KeyPressMsg{Code: tea.KeyEnter}, keypress('w'), tea.KeyPressMsg{Code: tea.KeyEnter},
		tea.WindowSizeMsg{Width: 80, Height: 40}, keypress('B'), keypress(' '),
	} {
		press(msg)
	}
	press(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(model.View().Content, "0/1 выполнено") || repo.copies != 0 {
		t.Fatal("canceling marked tasks should not write")
	}
	press(keypress('B'))
	picker := model.students.details.homework.bankPicker
	if !picker.keys.Mark.Enabled() || picker.keys.Export.Enabled() || len(picker.marked) != 0 {
		t.Fatal("Homework selection must be fresh and independent of ZIP export")
	}
	press(tea.KeyPressMsg{Code: tea.KeyDown})
	press(tea.KeyPressMsg{Code: tea.KeyEnter})
	press(keypress(' '))
	press(tea.KeyPressMsg{Code: tea.KeyEscape})
	press(tea.KeyPressMsg{Code: tea.KeyUp})
	press(keypress(' '))
	if !strings.Contains(model.View().Content, "Отмечено: 2") || !strings.Contains(model.students.ShortHelp(), "Space — отметить") {
		t.Fatal("selected count or marking shortcut missing")
	}
	for _, msg := range []tea.Msg{
		keypress('s'), tea.KeyPressMsg{Code: tea.KeyEnter},
		keypress('t'), tea.KeyPressMsg{Code: tea.KeyEnter},
		keypress('c'), tea.KeyPressMsg{Code: tea.KeyEnter},
	} {
		press(msg)
	}
	picker = model.students.details.homework.bankPicker
	wantIDs := []domain.TaskBankTaskID{second.ID, first.ID}
	wantFilter := domain.TaskBankFilter{Subject: "Информатика", Topic: "Циклы", CollectionID: collection.ID}
	if !reflect.DeepEqual(picker.marked, wantIDs) || picker.filter != wantFilter || len(picker.data.Tasks) != 0 {
		t.Fatalf("selection lost across empty filters: %+v %v", picker.filter, picker.marked)
	}
	model, save := updateApp(model, keypress('a'))
	if save == nil || repo.copies != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("batch should execute in Cmd even when marked tasks are hidden")
	}
	for _, msg := range []tea.Msg{keypress('a'), keypress(' '), tea.KeyPressMsg{Code: tea.KeyEscape}} {
		var duplicate tea.Cmd
		model, duplicate = updateApp(model, msg)
		if duplicate != nil || model.screenInputMode() != inputBusy {
			t.Fatal("input escaped or duplicated busy batch")
		}
	}
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 80, Height: 32})
	model, _ = updateApp(model, save())
	picker = model.students.details.homework.bankPicker
	if picker == nil || !reflect.DeepEqual(picker.marked, wantIDs) || picker.filter != wantFilter ||
		!strings.Contains(model.View().Content, "Не удалось добавить") {
		t.Fatal("failed batch lost selection, filters, or retry error")
	}
	if tasks, err := homework.Tasks(ctx, student.ID, assignment.ID); err != nil || len(tasks) != 1 {
		t.Fatalf("failed batch changed Homework: %+v %v", tasks, err)
	}
	repo.copyErr = nil
	model, save = updateApp(model, keypress('a'))
	if _, err := bank.Edit(ctx, first.ID, domain.TaskBankInput{
		Content: "Новая версия уравнения", Subject: first.Subject, Topics: first.Topics, Difficulty: first.Difficulty,
	}); err != nil {
		t.Fatal(err)
	}
	model, _ = updateApp(model, save())
	if model.students.details.homework.bankPicker != nil || !strings.Contains(model.View().Content, "0/3 выполнено") {
		t.Fatal("successful batch did not return to updated Homework")
	}
	tasks, err := homework.Tasks(ctx, student.ID, assignment.ID)
	if err != nil || len(tasks) != 3 || tasks[0].ID != manual.ID || tasks[1].Content != second.Content || tasks[2].Content != "Новая версия уравнения" {
		t.Fatalf("batch lost selection order, latest content, or existing tasks: %+v %v", tasks, err)
	}
}
