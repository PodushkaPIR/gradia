package tui

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestKeyBindingDrivesActionAndHelp(t *testing.T) {
	for _, editing := range []bool{false, true} {
		name := "create"
		if editing {
			name = "edit"
		}
		t.Run(name, func(t *testing.T) {
			model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}), nil)
			model, _ = updateApp(model, model.Init()())
			binding, old := &model.students.keys.Create, 'n'
			if editing {
				binding, old = &model.students.keys.Edit, 'e'
			}
			binding.SetKeys("x")
			binding.SetHelp("x", binding.Help().Desc)
			help := model.contextHelp().View()
			if !strings.Contains(help, "x           "+binding.description) {
				t.Fatal("полная помощь должна брать клавишу из привязки действия")
			}
			if !editing && !strings.Contains(model.students.ShortHelp(), "x — создать") {
				t.Fatal("короткая подсказка должна брать клавишу из той же привязки")
			}
			model, cmd := updateApp(model, keypress(old))
			if cmd != nil || model.students.form != nil {
				t.Fatal("старая клавиша продолжает запускать действие")
			}
			model, _ = updateApp(model, keypress('x'))
			if model.students.form == nil || (model.students.editStudentID != 0) != editing {
				t.Fatal("новая клавиша не запустила нужное действие")
			}
			other := newStudentsKeyMap()
			if !key.Matches(keypress('n'), other.Create.Binding) || !key.Matches(keypress('e'), other.Edit.Binding) {
				t.Fatal("изменение keymap одного экрана затронуло другой экземпляр")
			}
		})
	}
}

func TestDisabledBindingIsHiddenAndInactive(t *testing.T) {
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}), nil)
	model, _ = updateApp(model, model.Init()())
	model.students.keys.Create.SetEnabled(false)
	model.students.keys.Edit.SetEnabled(false)
	if strings.Contains(model.students.ShortHelp(), "n —") || strings.Contains(model.contextHelp().View(), "Редактировать имя") || strings.Contains(model.contextHelp().View(), "Создать ученика") {
		t.Fatal("отключённые действия не должны отображаться в помощи")
	}
	for _, pressed := range []rune{'n', 'e'} {
		var cmd tea.Cmd
		model, cmd = updateApp(model, keypress(pressed))
		if cmd != nil || model.students.form != nil {
			t.Fatal("отключённое действие всё ещё выполняется")
		}
	}
}

func TestHelpUsesContextBindingsWithoutRepeatedMovement(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}}
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{}), app.NewTemplateService(repo))
	check := func() {
		t.Helper()
		text := model.contextHelp().View()
		if strings.Count(text, "Передвижение") != 1 || strings.Contains(text, "Ctrl+C") {
			t.Fatal("помощь должна показывать передвижение один раз и не дублировать выход")
		}
	}
	check()
	model, load := updateApp(model, keypress('p'))
	model, _ = updateApp(model, load())
	check()
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	check()
	if !strings.Contains(model.contextHelp().View(), "Редактировать название") || strings.Contains(model.contextHelp().View(), "Редактировать имя") {
		t.Fatal("помощь должна использовать привязки текущего экрана")
	}
}
