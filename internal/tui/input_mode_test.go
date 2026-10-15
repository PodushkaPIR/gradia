package tui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestInputModeRules(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		mode                             inputMode
		help, quit, switchScreen, cancel bool
	}{
		{name: "navigation", mode: inputNavigation, help: true, quit: true, switchScreen: true},
		{name: "text", mode: inputText, cancel: true},
		{name: "confirmation", mode: inputConfirmation, help: true, cancel: true},
		{name: "busy", mode: inputBusy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mode.allowsHelp() != tc.help || tc.mode.allowsQuit() != tc.quit || tc.mode.allowsScreenSwitch() != tc.switchScreen || tc.mode.allowsCancel() != tc.cancel {
				t.Fatal("правила режима ввода не соответствуют контексту")
			}
		})
	}
}

func TestScreensReportNestedInputMode(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model interface{ InputMode() inputMode }
		want  inputMode
	}{
		{name: "student loading", model: studentsModel{loading: true}, want: inputNavigation},
		{name: "template loading", model: templatesModel{loading: true}, want: inputNavigation},
		{name: "topics loading", model: templateTopicsModel{loading: true}, want: inputNavigation},
		{name: "student input", model: studentsModel{form: &nameForm{}}, want: inputText},
		{name: "student saving", model: studentsModel{form: &nameForm{saving: true}}, want: inputBusy},
		{name: "student confirmation", model: studentsModel{deleteDialog: &studentDeleteModel{}}, want: inputConfirmation},
		{name: "student deleting", model: studentsModel{deleteDialog: &studentDeleteModel{deleting: true}}, want: inputBusy},
		{name: "template input", model: templatesModel{form: &nameForm{}}, want: inputText},
		{name: "template saving", model: templatesModel{form: &nameForm{saving: true}}, want: inputBusy},
		{name: "nested topic input", model: templatesModel{topics: &templateTopicsModel{form: &nameForm{}}}, want: inputText},
		{name: "nested topic saving", model: templatesModel{topics: &templateTopicsModel{form: &nameForm{saving: true}}}, want: inputBusy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.model.InputMode(); got != tc.want {
				t.Fatalf("mode=%v, want=%v", got, tc.want)
			}
		})
	}
}

func TestInputModeTransitionsAndGlobalKeys(t *testing.T) {
	repo := &studentRepoStub{}
	model := NewApp(context.Background(), app.NewStudentService(repo), app.NewTemplateService(&templateRepoStub{}), nil)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('n'))
	if model.screenInputMode() != inputText {
		t.Fatal("открытая форма должна сообщать режим ввода текста")
	}
	for _, key := range []rune{'q', '?', 'p'} {
		model, _ = updateApp(model, keypress(key))
	}
	if model.students.form.name.Value() != "q?p" || model.help != nil || model.showTemplates {
		t.Fatal("глобальные клавиши должны вводиться как символы")
	}
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.screenInputMode() != inputBusy || save == nil {
		t.Fatal("сохранение должно переводить форму в busy")
	}
	for _, key := range []tea.KeyPressMsg{keypress('n'), keypress('r'), keypress('p'), keypress('q'), keypress('?'), {Code: tea.KeyEnter}, {Code: tea.KeyEscape}} {
		var cmd tea.Cmd
		model, cmd = updateApp(model, key)
		if cmd != nil || model.screenInputMode() != inputBusy || model.help != nil || model.showTemplates || model.students.form.name.Value() != "q?p" {
			t.Fatalf("клавиша %q нарушила busy-режим", key.String())
		}
	}
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 30, Height: 20})
	if model.students.form.name.Width() != 26 {
		t.Fatal("busy не должен блокировать resize")
	}
	model, reload := updateApp(model, save())
	if model.screenInputMode() != inputNavigation || !model.students.loading || reload == nil {
		t.Fatal("после сохранения фоновая загрузка должна разрешать навигацию")
	}
	model, _ = updateApp(model, reload())
	model, _ = updateApp(model, keypress('n'))
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.screenInputMode() != inputText || model.students.form.err == nil {
		t.Fatal("ошибка сохранения должна возвращать режим ввода")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.screenInputMode() != inputNavigation {
		t.Fatal("отмена формы должна возвращать навигацию")
	}
	model, _ = updateApp(model, keypress('d'))
	if model.screenInputMode() != inputConfirmation {
		t.Fatal("диалог удаления должен сообщать confirmation")
	}
	for _, key := range []rune{'q', 'p'} {
		var cmd tea.Cmd
		model, cmd = updateApp(model, keypress(key))
		if cmd != nil || model.showTemplates || model.screenInputMode() != inputConfirmation {
			t.Fatal("подтверждение не должно разрешать выход по q или смену экрана")
		}
	}
	model, _ = updateApp(model, keypress('?'))
	if model.help == nil {
		t.Fatal("из подтверждения должна открываться помощь")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	repo.deleteErr = errors.New("storage failed")
	model, remove := updateApp(model, keypress('y'))
	if model.screenInputMode() != inputBusy {
		t.Fatal("удаление должно сообщать busy")
	}
	model, _ = updateApp(model, remove())
	if model.screenInputMode() != inputConfirmation || model.students.deleteDialog.err == nil {
		t.Fatal("ошибка удаления должна возвращать подтверждение")
	}
	repo.deleteErr = nil
	model, remove = updateApp(model, keypress('y'))
	model, _ = updateApp(model, remove())
	if model.screenInputMode() != inputNavigation || len(repo.students) != 0 {
		t.Fatal("успешное удаление должно возвращать навигацию")
	}
}

func TestNestedTopicInputModeAndBack(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}}
	model := openTemplatesForTopics(t, repo)
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('n'))
	if model.screenInputMode() != inputText {
		t.Fatal("root должен получать режим вложенной формы")
	}
	model.templates.topics.form.name.SetValue("Циклы")
	repo.topicCreateErr = errors.New("storage failed")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.screenInputMode() != inputBusy {
		t.Fatal("root должен получать busy от вложенной формы")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics.form == nil || !model.showTemplates {
		t.Fatal("busy должен блокировать уход из вложенной формы")
	}
	model, _ = updateApp(model, save())
	if model.screenInputMode() != inputText {
		t.Fatal("ответ с ошибкой должен вернуть вложенную форму в text")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics == nil || model.screenInputMode() != inputNavigation {
		t.Fatal("отмена формы должна оставить экран тем")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics != nil || !model.showTemplates {
		t.Fatal("следующий Esc должен вернуть к шаблонам")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.showTemplates || model.screenInputMode() != inputNavigation {
		t.Fatal("Esc из корня шаблонов должен вернуть к ученикам")
	}
}
