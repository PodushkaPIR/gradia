package app

import (
	"context"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type TemplateRepository interface {
	ListTemplates(context.Context) ([]domain.Template, error)
	CreateTemplate(context.Context, domain.Template) (domain.Template, error)
	RenameTemplate(context.Context, domain.TemplateID, string, time.Time) (domain.Template, error)
	ListTemplateTopics(context.Context, domain.TemplateID) ([]domain.TemplateTopic, error)
	CreateTemplateTopic(context.Context, domain.TemplateTopic) (domain.TemplateTopic, error)
	RenameTemplateTopic(context.Context, domain.TemplateID, domain.TemplateTopicID, string, time.Time) (domain.TemplateTopic, error)
}

type TemplateService struct {
	repo TemplateRepository
}

func NewTemplateService(repo TemplateRepository) *TemplateService {
	return &TemplateService{repo: repo}
}

func (s *TemplateService) List(ctx context.Context) ([]domain.Template, error) {
	return s.repo.ListTemplates(ctx)
}

func (s *TemplateService) Create(ctx context.Context, name string) (domain.Template, error) {
	template, err := domain.NewTemplate(name)
	if err != nil {
		return domain.Template{}, err
	}
	return s.repo.CreateTemplate(ctx, template)
}

func (s *TemplateService) Rename(ctx context.Context, id domain.TemplateID, name string) (domain.Template, error) {
	if id <= 0 {
		return domain.Template{}, domain.ErrTemplateNotFound
	}
	name, err := domain.NormalizeTemplateName(name)
	if err != nil {
		return domain.Template{}, err
	}
	return s.repo.RenameTemplate(ctx, id, name, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *TemplateService) ListTopics(ctx context.Context, id domain.TemplateID) ([]domain.TemplateTopic, error) {
	if id <= 0 {
		return nil, domain.ErrTemplateNotFound
	}
	return s.repo.ListTemplateTopics(ctx, id)
}

func (s *TemplateService) AddTopic(ctx context.Context, id domain.TemplateID, name string) (domain.TemplateTopic, error) {
	topic, err := domain.NewTemplateTopic(id, name)
	if err != nil {
		return domain.TemplateTopic{}, err
	}
	return s.repo.CreateTemplateTopic(ctx, topic)
}

func (s *TemplateService) AddSubtopic(ctx context.Context, id domain.TemplateID, parentID domain.TemplateTopicID, name string) (domain.TemplateTopic, error) {
	topic, err := domain.NewTemplateTopic(id, name)
	if err != nil {
		return domain.TemplateTopic{}, err
	}
	if parentID <= 0 {
		return domain.TemplateTopic{}, domain.ErrTopicNotFound
	}
	topics, err := s.repo.ListTemplateTopics(ctx, id)
	if err != nil {
		return domain.TemplateTopic{}, err
	}
	path, err := domain.TemplateTopicPath(topics, parentID)
	if err != nil {
		return domain.TemplateTopic{}, err
	}
	if len(path) >= domain.TopicMaxDepth {
		return domain.TemplateTopic{}, domain.ErrTopicDepthExceeded
	}
	topic.ParentID = &parentID
	return s.repo.CreateTemplateTopic(ctx, topic)
}

func (s *TemplateService) RenameTopic(ctx context.Context, templateID domain.TemplateID, topicID domain.TemplateTopicID, name string) (domain.TemplateTopic, error) {
	if templateID <= 0 {
		return domain.TemplateTopic{}, domain.ErrTemplateNotFound
	}
	if topicID <= 0 {
		return domain.TemplateTopic{}, domain.ErrTopicNotFound
	}
	name, err := domain.NormalizeTopicName(name)
	if err != nil {
		return domain.TemplateTopic{}, err
	}
	return s.repo.RenameTemplateTopic(ctx, templateID, topicID, name, time.Now().UTC().Truncate(time.Millisecond))
}
