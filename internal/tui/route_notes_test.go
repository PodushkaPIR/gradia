package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type notesRepoStub struct {
	progressRepoStub
	noteWrites int
	noteErr    error
}

func (r *notesRepoStub) SetRouteTopicNote(_ context.Context, route domain.RouteID, id domain.RouteTopicID, value string, at time.Time) error {
	r.noteWrites++
	if r.noteErr != nil {
		return r.noteErr
	}
	for i := range r.topics {
		if r.topics[i].ID == id && r.topics[i].RouteID == route {
			r.topics[i].Notes = value
			return nil
		}
	}
	return domain.ErrRouteTopicNotFound
}

func TestRouteNoteEditorSaveCancelAndErrors(t *testing.T) {
	repo := &notesRepoStub{progressRepoStub: progressRepoStub{topics: []domain.RouteTopic{{ID: 1, RouteID: 10, Name: "Циклы", Notes: "Прежняя заметка", Status: domain.ProgressLearning}}}}
	repo.routes = []domain.Route{{ID: 10, StudentID: 1, Name: "Python"}}
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}), nil, app.NewRouteService(repo))
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Прежняя заметка") {
		t.Fatal("при выборе темы должен отображаться предпросмотр")
	}
	model, _ = updateApp(model, keypress('e'))
	if model.screenInputMode() != inputText || model.students.details.view.note.text.Value() != "Прежняя заметка" || !strings.Contains(model.View().Content, "Ctrl+S — сохранить") {
		t.Fatal("e должен открывать редактор с существующей заметкой")
	}
	model, _ = updateApp(model, keypress('q'))
	model, _ = updateApp(model, keypress('?'))
	if model.help != nil || model.students.details.view.note == nil {
		t.Fatal("q и ? должны вводиться как текст")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.details.view == nil || model.students.details.view.note != nil || repo.noteWrites != 0 || repo.topics[0].Notes != "Прежняя заметка" {
		t.Fatal("отмена не должна закрывать маршрут или менять заметку")
	}
	model, _ = updateApp(model, keypress('e'))
	markdown := "# Ошибки\n\n- Повторить `for`\n- O'Reilly"
	model.students.details.view.note.text.SetValue(markdown)
	repo.noteErr = errors.New("write failed")
	saveKey := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	model, save := updateApp(model, saveKey)
	if save == nil || repo.noteWrites != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("сохранение должно выполняться через Cmd")
	}
	model, duplicate := updateApp(model, saveKey)
	if duplicate != nil {
		t.Fatal("повторный Ctrl+S запустил дублирующую запись")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.details.view.note == nil {
		t.Fatal("нельзя отменять выполняющееся сохранение")
	}
	model, _ = updateApp(model, save())
	if model.screenInputMode() != inputText || model.students.details.view.note.text.Value() != markdown || !strings.Contains(model.View().Content, "Не удалось сохранить") {
		t.Fatal("ошибка должна сохранять текст в редакторе")
	}
	repo.noteErr = nil
	model, save = updateApp(model, saveKey)
	model, _ = updateApp(model, save())
	if model.students.details.view.note != nil || model.students.details.view.rows[0].topic.Notes != markdown || model.students.details.view.rows[0].topic.Status != domain.ProgressLearning || repo.writes != 0 {
		t.Fatal("сохранение заметки не должно менять статус")
	}
	model, load = updateApp(model, keypress('r'))
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('e'))
	if model.students.details.view.note.text.Value() != markdown {
		t.Fatal("после загрузки редактор должен получать сохранённый Markdown")
	}
}
