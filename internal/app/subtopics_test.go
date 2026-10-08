package app

import (
	"context"
	"errors"
	"testing"

	"github.com/shesh/gradia/internal/domain"
)

func TestSubtopicDepthAndParent(t *testing.T) {
	var topics []domain.TemplateTopic
	for i := 1; i <= domain.TopicMaxDepth; i++ {
		topic := domain.TemplateTopic{ID: domain.TemplateTopicID(i), TemplateID: 10, Name: "Тема"}
		if i > 1 {
			parent := domain.TemplateTopicID(i - 1)
			topic.ParentID = &parent
		}
		topics = append(topics, topic)
	}
	for _, tc := range []struct {
		name   string
		parent domain.TemplateTopicID
		err    error
	}{
		{name: "fifth level allowed", parent: 4},
		{name: "sixth level rejected", parent: 5, err: domain.ErrTopicDepthExceeded},
		{name: "missing parent", parent: 99, err: domain.ErrTopicNotFound},
		{name: "invalid parent", parent: 0, err: domain.ErrTopicNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &templateRepositoryStub{topics: topics}
			topic, err := NewTemplateService(repo).AddSubtopic(context.Background(), 10, tc.parent, " Подтема ")
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v, want=%v", err, tc.err)
			}
			if tc.err != nil {
				if repo.topicCalls != 0 {
					t.Fatal("невалидная подтема передана в репозиторий")
				}
			} else if topic.ParentID == nil || *topic.ParentID != tc.parent || topic.Name != "Подтема" || repo.topicCalls != 1 {
				t.Fatalf("неверная подтема: %+v", topic)
			}
		})
	}
	repo := &templateRepositoryStub{topics: topics}
	if _, err := NewTemplateService(repo).AddSubtopic(context.Background(), 20, 1, "Подтема"); !errors.Is(err, domain.ErrTopicNotFound) {
		t.Fatalf("нельзя использовать родителя из другого шаблона: %v", err)
	}
}
