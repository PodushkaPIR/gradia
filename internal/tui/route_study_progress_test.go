package tui

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestLearningPlanShowsMultipleCurrentTopicsWithoutGroupingByStatus(t *testing.T) {
	parent, hidden := domain.RouteTopicID(1), domain.RouteTopicID(7)
	topics := []domain.RouteTopic{
		{ID: 9, Name: "Будущая Б", Position: 3},
		{ID: 5, ParentID: &parent, Name: "Практика", Status: domain.ProgressNeedsPractice, Position: 3},
		{ID: 2, ParentID: &parent, Name: "Будущая А"},
		{ID: 1, Name: "Раздел", Status: domain.ProgressMastered, Revision: true},
		{ID: 3, ParentID: &parent, Name: "Освоенная", Status: domain.ProgressMastered, Revision: true, Position: 1},
		{ID: 4, ParentID: &parent, Name: "Изучаемая", Status: domain.ProgressLearning, Position: 2},
		{ID: 6, Name: "Исключённая текущая", Status: domain.ProgressLearning, Excluded: true, Position: 1},
		{ID: 7, Name: "Скрытая ветка", Excluded: true, Position: 2},
		{ID: 8, ParentID: &hidden, Name: "Скрытая предстоящая"},
	}
	before := slices.Clone(topics)
	m := routeViewModel{keys: newRouteViewKeyMap(), height: 30}
	m, _ = m.Update(routeTopicsLoadedMsg{topics: topics})
	view := m.View()
	for _, text := range []string{
		"Текущих: 2 · Предстоящих: 2",
		"Будущая А  — не начато · предстоящая",
		"Изучаемая  — изучается · текущая",
		"Практика  — нужна практика · текущая",
		"Освоенная  — освоено · повторить · завершённая",
		"К повторению: 1 тем",
	} {
		if !strings.Contains(view, text) {
			t.Fatalf("missing %q in:\n%s", text, view)
		}
	}
	previous := -1
	for _, name := range []string{"Будущая А", "Освоенная", "Изучаемая", "Практика", "Будущая Б"} {
		index := strings.Index(view, "• "+name)
		if index <= previous {
			t.Fatalf("topics regrouped by status or missing %q:\n%s", name, view)
		}
		previous = index
	}
	if strings.Contains(view, "Рекомендуется начать:") || strings.Contains(view, "Исключённая текущая") || strings.Contains(view, "Скрытая предстоящая") {
		t.Fatalf("suggestion or excluded topic leaked into active study:\n%s", view)
	}
	m, _ = m.Update(keypress('a'))
	view = m.View()
	if !strings.Contains(view, "Скрытая предстоящая [исключено с разделом]  — не начато") || strings.Contains(view, "Скрытая предстоящая [исключено с разделом]  — не начато · предстоящая") || strings.Contains(view, "Исключённая текущая [исключено]  — изучается · текущая") {
		t.Fatalf("shown excluded topics received active categories:\n%s", view)
	}
	if !reflect.DeepEqual(topics, before) {
		t.Fatal("display changed topic states")
	}
}

func TestSuggestedNextTopicFollowsOrderAndProgressWithoutWritingState(t *testing.T) {
	parent, hidden := domain.RouteTopicID(1), domain.RouteTopicID(6)
	repo := &orderRepoStub{progressRepoStub: progressRepoStub{topics: []domain.RouteTopic{
		{ID: 1, RouteID: 10, Name: "Раздел", Status: domain.ProgressLearning},
		{ID: 2, RouteID: 10, ParentID: &parent, Name: "Освоенная", Status: domain.ProgressMastered},
		{ID: 3, RouteID: 10, ParentID: &parent, Name: "Первая", Position: 1},
		{ID: 4, RouteID: 10, ParentID: &parent, Name: "Исключённая", Excluded: true, Position: 2},
		{ID: 5, RouteID: 10, ParentID: &parent, Name: "Вторая", Position: 3},
		{ID: 6, RouteID: 10, Name: "Исключённый раздел", Excluded: true, Position: 1},
		{ID: 7, RouteID: 10, ParentID: &hidden, Name: "Исключённая текущая", Status: domain.ProgressLearning},
	}}}
	before := slices.Clone(repo.topics)
	m := routeViewModel{ctx: context.Background(), service: app.NewRouteService(repo), route: domain.Route{ID: 10}, keys: newRouteViewKeyMap(), height: 30}
	m, _ = m.Update(routeTopicsLoadedMsg{topics: repo.topics})
	if !strings.Contains(m.View(), "Рекомендуется начать: Первая") || !strings.Contains(m.View(), "Текущих: 0 · Предстоящих: 2") {
		t.Fatalf("missing first active terminal suggestion:\n%s", m.View())
	}
	if repo.writes != 0 || !reflect.DeepEqual(repo.topics, before) {
		t.Fatal("suggestion changed stored state")
	}
	m, _ = m.Update(keypress('j'))
	m, _ = m.Update(keypress('j'))
	m, cmd := m.Update(keypress('J'))
	if cmd == nil || !strings.Contains(m.View(), "Рекомендуется начать: Первая") {
		t.Fatal("order changed before save completed")
	}
	m = deliverRouteViewCommand(m, cmd)
	if !strings.Contains(m.View(), "Рекомендуется начать: Вторая") || m.rows[m.cursor].topic.ID != 3 {
		t.Fatalf("suggestion ignored persisted order:\n%s", m.View())
	}
	m, cmd = m.Update(keypress(' '))
	m = deliverRouteViewCommand(m, cmd)
	if strings.Contains(m.View(), "Рекомендуется начать:") || !strings.Contains(m.View(), "Текущих: 1 · Предстоящих: 1") {
		t.Fatal("suggestion remained when a topic became current")
	}
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = deliverRouteViewCommand(m, cmd)
	if !strings.Contains(m.View(), "Рекомендуется начать: Вторая") {
		t.Fatal("suggestion did not return after current topic became not started")
	}
	m, _ = m.Update(keypress('k'))
	m, cmd = m.Update(keypress('x'))
	m = deliverRouteViewCommand(m, cmd)
	if !strings.Contains(m.View(), "Рекомендуется начать: Первая") {
		t.Fatalf("excluded suggested topic is still recommended:\n%s", m.View())
	}
	// Only the requested reorder, two status changes, and exclusion have persisted.
	if repo.writes != 4 {
		t.Fatalf("suggestion caused extra writes: %d", repo.writes)
	}
}

func TestNoSuggestedTopicWhenPlanIsEmptyCompletedOrExcluded(t *testing.T) {
	parent := domain.RouteTopicID(1)
	for _, test := range []struct {
		name   string
		topics []domain.RouteTopic
	}{
		{"empty", nil},
		{"completed with revision", []domain.RouteTopic{{ID: 1, Status: domain.ProgressMastered, Revision: true}}},
		{"excluded", []domain.RouteTopic{{ID: 1, Excluded: true}}},
		{"excluded branch", []domain.RouteTopic{{ID: 1, Excluded: true}, {ID: 2, ParentID: &parent}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := routeViewModel{keys: newRouteViewKeyMap(), height: 10}
			m, _ = m.Update(routeTopicsLoadedMsg{topics: test.topics})
			if strings.Contains(m.View(), "Рекомендуется начать:") {
				t.Fatalf("unexpected recommendation:\n%s", m.View())
			}
		})
	}
}
