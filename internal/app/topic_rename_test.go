package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shesh/gradia/internal/domain"
)

func TestRenameTopicValidation(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		templateID  domain.TemplateID
		topicID     domain.TemplateTopicID
		err         error
	}{
		{name: "valid", input: "  Циклы  ", templateID: 5, topicID: 7},
		{name: "empty", input: "   ", templateID: 5, topicID: 7, err: domain.ErrTopicNameRequired},
		{name: "too long", input: strings.Repeat("Я", 201), templateID: 5, topicID: 7, err: domain.ErrTopicNameTooLong},
		{name: "invalid template", input: "Циклы", topicID: 7, err: domain.ErrTemplateNotFound},
		{name: "invalid topic", input: "Циклы", templateID: 5, err: domain.ErrTopicNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &templateRepositoryStub{}
			topic, err := NewTemplateService(repo).RenameTopic(context.Background(), tc.templateID, tc.topicID, tc.input)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v, want=%v", err, tc.err)
			}
			if tc.err != nil {
				if len(repo.renamed) != 0 {
					t.Fatal("невалидные данные переданы в репозиторий")
				}
				return
			}
			if topic.Name != "Циклы" || topic.ID != tc.topicID || topic.TemplateID != tc.templateID || topic.UpdatedAt.IsZero() || len(repo.renamed) != 1 || repo.topicCalls != 0 {
				t.Fatalf("переименование превратилось в создание или потеряло данные: %+v", topic)
			}
		})
	}
	for _, want := range []error{domain.ErrTopicNotFound, errors.New("storage failed")} {
		_, err := NewTemplateService(&templateRepositoryStub{err: want}).RenameTopic(context.Background(), 5, 7, "Циклы")
		if !errors.Is(err, want) {
			t.Fatalf("ошибка репозитория потеряна: %v", err)
		}
	}
}
