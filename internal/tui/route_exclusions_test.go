package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func (r *progressRepoStub) SetRouteTopicExcluded(_ context.Context, route domain.RouteID, id domain.RouteTopicID, excluded bool, at time.Time) error {
	r.writes++
	if r.writeErr != nil {
		return r.writeErr
	}
	for i := range r.topics {
		if r.topics[i].ID == id {
			r.topics[i].Excluded = excluded
			return nil
		}
	}
	return domain.ErrRouteTopicNotFound
}

func deliverRouteViewCommand(m routeViewModel, cmd tea.Cmd) routeViewModel {
	msg := cmd().(screenMsg).payload.(studentDetailsMsg).payload.(routeViewMsg).payload
	m, _ = m.Update(msg)
	return m
}

func TestRouteExclusionUIKeepsHiddenDataAndIndividualFlags(t *testing.T) {
	parent := domain.RouteTopicID(1)
	repo := &progressRepoStub{topics: []domain.RouteTopic{
		{ID: 1, RouteID: 10, Name: "Циклы"},
		{ID: 2, RouteID: 10, ParentID: &parent, Name: "for", Status: domain.ProgressMastered, Revision: true, Notes: "Заметка"},
		{ID: 3, RouteID: 10, ParentID: &parent, Name: "while", Excluded: true},
		{ID: 4, RouteID: 10, Name: "Функции", Status: domain.ProgressLearning},
	}}
	m := routeViewModel{ctx: context.Background(), service: app.NewRouteService(repo), route: domain.Route{ID: 10}, keys: newRouteViewKeyMap(), height: 10}
	m, _ = m.Update(routeTopicsLoadedMsg{topics: repo.topics})
	if len(m.rows) != 3 || m.summaries[0].Total() != 2 {
		t.Fatal("initial excluded topic counted")
	}
	m, cmd := m.Update(keypress('x'))
	if cmd == nil || repo.writes != 0 || m.InputMode() != inputBusy || len(m.rows) != 3 {
		t.Fatal("exclusion must save through Cmd")
	}
	for _, key := range []rune{'x', 'a'} {
		if _, duplicate := m.Update(keypress(key)); duplicate != nil {
			t.Fatal("duplicate save")
		}
	}
	if m.showExcluded {
		t.Fatal("visibility changed during save")
	}
	m = deliverRouteViewCommand(m, cmd)
	if len(m.rows) != 1 || m.rows[0].topic.ID != 4 || len(m.topics) != 4 || m.summaries[0].Total() != 1 || m.summaries[0].Mastered != 0 {
		t.Fatal("excluded branch still active or data lost")
	}
	if !strings.Contains(m.View(), "Ранее освоено среди исключённых: 1") || strings.Contains(m.View(), "К повторению:") {
		t.Fatal("historical mastery or active revision count wrong")
	}
	m, _ = m.Update(keypress('a'))
	if len(m.rows) != 4 || m.rows[m.cursor].topic.ID != 4 || !strings.Contains(m.View(), "for [исключено с разделом]") {
		t.Fatal("show excluded lost selection or inherited annotation")
	}
	m.cursor = 0
	m, cmd = m.Update(keypress('x'))
	m = deliverRouteViewCommand(m, cmd)
	if m.topics[0].Excluded || !m.topics[2].Excluded || m.summaries[0].Total() != 2 || m.summaries[0].Mastered != 1 {
		t.Fatal("restoring parent restored independent exclusion")
	}
	if !m.topics[1].Revision || m.topics[1].Notes != "Заметка" {
		t.Fatal("hidden learning data lost")
	}
	m, _ = m.Update(keypress('a'))
	if len(m.rows) != 3 || m.rows[m.cursor].topic.ID != 1 {
		t.Fatal("hide excluded lost parent selection")
	}
	// Обновление прогресса не должно пересчитать сводку по одним видимым строкам.
	m, _ = m.Update(routeProgressSavedMsg{topicID: 4, status: domain.ProgressMastered})
	if len(m.topics) != 4 || m.summaries[0].Mastered != 2 || !m.topics[2].Excluded {
		t.Fatal("progress update lost hidden topics")
	}
	repo.writeErr = errors.New("write failed")
	m, cmd = m.Update(keypress('x'))
	m = deliverRouteViewCommand(m, cmd)
	if m.saveErr == nil || m.InputMode() != inputNavigation || m.topics[0].Excluded || len(m.rows) != 3 {
		t.Fatal("failed exclusion changed visible state")
	}
}

func TestAllTopicsExcludedCanBeRestored(t *testing.T) {
	repo := &progressRepoStub{topics: []domain.RouteTopic{{ID: 1, RouteID: 10, Name: "Циклы"}}}
	m := routeViewModel{ctx: context.Background(), service: app.NewRouteService(repo), route: domain.Route{ID: 10}, keys: newRouteViewKeyMap(), height: 10}
	m, _ = m.Update(routeTopicsLoadedMsg{topics: repo.topics})
	m, cmd := m.Update(keypress('x'))
	m = deliverRouteViewCommand(m, cmd)
	if len(m.rows) != 0 || !strings.Contains(m.View(), "Все темы исключены. Нажмите a") {
		t.Fatal("missing recovery hint")
	}
	if _, cmd := m.Update(keypress('x')); cmd != nil {
		t.Fatal("empty selection should not save")
	}
	m, _ = m.Update(keypress('a'))
	if len(m.rows) != 1 {
		t.Fatal("excluded topic inaccessible")
	}
	m, cmd = m.Update(keypress('x'))
	m = deliverRouteViewCommand(m, cmd)
	m, _ = m.Update(keypress('a'))
	if len(m.rows) != 1 || m.rows[0].effectivelyExcluded || m.summaries[0].Total() != 1 {
		t.Fatal("restored topic missing")
	}
}
