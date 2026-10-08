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

func TestRenameTopicFormPreservesTreeAndSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   domain.TemplateTopicID
	}{
		{name: "parent", id: 1},
		{name: "subtopic", id: 3},
		{name: "fifth level", id: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}, topics: nestedTopics()}
			model := openTemplatesForTopics(t, repo)
			model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
			model, _ = updateApp(model, load())
			topics := model.templates.topics.selectTopic(tc.id)
			model.templates.topics = &topics
			cursor := topics.cursor
			before := slices.Clone(topics.topics)
			selected := topics.rows()[cursor].topic
			model, _ = updateApp(model, keypress('e'))
			form := model.templates.topics.form
			if form == nil || model.templates.topics.editTopicID != tc.id || form.name.Value() != selected.Name || !form.name.Focused() || !strings.Contains(model.View().Content, "Редактирование темы") {
				t.Fatal("e должен открыть форму с названием выбранной темы")
			}
			model, _ = updateApp(model, keypress('q'))
			model, _ = updateApp(model, keypress('?'))
			if model.help != nil || model.templates.topics.form.name.Value() != selected.Name+"q?" {
				t.Fatal("shortcuts перехватили ввод названия")
			}
			model.templates.topics.form.name.SetValue("  Новое название  ")
			model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
			if save == nil || repo.topicRenameCalls != 0 || !model.templates.topics.form.saving {
				t.Fatal("переименование должно выполняться через Cmd")
			}
			model, duplicate := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
			if duplicate != nil {
				t.Fatal("повторный Enter запустил дублирующий запрос")
			}
			model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
			if model.templates.topics.form == nil {
				t.Fatal("нельзя отменить выполняющееся сохранение")
			}
			model, _ = updateApp(model, save())
			topics = *model.templates.topics
			after := topics.rows()[topics.cursor].topic
			if topics.form != nil || topics.cursor != cursor || after.ID != tc.id || after.Name != "Новое название" || repo.topicCreateCalls != 0 || repo.topicRenameCalls != 1 {
				t.Fatal("переименование должно сохранить выбор и не создавать новую тему")
			}
			if topicParentID(after) != topicParentID(selected) || after.Position != selected.Position || !after.CreatedAt.Equal(selected.CreatedAt) {
				t.Fatal("изменились родитель, порядок или дата создания")
			}
			for i, topic := range before {
				if topic.ID == tc.id {
					before[i] = after
				}
			}
			if !reflect.DeepEqual(before, topics.topics) {
				t.Fatal("переименование затронуло другие темы или подтемы")
			}
			model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
			if !strings.Contains(model.View().Content, "› Новое название") {
				t.Fatal("заголовок открытой темы должен использовать новое название")
			}
		})
	}
}

func TestRenameTopicCancelErrorsAndStaleReply(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}, topics: nestedTopics()}
	model := openTemplatesForTopics(t, repo)
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('e'))
	model.templates.topics.form.name.SetValue("Другое название")
	model, cancel := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cancel != nil || model.templates.topics.form != nil || repo.topicRenameCalls != 0 || !strings.Contains(model.View().Content, "▾ Циклы") {
		t.Fatal("Esc должен отменять переименование без изменения дерева")
	}
	model, _ = updateApp(model, keypress('e'))
	model.templates.topics.form.name.SetValue("   ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if !errors.Is(model.templates.topics.form.err, domain.ErrTopicNameRequired) || repo.topicRenameCalls != 0 {
		t.Fatal("пустое название должно отклоняться без записи")
	}
	model.templates.topics.form.name.SetValue("Повторения")
	repo.topicRenameErr = errors.New("storage failed")
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.templates.topics.form.saving || model.templates.topics.form.name.Value() != "Повторения" || !strings.Contains(model.View().Content, "Не удалось сохранить тему") {
		t.Fatal("ошибка должна оставлять введённое название в форме")
	}
	old := model.templates.topics.screenID - 1
	model, _ = updateApp(model, routeToTopics(old, templateTopicRenamedMsg{topic: domain.TemplateTopic{ID: 1, Name: "Чужой ответ"}}))
	if model.templates.topics.form == nil || model.templates.topics.rows()[0].topic.Name != "Циклы" {
		t.Fatal("устаревший ответ закрыл форму или изменил дерево")
	}
	repo.topicRenameErr = nil
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.templates.topics.form != nil || model.templates.topics.rows()[0].topic.Name != "Повторения" || repo.topicRenameCalls != 2 {
		t.Fatal("повторное сохранение после ошибки не удалось")
	}
}
