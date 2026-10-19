package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type routeRepositoryStub struct {
	calls  int
	err    error
	topics []domain.RouteTopic
	writes int
}

func (r *routeRepositoryStub) SetRouteTopicExcluded(context.Context, domain.RouteID, domain.RouteTopicID, bool, time.Time) error {
	r.writes++
	return r.err
}

func TestRouteExclusionServiceValidatesIDsAndPropagatesErrors(t *testing.T) {
	ctx := context.Background()
	repo := &routeRepositoryStub{}
	service := NewRouteService(repo)
	if err := service.SetExcluded(ctx, 0, 1, true); !errors.Is(err, domain.ErrRouteNotFound) {
		t.Fatal(err)
	}
	if err := service.SetExcluded(ctx, 1, 0, true); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatal(err)
	}
	if _, err := service.ExclusionHistory(ctx, 0, 1); !errors.Is(err, domain.ErrRouteNotFound) {
		t.Fatal(err)
	}
	if _, err := service.ExclusionHistory(ctx, 1, 0); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatal(err)
	}
	if repo.writes != 0 {
		t.Fatal("invalid IDs reached repository")
	}
	repo.err = errors.New("storage unavailable")
	if err := service.SetExcluded(ctx, 1, 2, true); !errors.Is(err, repo.err) {
		t.Fatal("exclusion error lost")
	}
	if _, err := service.ExclusionHistory(ctx, 1, 2); !errors.Is(err, repo.err) {
		t.Fatal("history error lost")
	}
}
func (r *routeRepositoryStub) ListRouteTopicExclusionHistory(context.Context, domain.RouteID, domain.RouteTopicID) ([]domain.RouteTopicExclusionChange, error) {
	return nil, r.err
}

func (r *routeRepositoryStub) SetRouteTopicRevision(context.Context, domain.RouteID, domain.RouteTopicID, bool, time.Time) error {
	r.writes++
	return r.err
}
func (r *routeRepositoryStub) ListRouteTopicProgressHistory(context.Context, domain.RouteID, domain.RouteTopicID) ([]domain.RouteProgressChange, error) {
	return nil, r.err
}

func (r *routeRepositoryStub) ListStudentRoutes(context.Context, domain.StudentID) ([]domain.Route, error) {
	r.calls++
	return nil, r.err
}
func (r *routeRepositoryStub) SetRouteTopicStatus(context.Context, domain.RouteID, domain.RouteTopicID, domain.ProgressStatus, time.Time) error {
	r.writes++
	return r.err
}

func (r *routeRepositoryStub) SetRouteTopicNote(context.Context, domain.RouteID, domain.RouteTopicID, string, time.Time) error {
	r.writes++
	return r.err
}
func (r *routeRepositoryStub) ListRouteTopics(context.Context, domain.RouteID) ([]domain.RouteTopic, error) {
	r.calls++
	return r.topics, r.err
}
func (r *routeRepositoryStub) CreateRouteTopic(_ context.Context, topic domain.RouteTopic) (domain.RouteTopic, error) {
	r.writes++
	return topic, r.err
}
func (r *routeRepositoryStub) RenameRouteTopic(context.Context, domain.RouteID, domain.RouteTopicID, string, time.Time) error {
	r.writes++
	return r.err
}
func (r *routeRepositoryStub) CreateRouteFromTemplate(_ context.Context, student domain.StudentID, template domain.TemplateID, at time.Time) (domain.Route, error) {
	r.calls++
	return domain.Route{StudentID: student, SourceTemplateID: &template, CreatedAt: at, UpdatedAt: at}, r.err
}

func TestRouteServiceValidatesIDsAndPropagatesErrors(t *testing.T) {
	repo := &routeRepositoryStub{}
	service := NewRouteService(repo)
	if _, err := service.Create(context.Background(), 0, 1); !errors.Is(err, domain.ErrStudentNotFound) {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), 1, 0); !errors.Is(err, domain.ErrTemplateNotFound) {
		t.Fatal(err)
	}
	if _, err := service.List(context.Background(), 0); !errors.Is(err, domain.ErrStudentNotFound) {
		t.Fatal(err)
	}
	if _, err := service.Topics(context.Background(), 0); !errors.Is(err, domain.ErrRouteNotFound) {
		t.Fatal(err)
	}
	if err := service.SetNote(context.Background(), 0, 1, "текст"); !errors.Is(err, domain.ErrRouteNotFound) {
		t.Fatal(err)
	}
	if err := service.SetNote(context.Background(), 1, 0, "текст"); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatal(err)
	}
	if repo.calls != 0 || repo.writes != 0 {
		t.Fatal("невалидные ID переданы в хранилище")
	}
	want := errors.New("storage failed")
	repo.err = want
	if err := service.SetNote(context.Background(), 1, 2, "текст"); !errors.Is(err, want) {
		t.Fatal("потеряна ошибка записи заметки")
	}
	if _, err := service.Create(context.Background(), 1, 2); !errors.Is(err, want) {
		t.Fatal("потеряна ошибка хранилища")
	}
	repo.err = nil
	route, err := service.Create(context.Background(), 1, 2)
	if err != nil || route.StudentID != 1 || *route.SourceTemplateID != 2 || route.CreatedAt.IsZero() {
		t.Fatalf("route=%+v, err=%v", route, err)
	}
}
