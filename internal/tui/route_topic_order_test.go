package tui

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type orderRepoStub struct{ progressRepoStub }

func (r *orderRepoStub) SwapRouteTopics(_ context.Context, _ domain.RouteID, first, second domain.RouteTopicID, _ time.Time) ([]domain.RouteTopicPosition, error) {
	r.writes++
	if r.writeErr != nil {
		return nil, r.writeErr
	}
	firstIndex, secondIndex := -1, -1
	for i, topic := range r.topics {
		if topic.ID == first {
			firstIndex = i
		}
		if topic.ID == second {
			secondIndex = i
		}
	}
	if firstIndex < 0 || secondIndex < 0 {
		return nil, domain.ErrRouteTopicNotFound
	}
	r.topics[firstIndex].Position, r.topics[secondIndex].Position = r.topics[secondIndex].Position, r.topics[firstIndex].Position
	return []domain.RouteTopicPosition{{TopicID: first, Position: r.topics[firstIndex].Position}, {TopicID: second, Position: r.topics[secondIndex].Position}}, nil
}

func TestTutorReordersSiblingBranchesAndKeepsSelection(t *testing.T) {
	parent := domain.RouteTopicID(1)
	repo := &orderRepoStub{progressRepoStub: progressRepoStub{topics: []domain.RouteTopic{
		{ID: 1, RouteID: 10, Name: "Раздел"},
		{ID: 2, RouteID: 10, ParentID: &parent, Name: "Первая", Status: domain.ProgressMastered, Revision: true, Notes: "Заметка"},
		{ID: 3, RouteID: 10, ParentID: &parent, Name: "Исключённая", Position: 1, Excluded: true},
		{ID: 4, RouteID: 10, ParentID: &parent, Name: "Вторая", Position: 2},
		{ID: 5, RouteID: 10, Name: "Другой раздел", Position: 1},
	}}}
	m := routeViewModel{ctx: context.Background(), service: app.NewRouteService(repo), route: domain.Route{ID: 10}, keys: newRouteViewKeyMap(), height: 10}
	m, _ = m.Update(routeTopicsLoadedMsg{topics: repo.topics})
	if _, cmd := m.Update(keypress('K')); cmd != nil {
		t.Fatal("first sibling cannot move up")
	}
	m, cmd := m.Update(keypress('J'))
	if cmd == nil || m.InputMode() != inputBusy || repo.writes != 0 || m.rows[0].topic.ID != 1 {
		t.Fatal("reorder should wait for Cmd success")
	}
	if _, duplicate := m.Update(keypress('J')); duplicate != nil {
		t.Fatal("duplicate reorder")
	}
	m = deliverRouteViewCommand(m, cmd)
	if m.rows[0].topic.ID != 5 || m.rows[1].topic.ID != 1 || m.rows[m.cursor].topic.ID != 1 {
		t.Fatal("branch did not move intact or selection lost")
	}
	if _, cmd := m.Update(keypress('J')); cmd != nil {
		t.Fatal("descendants must not be treated as sibling targets")
	}
	m, _ = m.Update(keypress('j'))
	m, cmd = m.Update(keypress('J'))
	m = deliverRouteViewCommand(m, cmd)
	if m.rows[2].topic.ID != 4 || m.rows[3].topic.ID != 2 || m.rows[m.cursor].topic.ID != 2 {
		t.Fatal("visible children did not swap")
	}
	if !m.rows[3].topic.Revision || m.rows[3].topic.Notes != "Заметка" || m.summaries[0].Mastered != 1 || repo.topics[2].Position != 1 {
		t.Fatal("reorder changed data or hidden sibling position")
	}
	m, _ = m.Update(keypress('a'))
	if m.rows[m.cursor].topic.ID != 2 {
		t.Fatal("show excluded lost selection")
	}
	m, cmd = m.Update(keypress('K'))
	m = deliverRouteViewCommand(m, cmd)
	if m.rows[3].topic.ID != 2 || m.rows[4].topic.ID != 3 || !m.rows[4].topic.Excluded {
		t.Fatal("shown excluded sibling could not be reordered")
	}
	before := m.rows
	repo.writeErr = errors.New("write failed")
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModAlt})
	m = deliverRouteViewCommand(m, cmd)
	if m.saveErr == nil || m.InputMode() != inputNavigation || !reflect.DeepEqual(m.rows, before) {
		t.Fatal("failed reorder changed displayed order")
	}
}
