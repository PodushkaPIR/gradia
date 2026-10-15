package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type templateRepositoryStub struct {
	calls            int
	err              error
	topicCalls       int
	listedID         domain.TemplateID
	topics           []domain.TemplateTopic
	renamed          []domain.TemplateTopic
	renamedTemplates []domain.Template
}

func (r *templateRepositoryStub) ListTemplates(context.Context) ([]domain.Template, error) {
	return nil, r.err
}

func (r *templateRepositoryStub) CreateTemplate(_ context.Context, template domain.Template) (domain.Template, error) {
	r.calls++
	template.ID = 1
	return template, r.err
}

func (r *templateRepositoryStub) RenameTemplate(_ context.Context, id domain.TemplateID, name string, updatedAt time.Time) (domain.Template, error) {
	template := domain.Template{ID: id, Name: name, UpdatedAt: updatedAt}
	r.renamedTemplates = append(r.renamedTemplates, template)
	return template, r.err
}

func (r *templateRepositoryStub) ListTemplateTopics(_ context.Context, id domain.TemplateID) ([]domain.TemplateTopic, error) {
	r.listedID = id
	var topics []domain.TemplateTopic
	for _, topic := range r.topics {
		if topic.TemplateID == id {
			topics = append(topics, topic)
		}
	}
	return topics, r.err
}

func (r *templateRepositoryStub) CreateTemplateTopic(_ context.Context, topic domain.TemplateTopic) (domain.TemplateTopic, error) {
	r.topicCalls++
	topic.ID = 1
	return topic, r.err
}

func (r *templateRepositoryStub) RenameTemplateTopic(_ context.Context, templateID domain.TemplateID, topicID domain.TemplateTopicID, name string, updatedAt time.Time) (domain.TemplateTopic, error) {
	topic := domain.TemplateTopic{ID: topicID, TemplateID: templateID, Name: name, UpdatedAt: updatedAt}
	r.renamed = append(r.renamed, topic)
	return topic, r.err
}

func TestAddTemplateTopicValidation(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		id          domain.TemplateID
		err         error
	}{
		{name: "valid", input: "  Циклы  ", id: 5},
		{name: "empty", input: "   ", id: 5, err: domain.ErrTopicNameRequired},
		{name: "too long", input: strings.Repeat("Я", 201), id: 5, err: domain.ErrTopicNameTooLong},
		{name: "missing template", input: "Циклы", err: domain.ErrTemplateNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &templateRepositoryStub{}
			topic, err := NewTemplateService(repo).AddTopic(context.Background(), tc.id, tc.input)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v, want=%v", err, tc.err)
			}
			if tc.err != nil {
				if repo.topicCalls != 0 {
					t.Fatal("невалидная тема передана в репозиторий")
				}
				return
			}
			if topic.ID == 0 || topic.TemplateID != tc.id || topic.Name != "Циклы" || repo.topicCalls != 1 || repo.calls != 0 || topic.CreatedAt.IsZero() {
				t.Fatalf("topic=%+v, repo=%+v", topic, repo)
			}
		})
	}
	repo := &templateRepositoryStub{err: errors.New("storage failed")}
	service := NewTemplateService(repo)
	if _, err := service.AddTopic(context.Background(), 5, "Циклы"); !errors.Is(err, repo.err) {
		t.Fatalf("ошибка сохранения потеряна: %v", err)
	}
	if _, err := service.ListTopics(context.Background(), 5); !errors.Is(err, repo.err) || repo.listedID != 5 {
		t.Fatalf("ошибка загрузки или ID потеряны: %v", err)
	}
}

func TestCreateTemplateValidation(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		err               error
	}{
		{name: "trim", input: "  Python с нуля  ", want: "Python с нуля"},
		{name: "empty", input: " \t\n ", err: domain.ErrTemplateNameRequired},
		{name: "too long", input: strings.Repeat("Я", 201), err: domain.ErrTemplateNameTooLong},
		{name: "Unicode limit", input: strings.Repeat("Я", 200), want: strings.Repeat("Я", 200)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &templateRepositoryStub{}
			template, err := NewTemplateService(repo).Create(context.Background(), tc.input)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v, want=%v", err, tc.err)
			}
			if tc.err != nil {
				if repo.calls != 0 {
					t.Fatal("невалидный шаблон передан в репозиторий")
				}
				return
			}
			if template.Name != tc.want || template.ID == 0 || template.CreatedAt.IsZero() || !template.CreatedAt.Equal(template.UpdatedAt) || repo.calls != 1 {
				t.Fatalf("template=%+v, calls=%d", template, repo.calls)
			}
		})
	}
	want := errors.New("storage failed")
	_, err := NewTemplateService(&templateRepositoryStub{err: want}).Create(context.Background(), "Python")
	if !errors.Is(err, want) {
		t.Fatalf("ошибка репозитория потеряна: %v", err)
	}
}
