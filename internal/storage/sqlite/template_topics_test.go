package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestTemplateTopicsUpgradeAndPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.ExecContext(ctx, studentsMigration+templatesMigration+`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY); INSERT INTO schema_migrations VALUES (1), (2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO templates (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, 10, "Python с нуля", 1000, 1000); err != nil {
		t.Fatal(err)
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
	other, err := service.Create(ctx, "Математика")
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.AddTopic(ctx, 10, "  Циклы'); DROP TABLE template_topics; --  ")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.AddTopic(ctx, 10, "Алгоритмы")
	if err != nil {
		t.Fatal(err)
	}
	separate, err := service.AddTopic(ctx, other.ID, "Дроби")
	if err != nil {
		t.Fatal(err)
	}
	if first.Position != 0 || second.Position != 1 || separate.Position != 0 || first.ID == 0 {
		t.Fatalf("неверные позиции: %+v, %+v, %+v", first, second, separate)
	}
	if _, err := service.AddTopic(ctx, 999, "Сирота"); err == nil {
		t.Fatal("нельзя добавить тему несуществующего шаблона")
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, 10); err == nil {
		t.Fatal("удаление шаблона не должно каскадно стирать темы")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	topics, err := store.ListTemplateTopics(ctx, 10)
	if err != nil || !reflect.DeepEqual(topics, []domain.TemplateTopic{first, second}) {
		t.Fatalf("порядок или данные изменились: topics=%+v, err=%v", topics, err)
	}
	topics, err = store.ListTemplateTopics(ctx, other.ID)
	if err != nil || !reflect.DeepEqual(topics, []domain.TemplateTopic{separate}) {
		t.Fatalf("темы другого шаблона потеряны: topics=%+v, err=%v", topics, err)
	}
}
