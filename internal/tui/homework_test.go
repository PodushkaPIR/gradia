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

type homeworkTestStore struct {
	*sqlite.Store
	saveErr, completeErr error
}

func (s *homeworkTestStore) SaveHomeworkTask(ctx context.Context, student domain.StudentID, id domain.HomeworkID, task domain.HomeworkTaskID, input domain.HomeworkTaskInput, at time.Time) (domain.HomeworkTask, error) {
	if s.saveErr != nil {
		return domain.HomeworkTask{}, s.saveErr
	}
	return s.Store.SaveHomeworkTask(ctx, student, id, task, input, at)
}

func (s *homeworkTestStore) SetHomeworkTaskCompleted(ctx context.Context, student domain.StudentID, id domain.HomeworkID, task domain.HomeworkTaskID, completed bool, at time.Time) (domain.HomeworkTask, error) {
	if s.completeErr != nil {
		return domain.HomeworkTask{}, s.completeErr
	}
	return s.Store.SetHomeworkTaskCompleted(ctx, student, id, task, completed, at)
}

func typeHomeworkText(model App, text string) App {
	for _, r := range text {
		if r == '\n' {
			model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
		} else {
			model, _ = updateApp(model, keypress(r))
		}
	}
	return model
}

func TestTutorManagesHomeworkAndTaskCompletionThroughTUI(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "homework.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	repo := &homeworkTestStore{Store: store}
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	homework := app.NewHomeworkService(repo)
	model := NewApp(ctx, students, nil, app.NewRouteService(store)).WithHomework(homework)
	model, _ = updateApp(model, model.Init()())
	model, cmd := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, cmd())
	model, cmd = updateApp(model, keypress('w'))
	if cmd == nil {
		t.Fatal("w should open student Homework")
	}
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "Домашние задания") {
		t.Fatal("Homework screen missing")
	}
	model, _ = updateApp(model, keypress('n'))
	model = typeHomeworkText(model, "Практика")
	model, cmd = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || model.screenInputMode() != inputBusy {
		t.Fatal("Homework save must run in Cmd")
	}
	if list, err := homework.List(ctx, student.ID); err != nil || len(list) != 0 {
		t.Fatal("Homework was saved before Cmd")
	}
	model, _ = updateApp(model, cmd())
	model, cmd = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, cmd())
	model, _ = updateApp(model, keypress('n'))
	model = typeHomeworkText(model, "Задача q/p\nНапишите цикл")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model = typeHomeworkText(model, "Циклы; Функции")
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyTab})
	model, _ = updateApp(model, keypress('3'))
	repo.saveErr = errors.New("disk unavailable")
	saveKey := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	model, cmd = updateApp(model, saveKey)
	model, duplicate := updateApp(model, saveKey)
	if duplicate != nil {
		t.Fatal("duplicate task save")
	}
	model, _ = updateApp(model, cmd())
	if model.screenInputMode() != inputText || !strings.Contains(model.View().Content, "Не удалось сохранить задачу") || !strings.Contains(model.View().Content, "Циклы; Функции") {
		t.Fatal("failed task save lost editor input")
	}
	repo.saveErr = nil
	model, cmd = updateApp(model, saveKey)
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "0/1 выполнено") || !strings.Contains(model.View().Content, "сложная") || !strings.Contains(model.View().Content, "Напишите цикл") {
		t.Fatalf("saved task not shown:\n%s", model.View().Content)
	}
	model, cmd = updateApp(model, keypress(' '))
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "1/1 выполнено") {
		t.Fatal("individual task completion not reflected")
	}
	model, _ = updateApp(model, keypress('e'))
	model = typeHomeworkText(model, "!")
	model, cmd = updateApp(model, saveKey)
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "1/1 выполнено") {
		t.Fatal("editing task reset completion")
	}
	model, _ = updateApp(model, keypress('t'))
	model = typeHomeworkText(model, "!")
	model, cmd = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "Практика!") || !strings.Contains(model.View().Content, "1/1 выполнено") {
		t.Fatal("rename lost Homework summary")
	}
	repo.completeErr = errors.New("write failed")
	model, cmd = updateApp(model, keypress(' '))
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "1/1 выполнено") || !strings.Contains(model.View().Content, "Не удалось") {
		t.Fatal("failed completion update changed state")
	}
	repo.completeErr = nil
	model, cmd = updateApp(model, keypress(' '))
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "0/1 выполнено") {
		t.Fatal("completion cannot be cleared")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(model.View().Content, "Практика!") || !strings.Contains(model.View().Content, "0/1 выполнено") {
		t.Fatal("task counts missing in Homework list")
	}
}

