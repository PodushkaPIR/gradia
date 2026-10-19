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

func TestTutorEditsLearningPlanNameAndDescriptionFromStudentCard(t *testing.T) {
	repo := &routeRepoStub{routes: []domain.Route{{ID: 10, StudentID: 1, Name: "Python"}, {ID: 11, StudentID: 1, Name: "Другой план"}}}
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}), nil, app.NewRouteService(repo))
	model, _ = updateApp(model, model.Init()())
	model, cmd := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, cmd())
	model, _ = updateApp(model, keypress('t'))
	if model.screenInputMode() != inputText || !strings.Contains(model.View().Content, "Название учебного плана") {
		t.Fatal("plan name editor did not open")
	}
	for range "Python" {
		model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	model, cmd = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, cmd())
	if model.screenInputMode() != inputText || !strings.Contains(model.View().Content, "введите название учебного плана") {
		t.Fatal("invalid name did not keep editor open")
	}
	for _, r := range "  Новый план  " {
		model, _ = updateApp(model, keypress(r))
	}
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if save == nil || model.screenInputMode() != inputBusy || repo.routes[0].Name != "Python" {
		t.Fatal("rename must wait for Cmd success")
	}
	model, duplicate := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("duplicate save")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.details == nil {
		t.Fatal("left card during save")
	}
	model, _ = updateApp(model, save())
	if !strings.Contains(model.View().Content, "> Новый план") || repo.routes[1].Name != "Другой план" {
		t.Fatal("name did not update or another plan changed")
	}
	model, _ = updateApp(model, keypress('e'))
	if model.screenInputMode() != inputText || !strings.Contains(model.View().Content, "Описание учебного плана") {
		t.Fatal("description editor did not open")
	}
	for _, r := range "# Цели\nПрактика q и p" {
		if r == '\n' {
			model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
		} else {
			model, _ = updateApp(model, keypress(r))
		}
	}
	repo.err = errors.New("write failed")
	model, save = updateApp(model, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model, _ = updateApp(model, save())
	if model.screenInputMode() != inputText || !strings.Contains(model.View().Content, "Не удалось сохранить описание") || repo.routes[0].Description != "" {
		t.Fatal("failed description save changed plan or lost editor")
	}
	repo.err = nil
	model, save = updateApp(model, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model, _ = updateApp(model, save())
	if repo.routes[0].Description != "# Цели\nПрактика q и p" || !strings.Contains(model.View().Content, "Практика q и p") || repo.routes[0].Name != "Новый план" {
		t.Fatal("description was lost or overwrote the name")
	}
	// Esc cancels either editor without leaving the student card or changing its plan.
	for _, key := range []rune{'t', 'e'} {
		model, _ = updateApp(model, keypress(key))
		model, _ = updateApp(model, keypress('!'))
		model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
		if model.students.details == nil || model.screenInputMode() != inputNavigation || repo.routes[0].Name != "Новый план" || repo.routes[0].Description != "# Цели\nПрактика q и p" {
			t.Fatal("cancel left card or saved changes")
		}
	}
}
