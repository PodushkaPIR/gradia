package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestEditStudentFormFlow(t *testing.T) {
	repo := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Анна"}, {ID: 2, Name: "Иван"}}}
	model := NewApp(context.Background(), app.NewStudentService(repo))
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('j'))
	model, _ = updateApp(model, keypress('e'))
	if model.form == nil || model.form.studentID != 2 || model.form.name.Value() != "Иван" || !model.form.name.Focused() {
		t.Fatal("форма должна открыться с именем выбранного ученика")
	}
	if !strings.Contains(model.View().Content, "Редактирование ученика") {
		t.Fatal("не отображён режим редактирования")
	}
	model, _ = updateApp(model, keypress('e'))
	model, _ = updateApp(model, keypress('q'))
	if model.form.name.Value() != "Иванeq" {
		t.Fatal("горячие клавиши перехватили ввод в форме")
	}
	model.form.name.SetValue("  Аарон  ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if save == nil || !model.form.saving || repo.renameCalls != 0 {
		t.Fatal("обновление должно запускаться только через Cmd")
	}
	model, duplicate := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("повторный Enter запустил дублирующее обновление")
	}
	model, reload := updateApp(model, save())
	if model.form != nil || reload == nil || repo.renameCalls != 1 || repo.createCalls != 0 {
		t.Fatal("редактирование должно обновить ученика и закрыть форму")
	}
	model, _ = updateApp(model, reload())
	selected, ok := model.students.selectedStudent()
	if !ok || selected.ID != 2 || selected.Name != "Аарон" || model.students.cursor != 0 || len(repo.students) != 2 {
		t.Fatalf("выбор потерян после изменения порядка: selected=%+v, cursor=%d", selected, model.students.cursor)
	}
}

func TestEditStudentCancelAndErrors(t *testing.T) {
	repo := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Анна"}}}
	model := NewApp(context.Background(), app.NewStudentService(repo))
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('e'))
	model.form.name.SetValue("Другое имя")
	model, cmd := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil || model.form != nil || repo.renameCalls != 0 || repo.students[0].Name != "Анна" {
		t.Fatal("Esc должен закрывать форму без изменения имени")
	}
	model, _ = updateApp(model, keypress('e'))
	model.form.name.SetValue("  ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.form == nil || model.form.saving || !errors.Is(model.form.err, domain.ErrStudentNameRequired) || repo.renameCalls != 0 {
		t.Fatal("пустое имя должно остаться в форме с ошибкой")
	}
	model.form.name.SetValue("Новое имя")
	repo.renameErr = errors.New("storage failed")
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.form == nil || model.form.saving || model.form.name.Value() != "Новое имя" || repo.students[0].Name != "Анна" {
		t.Fatal("ошибка сохранения должна оставить ввод в форме и прежние данные в списке")
	}
	repo.renameErr = nil
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, reload := updateApp(model, save())
	model, _ = updateApp(model, reload())
	if model.form != nil || repo.students[0].Name != "Новое имя" || repo.createCalls != 0 {
		t.Fatal("повторная попытка должна обновить существующего ученика")
	}
}

func TestEditStudentUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		loading bool
		err     error
	}{
		{name: "loading", loading: true},
		{name: "empty"},
		{name: "load failed", err: errors.New("storage failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{}))
			model.students.loading = tc.loading
			model.students.err = tc.err
			model, cmd := updateApp(model, keypress('e'))
			if model.form != nil || cmd != nil {
				t.Fatal("нельзя открыть редактирование без выбранного ученика")
			}
		})
	}
}