func deliverHomeworkCommand(model homeworkModel, cmd tea.Cmd) homeworkModel {
	msg := cmd().(screenMsg).payload.(studentDetailsMsg).payload.(homeworkMsg).payload
	model, _ = model.Update(msg)
	return model
}

func TestHomeworkIgnoresStaleTaskLoadsAndCancelsUnsavedEditors(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "routing.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewHomeworkService(store)
	first, err := service.Create(ctx, student.ID, "Первое ДЗ")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, student.ID, "Второе ДЗ")
	if err != nil {
		t.Fatal(err)
	}
	wantTopics := []string{"Циклы; вложенные", "Тема \"цитата\"", "Класс\nи функции"}
	for _, item := range []struct {
		id      domain.HomeworkID
		content string
	}{{first.ID, "Задача А"}, {second.ID, "Задача Б"}} {
		if _, err := service.AddTask(ctx, student.ID, item.id, domain.HomeworkTaskInput{Content: item.content, Topics: wantTopics, Difficulty: domain.DifficultyEasy}); err != nil {
			t.Fatal(err)
		}
	}
	model := homeworkModel{ctx: ctx, service: service, student: student.ID, width: 80, height: 12, keys: newHomeworkKeyMap()}
	model, load := model.reload()
	model = deliverHomeworkCommand(model, load)
	model, _ = model.Update(keypress('n'))
	model, _ = model.Update(keypress('!'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !model.atRoot() || model.InputMode() != inputNavigation {
		t.Fatal("cancel should close only the name editor")
	}
	model, firstLoad := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = model.Update(keypress('j'))
	model, secondLoad := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = deliverHomeworkCommand(model, firstLoad)
	if !strings.Contains(model.View(), "Второе ДЗ") || !strings.Contains(model.View(), "Загрузка") {
		t.Fatal("old task response replaced the selected Homework")
	}
	model = deliverHomeworkCommand(model, secondLoad)
	if !strings.Contains(model.View(), "Задача Б") || strings.Contains(model.View(), "Задача А") {
		t.Fatal("wrong tasks displayed")
	}
	model, _ = model.Update(homeworkLoadFailedMsg{id: first.ID, err: errors.New("old load failed"), seq: model.loadSeq})
	if strings.Contains(model.View(), "Не удалось") {
		t.Fatal("old load error poisoned the current Homework")
	}
	model, previousLoad := model.Update(keypress('r'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model, currentLoad := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = deliverHomeworkCommand(model, previousLoad)
	if !strings.Contains(model.View(), "Загрузка") {
		t.Fatal("old response for reopened Homework ended the new load")
	}
	model = deliverHomeworkCommand(model, currentLoad)
	model, _ = model.Update(keypress('e'))
	model, _ = model.Update(keypress('!'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(model.View(), "Задача Б") || model.InputMode() != inputNavigation {
		t.Fatal("cancel should keep the task view open")
	}
	if actual, err := service.Tasks(ctx, student.ID, second.ID); err != nil || len(actual) != 1 || actual[0].Content != "Задача Б" {
		t.Fatalf("cancel changed stored task: %+v %v", actual, err)
	}
	if list, err := service.List(ctx, student.ID); err != nil || len(list) != 2 {
		t.Fatalf("cancel created Homework: %+v %v", list, err)
	}
	model, _ = model.Update(keypress('e'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	model, _ = model.Update(keypress('2'))
	model, save := model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model = deliverHomeworkCommand(model, save)
	if actual, err := service.Tasks(ctx, student.ID, second.ID); err != nil || len(actual) != 1 || !reflect.DeepEqual(actual[0].Topics, wantTopics) || actual[0].Difficulty != domain.DifficultyMedium {
		t.Fatalf("editing difficulty changed topic labels: %+v %v", actual, err)
	}
}
