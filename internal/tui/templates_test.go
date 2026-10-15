package tui

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type templateRepoStub struct {
	templates        []domain.Template
	loadErr          error
	createErr        error
	loadCalls        int
	createCalls      int
	renameCalls      int
	renameErr        error
	topics           []domain.TemplateTopic
	topicsLoadErr    error
	topicCreateErr   error
	topicsLoadCalls  int
	topicCreateCalls int
	topicRenameCalls int
	topicRenameErr   error
}

func (r *templateRepoStub) ListTemplates(context.Context) ([]domain.Template, error) {
	r.loadCalls++
	return slices.Clone(r.templates), r.loadErr
}

func (r *templateRepoStub) CreateTemplate(_ context.Context, template domain.Template) (domain.Template, error) {
	r.createCalls++
	if r.createErr != nil {
		return domain.Template{}, r.createErr
	}
	template.ID = domain.TemplateID(len(r.templates) + 1)
	r.templates = append(r.templates, template)
	sort.Slice(r.templates, func(i, j int) bool { return r.templates[i].Name < r.templates[j].Name })
	return template, nil
}

func (r *templateRepoStub) RenameTemplate(_ context.Context, id domain.TemplateID, name string, updatedAt time.Time) (domain.Template, error) {
	r.renameCalls++
	if r.renameErr != nil {
		return domain.Template{}, r.renameErr
	}
	for i, template := range r.templates {
		if template.ID == id {
			template.Name, template.UpdatedAt = name, updatedAt
			r.templates = slices.Clone(r.templates)
			r.templates[i] = template
			sort.Slice(r.templates, func(i, j int) bool { return r.templates[i].Name < r.templates[j].Name })
			return template, nil
		}
	}
	return domain.Template{}, domain.ErrTemplateNotFound
}

func (r *templateRepoStub) ListTemplateTopics(_ context.Context, id domain.TemplateID) ([]domain.TemplateTopic, error) {
	r.topicsLoadCalls++
	var topics []domain.TemplateTopic
	for _, topic := range r.topics {
		if topic.TemplateID == id {
			topics = append(topics, topic)
		}
	}
	return topics, r.topicsLoadErr
}

func (r *templateRepoStub) CreateTemplateTopic(_ context.Context, topic domain.TemplateTopic) (domain.TemplateTopic, error) {
	r.topicCreateCalls++
	if r.topicCreateErr != nil {
		return domain.TemplateTopic{}, r.topicCreateErr
	}
	topic.ID = domain.TemplateTopicID(len(r.topics) + 1)
	for _, existing := range r.topics {
		if existing.TemplateID == topic.TemplateID && topicParentID(existing) == topicParentID(topic) {
			topic.Position++
		}
	}
	r.topics = append(r.topics, topic)
	return topic, nil
}

func (r *templateRepoStub) RenameTemplateTopic(_ context.Context, templateID domain.TemplateID, topicID domain.TemplateTopicID, name string, updatedAt time.Time) (domain.TemplateTopic, error) {
	r.topicRenameCalls++
	if r.topicRenameErr != nil {
		return domain.TemplateTopic{}, r.topicRenameErr
	}
	for i, topic := range r.topics {
		if topic.TemplateID == templateID && topic.ID == topicID {
			topic.Name, topic.UpdatedAt = name, updatedAt
			r.topics = slices.Clone(r.topics)
			r.topics[i] = topic
			return topic, nil
		}
	}
	return domain.TemplateTopic{}, domain.ErrTopicNotFound
}

func TestTemplatesCreateAndReturn(t *testing.T) {
	students := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Анна"}, {ID: 2, Name: "Иван"}}}
	templates := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Алгоритмы"}}}
	model := NewApp(context.Background(), app.NewStudentService(students), app.NewTemplateService(templates))
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('j'))
	model, load := updateApp(model, keypress('p'))
	if !model.showTemplates || load == nil || templates.loadCalls != 0 || !strings.Contains(model.View().Content, "Загрузка") {
		t.Fatal("p должен открыть экран и вернуть команду загрузки")
	}
	model, _ = updateApp(model, load())
	model, _ = updateApp(model, keypress('n'))
	if model.templates.form == nil || !model.templates.form.name.Focused() {
		t.Fatal("форма шаблона не открылась")
	}
	for _, key := range []rune{'q', 'p', 'n', 'r'} {
		model, _ = updateApp(model, keypress(key))
	}
	if model.templates.form.name.Value() != "qpnr" || !model.showTemplates || model.students.form != nil {
		t.Fatal("shortcuts перехватили ввод в форме шаблона")
	}
	model.templates.form.name.SetValue("  Python с нуля  ")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if save == nil || templates.createCalls != 0 {
		t.Fatal("сохранение должно выполняться через Cmd")
	}
	model, duplicate := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("запущено дублирующее сохранение")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.form == nil || !model.showTemplates {
		t.Fatal("нельзя отменять уже выполняющееся сохранение")
	}
	model, reload := updateApp(model, save())
	model, _ = updateApp(model, reload())
	if model.templates.form != nil || templates.createCalls != 1 || !strings.Contains(model.View().Content, "> Python с нуля") {
		t.Fatal("созданный шаблон не выбран в списке")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.showTemplates || model.students.cursor != 1 || !strings.Contains(model.View().Content, "> Иван") {
		t.Fatal("возврат должен сохранить выбор ученика")
	}
}

func TestTemplatesErrorsAndCancel(t *testing.T) {
	repo := &templateRepoStub{loadErr: errors.New("storage failed")}
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{}), app.NewTemplateService(repo))
	model, load := updateApp(model, keypress('p'))
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Не удалось загрузить шаблоны") {
		t.Fatal("ошибка загрузки не отображена")
	}
	repo.loadErr = nil
	model, load = updateApp(model, keypress('r'))
	model, _ = updateApp(model, load())
	if !strings.Contains(model.View().Content, "Шаблонов пока нет") {
		t.Fatal("пустой список не отображён")
	}
	model, _ = updateApp(model, keypress('n'))
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.templates.form == nil || !errors.Is(model.templates.form.err, domain.ErrTemplateNameRequired) || repo.createCalls != 0 {
		t.Fatal("пустое название должно отклоняться без записи")
	}
	model.templates.form.name.SetValue("Python")
	repo.createErr = errors.New("write failed")
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	if model.templates.form.saving || model.templates.form.name.Value() != "Python" || !strings.Contains(model.View().Content, "Не удалось сохранить шаблон") {
		t.Fatal("ошибка сохранения должна оставить введённое название")
	}
	repo.createErr = nil
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, load = updateApp(model, save())
	model, _ = updateApp(model, load())
	if len(repo.templates) != 1 || model.templates.form != nil {
		t.Fatal("повторное сохранение не удалось")
	}
	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, keypress('x'))
	model, cmd := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil || model.templates.form != nil || !model.showTemplates || len(repo.templates) != 1 {
		t.Fatal("Esc из формы должен вернуться к шаблонам без сохранения")
	}
}

func TestTemplatesLateLoad(t *testing.T) {
	repo := &templateRepoStub{}
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{}), app.NewTemplateService(repo))
	model, load := updateApp(model, keypress('p'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, duplicate := updateApp(model, keypress('p'))
	if duplicate != nil {
		t.Fatal("повторный вход запустил дублирующую загрузку")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = updateApp(model, load())
	if model.showTemplates || model.templates.loading || !model.students.loading {
		t.Fatal("поздний ответ должен обновить только состояние шаблонов")
	}
}
