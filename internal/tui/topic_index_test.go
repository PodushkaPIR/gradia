package tui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/shesh/gradia/internal/domain"
)

func TestTopicIndexFollowsDataChanges(t *testing.T) {
	model := templateTopicsModel{}
	model, _ = model.Update(templateTopicsLoadedMsg{topics: nestedTopics()})
	ids := func(model templateTopicsModel) []domain.TemplateTopicID {
		var result []domain.TemplateTopicID
		for _, row := range model.rows() {
			result = append(result, row.topic.ID)
		}
		return result
	}
	want := []domain.TemplateTopicID{1, 3, 5, 6, 7, 4, 2}
	if !reflect.DeepEqual(ids(model), want) {
		t.Fatalf("индекс не сохраняет порядок дерева: %v", ids(model))
	}
	before := model
	loaded := slices.Clone(model.topics)
	slices.Reverse(loaded)
	model, _ = model.Update(templateTopicsLoadedMsg{topics: loaded})
	if !reflect.DeepEqual(ids(model), want) {
		t.Fatal("порядок дерева не должен зависеть от порядка записей в результате загрузки")
	}
	parent := domain.TemplateTopicID(1)
	created := domain.TemplateTopic{ID: 8, TemplateID: 1, ParentID: &parent, Name: "Новый цикл", Position: 2}
	model, _ = model.Update(templateTopicCreatedMsg{topic: created})
	if topic, ok := model.topicByID(8); !ok || topic.Name != created.Name {
		t.Fatal("новая тема не появилась в индексе")
	}
	if !reflect.DeepEqual(ids(model), []domain.TemplateTopicID{1, 3, 5, 6, 7, 4, 8, 2}) {
		t.Fatal("новая тема добавлена не в свою ветку")
	}
	created.Name = "Переименованный цикл"
	model, _ = model.Update(templateTopicRenamedMsg{topic: created})
	if topic, ok := model.topicByID(8); !ok || topic.Name != created.Name {
		t.Fatal("индекс возвращает устаревшее название")
	}
	if _, ok := before.topicByID(8); ok {
		t.Fatal("обновление изменило индекс прежнего состояния модели")
	}
	model, _ = model.Update(templateTopicsLoadedMsg{topics: nestedTopics()})
	if _, ok := model.topicByID(8); ok || !reflect.DeepEqual(ids(model), want) {
		t.Fatal("повторная загрузка должна убирать отсутствующие записи из индекса")
	}
}
