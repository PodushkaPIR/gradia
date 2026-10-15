package app

import (
	"context"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type RouteRepository interface {
	ListStudentRoutes(context.Context, domain.StudentID) ([]domain.Route, error)
	CreateRouteFromTemplate(context.Context, domain.StudentID, domain.TemplateID, time.Time) (domain.Route, error)
	ListRouteTopics(context.Context, domain.RouteID) ([]domain.RouteTopic, error)
	SetRouteTopicStatus(context.Context, domain.RouteID, domain.RouteTopicID, domain.ProgressStatus, time.Time) error
	SetRouteTopicNote(context.Context, domain.RouteID, domain.RouteTopicID, string, time.Time) error
}

type RouteService struct{ repo RouteRepository }

func NewRouteService(repo RouteRepository) *RouteService { return &RouteService{repo: repo} }

func (s *RouteService) List(ctx context.Context, id domain.StudentID) ([]domain.Route, error) {
	if id <= 0 {
		return nil, domain.ErrStudentNotFound
	}
	return s.repo.ListStudentRoutes(ctx, id)
}

func (s *RouteService) Create(ctx context.Context, student domain.StudentID, template domain.TemplateID) (domain.Route, error) {
	if student <= 0 {
		return domain.Route{}, domain.ErrStudentNotFound
	}
	if template <= 0 {
		return domain.Route{}, domain.ErrTemplateNotFound
	}
	return s.repo.CreateRouteFromTemplate(ctx, student, template, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *RouteService) Topics(ctx context.Context, id domain.RouteID) ([]domain.RouteTopic, error) {
	if id <= 0 {
		return nil, domain.ErrRouteNotFound
	}
	return s.repo.ListRouteTopics(ctx, id)
}

func (s *RouteService) SetStatus(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, status domain.ProgressStatus) error {
	if routeID <= 0 {
		return domain.ErrRouteNotFound
	}
	if topicID <= 0 {
		return domain.ErrRouteTopicNotFound
	}
	if !status.Valid() {
		return domain.ErrInvalidProgressStatus
	}
	topics, err := s.repo.ListRouteTopics(ctx, routeID)
	if err != nil {
		return err
	}
	found := false
	for _, topic := range topics {
		if topic.ID == topicID {
			found = true
		}
		if topic.ParentID != nil && *topic.ParentID == topicID {
			return domain.ErrTopicHasSubtopics
		}
	}
	if !found {
		return domain.ErrRouteTopicNotFound
	}
	return s.repo.SetRouteTopicStatus(ctx, routeID, topicID, status, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *RouteService) SetNote(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, markdown string) error {
	if routeID <= 0 {
		return domain.ErrRouteNotFound
	}
	if topicID <= 0 {
		return domain.ErrRouteTopicNotFound
	}
	return s.repo.SetRouteTopicNote(ctx, routeID, topicID, markdown, time.Now().UTC().Truncate(time.Millisecond))
}
