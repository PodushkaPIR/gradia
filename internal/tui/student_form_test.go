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

func updateApp(model App, msg tea.Msg) (App, tea.Cmd) {
	updated, cmd := model.Update(msg)
	return updated.(App), cmd
}

func keypress(key rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: key, Text: string(key)}
}

func TestCreateStudentFormFlow(t *testing.T) {
	repo := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Анна"}}}
	model := NewApp(context.Background(), app.NewStudentService(repo), nil, nil)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('n'))
	if model.students.form == nil || !model.students.form.name.Focused() {
		t.Fatal("форма не открылась или поле не получило фокус")
	}

	// Буквы глобальных shortcuts должны вводиться, а не управлять приложением.
	model, _ = updateApp(model, keypress('q'))
	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, keypress('r'))
	if model.students.form.name.Value() != "qnr" || model.students.cursor != 0 {
		t.Fatal("shortcuts перехватили ввод текста")
	}
	model.students.form.name.SetValue("  Иван  ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if save == nil || !model.students.form.saving || repo.createCalls != 0 {
		t.Fatal("сохранение должно запускаться только внутри Cmd")
	}
	model, duplicate := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("повторный Enter запустил дублирующее сохранение")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.form == nil {
		t.Fatal("нельзя отменять уже выполняющееся сохранение")
	}
	model, reload := updateApp(model, save())
	if model.students.form != nil || reload == nil || !model.students.loading || repo.createCalls != 1 {
		t.Fatal("после сохранения ожидаются закрытие формы и загрузка списка")
	}
	model, _ = updateApp(model, reload())
	if model.students.cursor != 1 || !strings.Contains(model.View().Content, "> Иван") {
		t.Fatal("созданный ученик не выбран в списке")
	}
}

func TestStudentFormErrorsAndCancel(t *testing.T) {
	repo := &studentRepoStub{}
	model := NewApp(context.Background(), app.NewStudentService(repo), nil, nil)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('n'))
	model.students.form.name.SetValue("   ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.students.form == nil || model.students.form.saving || !errors.Is(model.students.form.err, domain.ErrStudentNameRequired) || repo.createCalls != 0 {
		t.Fatal("пустое имя должно остаться в форме с ошибкой и без записи в БД")
	}

	model.students.form.name.SetValue("Иван")
	repo.createErr = errors.New("database unavailable")
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.students.form == nil || model.students.form.saving || model.students.form.name.Value() != "Иван" || !strings.Contains(model.View().Content, "Не удалось сохранить") {
		t.Fatal("ошибка хранилища должна сохранять введённое имя")
	}

	repo.createErr = nil
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, reload := updateApp(model, save())
	model, _ = updateApp(model, reload())
	if len(repo.students) != 1 || model.students.form != nil {
		t.Fatal("не удалось повторить сохранение после ошибки")
	}

	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, keypress('x'))
	model, cancel := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.form != nil || cancel != nil || len(repo.students) != 1 {
		t.Fatal("Esc должен закрыть форму без сохранения")
	}

	model, _ = updateApp(model, keypress('n'))
	_, quit := updateApp(model, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if quit == nil {
		t.Fatal("Ctrl+C должен работать внутри формы")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("ожидалась команда выхода")
	}
}
