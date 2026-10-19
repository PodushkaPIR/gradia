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
	CreateRouteTopic(context.Context, domain.RouteTopic) (domain.RouteTopic, error)
	RenameRouteTopic(context.Context, domain.RouteID, domain.RouteTopicID, string, time.Time) error
	SwapRouteTopics(context.Context, domain.RouteID, domain.RouteTopicID, domain.RouteTopicID, time.Time) ([]domain.RouteTopicPosition, error)
	SetRouteTopicStatus(context.Context, domain.RouteID, domain.RouteTopicID, domain.ProgressStatus, time.Time) error
	SetRouteTopicNote(context.Context, domain.RouteID, domain.RouteTopicID, string, time.Time) error
	SetRouteTopicRevision(context.Context, domain.RouteID, domain.RouteTopicID, bool, time.Time) error
	ListRouteTopicProgressHistory(context.Context, domain.RouteID, domain.RouteTopicID) ([]domain.RouteProgressChange, error)
	SetRouteTopicExcluded(context.Context, domain.RouteID, domain.RouteTopicID, bool, time.Time) error
	ListRouteTopicExclusionHistory(context.Context, domain.RouteID, domain.RouteTopicID) ([]domain.RouteTopicExclusionChange, error)
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

func (s *RouteService) AddTopic(ctx context.Context, routeID domain.RouteID, parentID domain.RouteTopicID, name string) (domain.RouteTopic, error) {
	if routeID <= 0 {
		return domain.RouteTopic{}, domain.ErrRouteNotFound
	}
	if parentID < 0 {
		return domain.RouteTopic{}, domain.ErrRouteTopicNotFound
	}
	topic, err := domain.NewRouteTopic(routeID, name)
	if err != nil {
		return domain.RouteTopic{}, err
	}
	if parentID > 0 {
		topics, err := s.repo.ListRouteTopics(ctx, routeID)
		if err != nil {
			return domain.RouteTopic{}, err
		}
		path, err := domain.RouteTopicPath(topics, parentID)
		if err != nil {
			return domain.RouteTopic{}, err
		}
		if len(path) >= domain.TopicMaxDepth {
			return domain.RouteTopic{}, domain.ErrTopicDepthExceeded
		}
		topic.ParentID = &parentID
	}
	return s.repo.CreateRouteTopic(ctx, topic)
}

func (s *RouteService) RenameTopic(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, name string) error {
	if routeID <= 0 {
		return domain.ErrRouteNotFound
	}
	if topicID <= 0 {
		return domain.ErrRouteTopicNotFound
	}
	name, err := domain.NormalizeTopicName(name)
	if err != nil {
		return err
	}
	return s.repo.RenameRouteTopic(ctx, routeID, topicID, name, time.Now().UTC().Truncate(time.Millisecond))
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

func (s *RouteService) SetRevision(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, revision bool) error {
	if routeID <= 0 {
		return domain.ErrRouteNotFound
	}
	if topicID <= 0 {
		return domain.ErrRouteTopicNotFound
	}
	return s.repo.SetRouteTopicRevision(ctx, routeID, topicID, revision, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *RouteService) ProgressHistory(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID) ([]domain.RouteProgressChange, error) {
	if routeID <= 0 {
		return nil, domain.ErrRouteNotFound
	}
	if topicID <= 0 {
		return nil, domain.ErrRouteTopicNotFound
	}
	return s.repo.ListRouteTopicProgressHistory(ctx, routeID, topicID)
}

func (s *RouteService) SetExcluded(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID, excluded bool) error {
	if routeID <= 0 {
		return domain.ErrRouteNotFound
	}
	if topicID <= 0 {
		return domain.ErrRouteTopicNotFound
	}
	return s.repo.SetRouteTopicExcluded(ctx, routeID, topicID, excluded, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *RouteService) ExclusionHistory(ctx context.Context, routeID domain.RouteID, topicID domain.RouteTopicID) ([]domain.RouteTopicExclusionChange, error) {
	if routeID <= 0 {
		return nil, domain.ErrRouteNotFound
	}
	if topicID <= 0 {
		return nil, domain.ErrRouteTopicNotFound
	}
	return s.repo.ListRouteTopicExclusionHistory(ctx, routeID, topicID)
}

func (s *RouteService) SwapTopics(ctx context.Context, routeID domain.RouteID, first, second domain.RouteTopicID) ([]domain.RouteTopicPosition, error) {
	if routeID <= 0 {
		return nil, domain.ErrRouteNotFound
	}
	if first <= 0 || second <= 0 {
		return nil, domain.ErrRouteTopicNotFound
	}
	return s.repo.SwapRouteTopics(ctx, routeID, first, second, time.Now().UTC().Truncate(time.Millisecond))
}
