package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestHelpCapturesKeysAndPreservesScreen(t *testing.T) {
	repo := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Анна"}, {ID: 2, Name: "Иван"}}}
	model := NewApp(context.Background(), app.NewStudentService(repo), app.NewTemplateService(&templateRepoStub{}), nil)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('j'))
	model, _ = updateApp(model, keypress('?'))
	if model.help == nil || !strings.Contains(model.View().Content, "Удалить с подтверждением") {
		t.Fatal("помощь должна показывать команды текущего экрана")
	}
	for _, key := range []rune{'n', 'e', 'd', 'p', 'r', 'q', 'j', 'k'} {
		var cmd tea.Cmd
		model, cmd = updateApp(model, keypress(key))
		if cmd != nil || model.students.form != nil || model.students.deleteDialog != nil || model.showTemplates || model.students.cursor != 1 {
			t.Fatalf("помощь пропустила клавишу %q в основной экран", key)
		}
	}
	// Асинхронные данные продолжают доставляться, не закрывая помощь.
	model, _ = updateApp(model, routeToStudents(studentsLoadedMsg{students: repo.students}))
	if model.help == nil {
		t.Fatal("результат загрузки закрыл помощь")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.help != nil || !strings.Contains(model.View().Content, "> Иван") {
		t.Fatal("Esc должен возвращать на прежний экран с прежним выбором")
	}
	model, load := updateApp(model, keypress('p'))
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('?'))
	view := model.View().Content
	if !strings.Contains(view, "Помощь — Шаблоны обучения") || strings.Contains(view, "Редактировать имя") || strings.Contains(view, "Удалить с подтверждением") {
		t.Fatal("помощь шаблонов содержит команды другого экрана")
	}
	model, _ = updateApp(model, keypress('?'))
	if model.help != nil || !model.showTemplates {
		t.Fatal("? должен закрывать помощь без перехода к ученикам")
	}
}

func TestQuestionMarkIsTextInForms(t *testing.T) {
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{}), app.NewTemplateService(&templateRepoStub{}), nil)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, keypress('?'))
	if model.help != nil || model.students.form.name.Value() != "?" {
		t.Fatal("? в форме ученика должен вводиться как символ")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, load := updateApp(model, keypress('p'))
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, keypress('?'))
	if model.help != nil || model.templates.form.name.Value() != "?" {
		t.Fatal("? в форме шаблона должен вводиться как символ")
	}
}

func TestHelpScrollAndQuit(t *testing.T) {
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{}), nil, nil)
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 80, Height: 8})
	model, _ = updateApp(model, keypress('?'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if model.help.offset != 2 || !strings.Contains(model.View().Content, "Редактировать имя") {
		t.Fatal("помощь должна прокручиваться в небольшом терминале")
	}
	model, _ = updateApp(model, tea.WindowSizeMsg{Width: 80, Height: 24})
	if model.help.offset != 0 {
		t.Fatal("после увеличения терминала смещение должно ограничиваться размером содержимого")
	}
	_, quit := updateApp(model, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if quit == nil {
		t.Fatal("Ctrl+C должен работать в помощи")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("ожидалась команда выхода")
	}
}
