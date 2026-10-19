package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func typeIntoRouteTopicForm(model routeViewModel, value string) routeViewModel {
	for _, r := range value {
		model, _ = model.Update(keypress(r))
	}
	return model
}

func TestTutorCanAddNestedTopicAndRenameItWithoutChangingParentData(t *testing.T) {
	repo := &progressRepoStub{topics: []domain.RouteTopic{{
		ID: 1, RouteID: 10, Name: "Циклы", Status: domain.ProgressMastered,
		Revision: true, Notes: "Заметка преподавателя",
	}}}
	model := routeViewModel{
		ctx: context.Background(), service: app.NewRouteService(repo), route: domain.Route{ID: 10},
		keys: newRouteViewKeyMap(), height: 10,
	}
	model, _ = model.Update(routeTopicsLoadedMsg{topics: repo.topics})
	model, _ = model.Update(keypress('n'))
	if model.InputMode() != inputText {
		t.Fatal("n should open a form to add a topic at the selected level")
	}
	model = typeIntoRouteTopicForm(model, "Функции")
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = deliverRouteViewCommand(model, cmd)
	if len(model.topics) != 2 || model.topics[1].ParentID != nil || model.topics[1].Status != domain.ProgressNotStarted {
		t.Fatalf("root topic not added as a fresh terminal topic: %+v", model.topics)
	}
	model, _ = model.Update(keypress('k'))
	model, _ = model.Update(keypress('c'))
	if model.InputMode() != inputText {
		t.Fatal("c should open a form to add a child to the selected topic")
	}
	model = typeIntoRouteTopicForm(model, "Цикл for")
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || model.InputMode() != inputBusy || len(repo.topics) != 2 {
		t.Fatal("creating a child should be a Cmd and not optimistically change the topic tree")
	}
	model = deliverRouteViewCommand(model, cmd)
	if len(model.topics) != 3 || model.topics[2].ParentID == nil || *model.topics[2].ParentID != 1 {
		t.Fatalf("child topic not added: %+v", model.topics)
	}
	if model.topics[0].Status != domain.ProgressMastered || !model.topics[0].Revision || model.topics[0].Notes != "Заметка преподавателя" {
		t.Fatalf("parent learning data changed: %+v", model.topics[0])
	}
	if model.topics[2].Status != domain.ProgressNotStarted || model.summaries[0].Total() != 2 || model.summaries[0].Mastered != 0 {
		t.Fatalf("new child should start a fresh terminal progress item: %+v, %+v", model.topics[2], model.summaries[0])
	}

	model, _ = model.Update(keypress('n'))
	model = typeIntoRouteTopicForm(model, "Цикл while")
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = deliverRouteViewCommand(model, cmd)
	if len(model.topics) != 4 || model.topics[3].ParentID == nil || *model.topics[3].ParentID != 1 || model.topics[3].Position != 1 {
		t.Fatalf("sibling topic was not added at the current level: %+v", model.topics)
	}
	model, _ = model.Update(keypress('t'))
	for range []rune("Цикл while") {
		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	model = typeIntoRouteTopicForm(model, "Цикл с условием")
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = deliverRouteViewCommand(model, cmd)
	if model.topics[3].Name != "Цикл с условием" || model.topics[3].Status != domain.ProgressNotStarted {
		t.Fatalf("rename changed progress or failed to persist: %+v", model.topics[3])
	}
	if !strings.Contains(model.View(), "Цикл с условием") {
		t.Fatal("renamed topic is not visible")
	}
}
