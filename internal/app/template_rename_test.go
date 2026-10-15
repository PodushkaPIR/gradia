package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shesh/gradia/internal/domain"
)

func TestRenameTemplateValidation(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		id          domain.TemplateID
		err         error
	}{
		{name: "valid", input: "  Python с нуля  ", id: 7},
		{name: "empty", input: "   ", id: 7, err: domain.ErrTemplateNameRequired},
		{name: "too long", input: strings.Repeat("Я", 201), id: 7, err: domain.ErrTemplateNameTooLong},
		{name: "invalid ID", input: "Python", err: domain.ErrTemplateNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &templateRepositoryStub{}
			template, err := NewTemplateService(repo).Rename(context.Background(), tc.id, tc.input)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v, want=%v", err, tc.err)
			}
			if tc.err != nil {
				if len(repo.renamedTemplates) != 0 {
					t.Fatal("невалидные данные переданы в репозиторий")
				}
				return
			}
			if template.ID != tc.id || template.Name != "Python с нуля" || template.UpdatedAt.IsZero() || len(repo.renamedTemplates) != 1 || repo.calls != 0 {
				t.Fatalf("переименование потеряло данные или создало новую запись: %+v", template)
			}
		})
	}
	for _, want := range []error{domain.ErrTemplateNotFound, errors.New("storage failed")} {
		_, err := NewTemplateService(&templateRepositoryStub{err: want}).Rename(context.Background(), 7, "Python")
		if !errors.Is(err, want) {
			t.Fatalf("ошибка репозитория потеряна: %v", err)
		}
	}
}
