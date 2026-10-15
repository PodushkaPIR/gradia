package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestRenameTemplatePreservesTopicsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if store != nil {
			store.Close()
		}
	})
	createdAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	template, err := store.CreateTemplate(ctx, domain.Template{Name: "Python", Description: "# Программа\n- Циклы\n- Строки", CreatedAt: createdAt, UpdatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewTemplateService(store)
	parent, err := service.AddTopic(ctx, template.ID, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddSubtopic(ctx, template.ID, parent.ID, "Цикл for"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddTopic(ctx, template.ID, "Строки"); err != nil {
		t.Fatal(err)
	}
	before, err := service.ListTopics(ctx, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	name := "Python', description_md = ''; DROP TABLE template_topics; --"
	renamed, err := service.Rename(ctx, template.ID, "  "+name+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != template.ID || renamed.Name != name || renamed.Description != template.Description || !renamed.CreatedAt.Equal(createdAt) || !renamed.UpdatedAt.After(createdAt) {
		t.Fatalf("переименование затронуло лишние поля: %+v", renamed)
	}
	if _, err := service.Rename(ctx, 999, "Не существует"); !errors.Is(err, domain.ErrTemplateNotFound) {
		t.Fatalf("ожидался not found: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	templates, err := store.ListTemplates(ctx)
	if err != nil || !reflect.DeepEqual(templates, []domain.Template{renamed}) {
		t.Fatalf("название не сохранено: %+v, %v", templates, err)
	}
	after, err := store.ListTemplateTopics(ctx, template.ID)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("темы, их порядок или иерархия изменились: %+v, %v", after, err)
	}
}
