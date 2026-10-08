package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestTopicHierarchyMigrationAndDepth(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v3.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.ExecContext(ctx, studentsMigration+templatesMigration+templateTopicsMigration+`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY); INSERT INTO schema_migrations VALUES (1), (2), (3)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO templates (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, 10, "Python", 1000, 1000); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"Циклы", "Алгоритмы"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO template_topics (id, template_id, name, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, i+1, 10, name, i, 1000, 1000); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
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
	initial, err := service.ListTopics(ctx, 10)
	if err != nil || len(initial) != 2 || initial[0].ID != 1 || initial[0].Name != "Циклы" || initial[1].Position != 1 || initial[0].ParentID != nil {
		t.Fatalf("миграция изменила существующие темы: %+v, %v", initial, err)
	}
	parent := initial[0]
	for level := 2; level <= domain.TopicMaxDepth; level++ {
		parent, err = service.AddSubtopic(ctx, 10, parent.ID, fmt.Sprintf("Уровень %d", level))
		if err != nil {
			t.Fatal(err)
		}
		if parent.Position != 0 || parent.ParentID == nil {
			t.Fatalf("неверная позиция подтемы: %+v", parent)
		}
	}
	if _, err := service.AddSubtopic(ctx, 10, parent.ID, "Шестой уровень"); !errors.Is(err, domain.ErrTopicDepthExceeded) {
		t.Fatalf("сервис не запретил шестой уровень: %v", err)
	}
	// Ограничение действует и при прямом обращении к репозиторию.
	draft, err := domain.NewTemplateTopic(10, "Шестой уровень")
	if err != nil {
		t.Fatal(err)
	}
	draft.ParentID = &parent.ID
	if _, err := store.CreateTemplateTopic(ctx, draft); !errors.Is(err, domain.ErrTopicDepthExceeded) {
		t.Fatalf("SQLite не запретил шестой уровень: %v", err)
	}
	child, err := service.AddSubtopic(ctx, 10, 1, "Другой цикл'); DROP TABLE template_topics; --")
	if err != nil || child.Position != 1 {
		t.Fatalf("порядок соседних подтем неверен: %+v, %v", child, err)
	}
	other, err := service.Create(ctx, "Математика")
	if err != nil {
		t.Fatal(err)
	}
	draft, err = domain.NewTemplateTopic(other.ID, "Чужой родитель")
	if err != nil {
		t.Fatal(err)
	}
	parentID := domain.TemplateTopicID(1)
	draft.ParentID = &parentID
	if _, err := store.CreateTemplateTopic(ctx, draft); !errors.Is(err, domain.ErrTopicNotFound) {
		t.Fatalf("SQLite допустил родителя из другого шаблона: %v", err)
	}
	before, err := service.ListTopics(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 7 {
		t.Fatalf("отклонённая тема всё же сохранена: %d", len(before))
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM template_topics WHERE id = ?`, 1); err == nil {
		t.Fatal("родитель не должен удаляться вместе с потомками")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.ListTemplateTopics(ctx, 10)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("иерархия изменилась после переоткрытия: %+v, %v", after, err)
	}
}
