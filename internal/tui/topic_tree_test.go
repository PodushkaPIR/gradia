package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/domain"
)

func nestedTopics() []domain.TemplateTopic {
	parent := func(id domain.TemplateTopicID) *domain.TemplateTopicID { return &id }
	return []domain.TemplateTopic{
		{ID: 2, TemplateID: 1, Name: "Строки", Position: 1},
		{ID: 4, TemplateID: 1, Name: "Цикл while", ParentID: parent(1), Position: 1},
		{ID: 1, TemplateID: 1, Name: "Циклы", Position: 0},
		{ID: 3, TemplateID: 1, Name: "Цикл for", ParentID: parent(1), Position: 0},
		{ID: 5, TemplateID: 1, Name: "Вложенные циклы", ParentID: parent(3), Position: 0},
		{ID: 6, TemplateID: 1, Name: "Условия", ParentID: parent(5), Position: 0},
		{ID: 7, TemplateID: 1, Name: "Правила", ParentID: parent(6), Position: 0},
	}
}

func TestTopicTreeNavigationAndCreation(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}, topics: nestedTopics()}
	model := openTemplatesForTopics(t, repo)
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	var ids []domain.TemplateTopicID
	for _, row := range model.templates.topics.rows() {
		ids = append(ids, row.topic.ID)
	}
	if !reflect.DeepEqual(ids, []domain.TemplateTopicID{1, 3, 5, 6, 7, 4, 2}) {
		t.Fatalf("неверный порядок дерева: %v", ids)
	}
	if !strings.Contains(model.View().Content, "    ▾ Цикл for") || !strings.Contains(model.View().Content, "      ▾ Вложенные циклы") {
		t.Fatal("в общем обзоре должна отображаться вложенность")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.templates.topics.currentParentID() != 1 || !strings.Contains(model.View().Content, "Python › Циклы") || strings.Contains(model.View().Content, "Строки") {
		t.Fatal("Enter должен открывать выбранную тему и её содержимое")
	}
	// Выбираем вложенную тему из обзора: Back идёт к её настоящему родителю.
	model, _ = updateApp(model, keypress('j'))
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.templates.topics.currentParentID() != 5 || len(model.templates.topics.path) != 3 {
		t.Fatal("не построен полный путь к выбранной подтеме")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics.currentParentID() != 3 {
		t.Fatal("Esc должен возвращать на один уровень выше")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics.currentParentID() != 1 || model.templates.topics.cursor != 1 {
		t.Fatal("Back должен сохранить выбор на прежнем уровне")
	}
	model, _ = updateApp(model, keypress('n'))
	if model.templates.topics.editTopicID != 0 || model.templates.topics.currentParentID() != 1 {
		t.Fatal("n должен создавать тему внутри текущей темы, а не выбранного потомка")
	}
	model.templates.topics.form.name.SetValue("Новый цикл")
	model, save := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, save())
	topics := model.templates.topics
	selected := topics.rows()[topics.cursor].topic
	if selected.Name != "Новый цикл" || selected.ParentID == nil || *selected.ParentID != 1 || selected.Position != 2 {
		t.Fatalf("подтема создана не на текущем уровне: %+v", selected)
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics.currentParentID() != 0 || model.templates.topics.cursor != 0 || !strings.Contains(model.View().Content, "    • Новый цикл") {
		t.Fatal("Esc должен вернуть в общий обзор с сохранённым выбором и новой подтемой")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics != nil || !model.showTemplates {
		t.Fatal("Esc из корня тем должен вернуть к шаблонам")
	}
}

func TestTopicTreeFifthLevel(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}, topics: nestedTopics()}
	model := openTemplatesForTopics(t, repo)
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	topics := model.templates.topics.selectTopic(7)
	model.templates.topics = &topics
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(model.templates.topics.path) != domain.TopicMaxDepth || !strings.Contains(model.View().Content, "5 уровней") {
		t.Fatal("на пятом уровне должно отображаться ограничение")
	}
	model, cmd := updateApp(model, keypress('n'))
	if cmd != nil || model.templates.topics.form != nil || repo.topicCreateCalls != 0 {
		t.Fatal("на пятом уровне нельзя создавать подтемы")
	}
	model, _ = updateApp(model, keypress('?'))
	if strings.Contains(model.View().Content, "Создать тему") {
		t.Fatal("помощь не должна предлагать создание шестого уровня")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = updateApp(model, keypress('n'))
	if model.templates.topics.form == nil || model.templates.topics.editTopicID != 0 || model.templates.topics.currentParentID() != 6 {
		t.Fatal("на четвёртом уровне создание должно быть доступно")
	}
}

func TestBreadcrumbUsesCurrentTopicData(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}, topics: nestedTopics()}
	model := openTemplatesForTopics(t, repo)
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	topics := model.templates.topics.selectTopic(5)
	model.templates.topics = &topics
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	wantPath := []domain.TemplateTopicID{1, 3, 5}
	if !reflect.DeepEqual(model.templates.topics.path, wantPath) {
		t.Fatalf("путь должен хранить ID: %v", model.templates.topics.path)
	}
	repo.topics = slices.Clone(repo.topics)
	for i := range repo.topics {
		switch repo.topics[i].ID {
		case 1:
			repo.topics[i].Name = "Повторения"
		case 3:
			repo.topics[i].Name = "Перебор"
		}
	}
	model, reload := updateApp(model, keypress('r'))
	model, _ = updateApp(model, reload())
	if !reflect.DeepEqual(model.templates.topics.path, wantPath) || !strings.Contains(model.View().Content, "Python › Повторения › Перебор › Вложенные циклы") {
		t.Fatal("после загрузки заголовок должен использовать актуальные названия без изменения пути")
	}
	selected := model.templates.topics.rows()[model.templates.topics.cursor].topic
	if selected.ID != 6 {
		t.Fatal("обновление названий не должно менять выбранную подтему")
	}
	parent, _ := model.templates.topics.topicByID(1)
	parent.Name = "Циклические конструкции"
	model, _ = updateApp(model, routeToTopics(model.templates.topics.screenID, templateTopicRenamedMsg{topic: parent}))
	if !reflect.DeepEqual(model.templates.topics.path, wantPath) || !strings.Contains(model.View().Content, "Python › Циклические конструкции › Перебор › Вложенные циклы") {
		t.Fatal("переименование должно обновлять заголовок через данные тем, не меняя путь")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics.currentParentID() != 3 || !strings.Contains(model.View().Content, "Python › Циклические конструкции › Перебор") {
		t.Fatal("Back должен сохранять актуальные названия предков")
	}
}

func TestReloadDropsMissingPathSuffix(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}, topics: nestedTopics()}
	model := openTemplatesForTopics(t, repo)
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	topics := model.templates.topics.selectTopic(5)
	model.templates.topics = &topics
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	// Моделируем загрузку дерева, где текущая ветка уже отсутствует.
	repo.topics = slices.DeleteFunc(slices.Clone(repo.topics), func(topic domain.TemplateTopic) bool { return topic.ID >= 5 })
	model, reload := updateApp(model, keypress('r'))
	model, _ = updateApp(model, reload())
	if !reflect.DeepEqual(model.templates.topics.path, []domain.TemplateTopicID{1, 3}) || model.templates.topics.cursor != 0 || strings.Contains(model.View().Content, "Вложенные циклы") {
		t.Fatal("обновление должно возвращать к ближайшему существующему предку")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.templates.topics.currentParentID() != 1 {
		t.Fatal("после восстановления пути должна работать навигация назад")
	}
}
