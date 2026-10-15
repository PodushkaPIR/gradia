package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

func TestStudentRouteFlowWithSQLite(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	students, templates, routes := app.NewStudentService(store), app.NewTemplateService(store), app.NewRouteService(store)
	if _, err := students.Create(ctx, "Анна"); err != nil {
		t.Fatal(err)
	}
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	template, err := templates.Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := templates.AddTopic(ctx, template.ID, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := templates.AddSubtopic(ctx, template.ID, parent.ID, "Цикл for"); err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, students, templates, routes)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('j'))
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Ученик — Иван") || model.students.details.student.ID != student.ID {
		t.Fatal("открыта карточка не того ученика")
	}
	model, load = updateApp(model, keypress('n'))
	model, _ = updateApp(model, load())
	model, create := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if create == nil || model.screenInputMode() != inputBusy {
		t.Fatal("создание должно выполняться через Cmd")
	}
	model, duplicate := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("запущено дублирующее создание")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.details.picker == nil {
		t.Fatal("нельзя отменить выполняющееся создание")
	}
	model, load = updateApp(model, create())
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "> Python") || len(model.students.details.routes) != 1 {
		t.Fatal("маршрут не выбран после создания")
	}
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "▾ Циклы") || !strings.Contains(model.View().Content, "    • Цикл for") {
		t.Fatal("дерево копии не отображено")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, pickerLoad := updateApp(model, keypress('n'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = updateApp(model, pickerLoad())
	if model.students.details.picker != nil {
		t.Fatal("поздняя загрузка открыла отменённый выбор шаблона")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.details != nil || model.students.cursor != 1 {
		t.Fatal("Back должен возвращать к прежнему ученику")
	}
	list, err := routes.List(ctx, student.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("отмена создала лишний маршрут: %+v, %v", list, err)
	}
}

func TestClosedStudentCardIgnoresLoad(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	students, templates, routes := app.NewStudentService(store), app.NewTemplateService(store), app.NewRouteService(store)
	if _, err := students.Create(ctx, "Иван"); err != nil {
		t.Fatal(err)
	}
	model := NewApp(ctx, students, templates, routes)
	model, _ = updateApp(model, model.Init()())
	model, oldLoad := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, newLoad := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, oldLoad())
	if !model.students.details.loading {
		t.Fatal("старый ответ обновил новую карточку")
	}
	model, _ = updateApp(model, newLoad())
	if model.students.details.loading {
		t.Fatal("ответ новой карточки не доставлен")
	}
}
