package tui

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/domain"
)

func TestRenameTemplateKeepsTopicsAndSelection(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Алгоритмы"}, {ID: 2, Name: "Математика"}}, topics: []domain.TemplateTopic{{ID: 1, TemplateID: 2, Name: "Дроби"}}}
	model := openTemplatesForTopics(t, repo)
	model, _ = updateApp(model, keypress('j'))
	before := slices.Clone(repo.topics)
	if !strings.Contains(model.contextHelp().View(), "Редактировать название шаблона") || strings.Contains(model.templates.ShortHelp(), "e —") {
		t.Fatal("редактирование должно отображаться в полной помощи, без дополнительной клавиши внизу")
	}
	model, _ = updateApp(model, keypress('e'))
	if model.templates.form == nil || model.templates.editTemplateID != 2 || model.templates.form.name.Value() != "Математика" || !model.templates.form.name.Focused() {
		t.Fatal("e должен открыть название выбранного шаблона")
	}
	model.templates.form.name.SetValue("  Аарон  ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if save == nil || repo.renameCalls != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("переименование должно выполняться через Cmd")
	}
	model, duplicate := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("повторный Enter запустил дублирующее сохранение")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.form == nil {
		t.Fatal("нельзя отменять выполняющееся сохранение")
	}
	model, reload := updateApp(model, save())
	model, _ = updateApp(model, reload())
	if model.templates.form != nil || model.templates.cursor != 0 || model.templates.templates[0].ID != 2 || !strings.Contains(model.View().Content, "> Аарон") || repo.createCalls != 0 || repo.renameCalls != 1 {
		t.Fatal("переименование должно сохранить ID и выбор при изменении порядка списка")
	}
	if !reflect.DeepEqual(repo.topics, before) {
		t.Fatal("переименование изменило темы шаблона")
	}
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Темы — Аарон") || !strings.Contains(model.View().Content, "Дроби") {
		t.Fatal("темы должны открываться с новым названием шаблона")
	}
}

func TestRenameTemplateCancelAndErrors(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}}
	model := openTemplatesForTopics(t, repo)
	model, _ = updateApp(model, keypress('e'))
	model.templates.form.name.SetValue("Отменённое имя")
	model, cmd := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil || model.templates.form != nil || model.templates.editTemplateID != 0 || repo.renameCalls != 0 || repo.templates[0].Name != "Python" {
		t.Fatal("отмена не должна менять название")
	}
	model, _ = updateApp(model, keypress('e'))
	model.templates.form.name.SetValue("  ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if !errors.Is(model.templates.form.err, domain.ErrTemplateNameRequired) || repo.renameCalls != 0 || model.templates.editTemplateID != 1 {
		t.Fatal("пустое название должно отклоняться без записи")
	}
	model.templates.form.name.SetValue("Python с нуля")
	repo.renameErr = errors.New("storage failed")
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.templates.form.name.Value() != "Python с нуля" || model.screenInputMode() != inputText || repo.templates[0].Name != "Python" {
		t.Fatal("ошибка должна сохранять ввод и прежнее название")
	}
	repo.renameErr = nil
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, reload := updateApp(model, save())
	model, _ = updateApp(model, reload())
	if model.templates.templates[0].Name != "Python с нуля" || repo.renameCalls != 2 || repo.createCalls != 0 {
		t.Fatal("повторная попытка должна переименовать существующий шаблон")
	}
}
