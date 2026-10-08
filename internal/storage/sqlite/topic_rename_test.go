package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestRenameTopicPreservesHierarchyAcrossReopen(t *testing.T) {
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
	service := app.NewTemplateService(store)
	template, err := service.Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := domain.NewTemplateTopic(template.ID, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	draft.CreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	draft.UpdatedAt = draft.CreatedAt
	parent, err := store.CreateTemplateTopic(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.AddSubtopic(ctx, template.ID, parent.ID, "Цикл for")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddSubtopic(ctx, template.ID, child.ID, "Вложенные циклы"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddTopic(ctx, template.ID, "Строки"); err != nil {
		t.Fatal(err)
	}
	before, err := service.ListTopics(ctx, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	name := "Повторения', parent_id = NULL; DROP TABLE template_topics; --"
	renamedParent, err := service.RenameTopic(ctx, template.ID, parent.ID, "  "+name+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if renamedParent.Name != name || renamedParent.ID != parent.ID || renamedParent.Position != parent.Position || renamedParent.ParentID != nil || !renamedParent.CreatedAt.Equal(parent.CreatedAt) || !renamedParent.UpdatedAt.After(parent.UpdatedAt) {
		t.Fatalf("переименование затронуло лишние поля: %+v", renamedParent)
	}
	renamedChild, err := service.RenameTopic(ctx, template.ID, child.ID, "Перебор")
	if err != nil {
		t.Fatal(err)
	}
	if renamedChild.ParentID == nil || *renamedChild.ParentID != parent.ID || renamedChild.Position != child.Position || !renamedChild.CreatedAt.Equal(child.CreatedAt) {
		t.Fatalf("потерян родитель или порядок подтемы: %+v", renamedChild)
	}
	other, err := service.Create(ctx, "Математика")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RenameTopic(ctx, other.ID, parent.ID, "Чужая тема"); !errors.Is(err, domain.ErrTopicNotFound) {
		t.Fatalf("допущено переименование темы из чужого шаблона: %v", err)
	}
	if _, err := service.RenameTopic(ctx, template.ID, 999, "Не существует"); !errors.Is(err, domain.ErrTopicNotFound) {
		t.Fatalf("ожидался not found: %v", err)
	}
	expected := slices.Clone(before)
	for i, topic := range expected {
		switch topic.ID {
		case parent.ID:
			expected[i] = renamedParent
		case child.ID:
			expected[i] = renamedChild
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.ListTemplateTopics(ctx, template.ID)
	if err != nil || !reflect.DeepEqual(after, expected) {
		t.Fatalf("дерево или порядок изменились после переоткрытия: %+v, %v", after, err)
	}
}
