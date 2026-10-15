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

func openTemplatesForTopics(t *testing.T, repo *templateRepoStub) App {
	t.Helper()
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{}), app.NewTemplateService(repo), nil)
	model, load := updateApp(model, keypress('p'))
	model, _ = updateApp(model, load())
	return model
}

func TestTemplateTopicsCreateAndBack(t *testing.T) {
	repo := &templateRepoStub{
		templates: []domain.Template{{ID: 1, Name: "Математика"}, {ID: 2, Name: "Python"}},
		topics:    []domain.TemplateTopic{{ID: 1, TemplateID: 2, Name: "Циклы"}},
	}
	model := openTemplatesForTopics(t, repo)
	model, _ = updateApp(model, keypress('j'))
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.templates.topics == nil || load == nil || repo.topicsLoadCalls != 0 || model.templates.topics.template.ID != 2 {
		t.Fatal("Enter должен открыть темы выбранного шаблона через Cmd")
	}
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('?'))
	if !strings.Contains(model.View().Content, "Создать тему") {
		t.Fatal("помощь не соответствует экрану тем")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, keypress('q'))
	model, _ = updateApp(model, keypress('?'))
	if model.help != nil || model.templates.topics.form.name.Value() != "q?" {
		t.Fatal("q и ? должны вводиться в поле названия темы")
	}
	model.templates.topics.form.name.SetValue("  Алгоритмы  ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if save == nil || repo.topicCreateCalls != 0 {
		t.Fatal("сохранение должно выполняться через Cmd")
	}
	model, duplicate := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("запущено дублирующее сохранение")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics.form == nil {
		t.Fatal("нельзя отменить выполняющееся сохранение")
	}
	model, _ = updateApp(model, save())
	topics := model.templates.topics
	if topics.form != nil || len(topics.topics) != 2 || topics.topics[0].Name != "Циклы" || topics.topics[1].Name != "Алгоритмы" || topics.cursor != 1 || repo.createCalls != 0 {
		t.Fatal("новая тема должна добавиться в конец и стать выбранной")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !model.showTemplates || model.templates.topics != nil || model.templates.cursor != 1 {
		t.Fatal("Esc должен вернуть к шаблонам и сохранить выбор")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.showTemplates {
		t.Fatal("следующий Esc должен вернуть к ученикам")
	}
}

func TestTemplateTopicsErrorsAndCancel(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}, topicsLoadErr: errors.New("load failed")}
	model := openTemplatesForTopics(t, repo)
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Не удалось загрузить темы") {
		t.Fatal("ошибка загрузки не отображена")
	}
	repo.topicsLoadErr = nil
	model, load = updateApp(model, keypress('r'))
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, keypress('x'))
	model, cancel := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cancel != nil || model.templates.topics == nil || model.templates.topics.form != nil || repo.topicCreateCalls != 0 {
		t.Fatal("отмена формы не должна закрывать темы или сохранять запись")
	}
	model, _ = updateApp(model, keypress('n'))
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if !errors.Is(model.templates.topics.form.err, domain.ErrTopicNameRequired) || repo.topicCreateCalls != 0 {
		t.Fatal("пустое название должно отклоняться без записи")
	}
	model.templates.topics.form.name.SetValue("Циклы")
	repo.topicCreateErr = errors.New("write failed")
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.templates.topics.form.saving || model.templates.topics.form.name.Value() != "Циклы" || !strings.Contains(model.View().Content, "Не удалось сохранить тему") {
		t.Fatal("ошибка сохранения должна оставлять название в форме")
	}
	repo.topicCreateErr = nil
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.templates.topics.form != nil || len(repo.topics) != 1 || repo.topicCreateCalls != 2 {
		t.Fatal("повторное сохранение не удалось")
	}
}

func TestTemplateTopicsIgnorePreviousScreenReply(t *testing.T) {
	repo := &templateRepoStub{
		templates: []domain.Template{{ID: 1, Name: "Python"}, {ID: 2, Name: "Математика"}},
		topics:    []domain.TemplateTopic{{ID: 1, TemplateID: 1, Name: "Циклы"}, {ID: 2, TemplateID: 2, Name: "Дроби"}},
	}
	model := openTemplatesForTopics(t, repo)
	model, oldLoad := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = updateApp(model, keypress('j'))
	model, newLoad := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, oldLoad())
	if !model.templates.topics.loading || len(model.templates.topics.topics) != 0 {
		t.Fatal("поздний ответ старого экрана изменил новый экран")
	}
	model, _ = updateApp(model, newLoad())
	if model.templates.topics.loading || !strings.Contains(model.View().Content, "Дроби") || strings.Contains(model.View().Content, "Циклы") {
		t.Fatal("загружены темы не того шаблона")
	}
}
