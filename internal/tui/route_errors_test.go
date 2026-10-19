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

type routeRepoStub struct {
	routes []domain.Route
	err    error
	calls  int
}

func (r *routeRepoStub) SetRouteTopicExcluded(context.Context, domain.RouteID, domain.RouteTopicID, bool, time.Time) error {
	return r.err
}
func (r *routeRepoStub) ListRouteTopicExclusionHistory(context.Context, domain.RouteID, domain.RouteTopicID) ([]domain.RouteTopicExclusionChange, error) {
	return nil, r.err
}

func (r *routeRepoStub) SetRouteTopicRevision(context.Context, domain.RouteID, domain.RouteTopicID, bool, time.Time) error {
	return r.err
}
func (r *routeRepoStub) ListRouteTopicProgressHistory(context.Context, domain.RouteID, domain.RouteTopicID) ([]domain.RouteProgressChange, error) {
	return nil, r.err
}

func (r *routeRepoStub) ListStudentRoutes(context.Context, domain.StudentID) ([]domain.Route, error) {
	return r.routes, nil
}
func (r *routeRepoStub) ListRouteTopics(context.Context, domain.RouteID) ([]domain.RouteTopic, error) {
	return nil, r.err
}
func (r *routeRepoStub) SetRouteTopicStatus(context.Context, domain.RouteID, domain.RouteTopicID, domain.ProgressStatus, time.Time) error {
	return r.err
}

func (r *routeRepoStub) SetRouteTopicNote(context.Context, domain.RouteID, domain.RouteTopicID, string, time.Time) error {
	return r.err
}
func (r *routeRepoStub) CreateRouteFromTemplate(_ context.Context, student domain.StudentID, template domain.TemplateID, at time.Time) (domain.Route, error) {
	r.calls++
	if r.err != nil {
		return domain.Route{}, r.err
	}
	route := domain.Route{ID: 1, StudentID: student, SourceTemplateID: &template, Name: "Python", CreatedAt: at, UpdatedAt: at}
	r.routes = append(r.routes, route)
	return route, nil
}

func TestRouteCreationRetryAndScopedReplies(t *testing.T) {
	repo := &routeRepoStub{err: errors.New("write failed")}
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}), app.NewTemplateService(&templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}}), app.NewRouteService(repo))
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, load = updateApp(model, keypress('n'))
	model, _ = updateApp(model, load())
	model, create := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, create())
	if model.students.details.picker == nil || model.screenInputMode() != inputConfirmation || !strings.Contains(model.View().Content, "Не удалось") {
		t.Fatal("ошибка должна возвращать выбор шаблона для повторной попытки")
	}
	repo.err = nil
	model, create = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, load = updateApp(model, create())
	model, _ = updateApp(model, load())
	if repo.calls != 2 || len(model.students.details.routes) != 1 {
		t.Fatal("повторное создание не удалось")
	}
	model, oldLoad := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, newLoad := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, oldLoad())
	if !model.students.details.view.loading {
		t.Fatal("ответ закрытого просмотра обновил новый экземпляр")
	}
	model, _ = updateApp(model, newLoad())
	if model.students.details.view.loading {
		t.Fatal("ответ нового просмотра не доставлен")
	}
}
