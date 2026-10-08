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

var _ app.TemplateRepository = (*Store)(nil)

func TestTemplatesUpgradeAndPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	// Создаём БД предыдущей версии: новая миграция должна сохранить ученика.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.ExecContext(ctx, studentsMigration); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY); INSERT INTO schema_migrations VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (name, notes_md, created_at, updated_at) VALUES (?, ?, ?, ?)`, "Иван", "# Существующая заметка", 1000, 1000); err != nil {
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
	templates, err := store.ListTemplates(ctx)
	if err != nil || len(templates) != 0 {
		t.Fatalf("templates=%v, err=%v", templates, err)
	}
	students, err := store.ListStudents(ctx)
	if err != nil || len(students) != 1 || students[0].Name != "Иван" || students[0].Notes != "# Существующая заметка" {
		t.Fatalf("данные старой версии потеряны: students=%v, err=%v", students, err)
	}
	name := "Python'); DROP TABLE templates; --"
	created, err := app.NewTemplateService(store).Create(ctx, "  "+name+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Name != name {
		t.Fatalf("некорректный шаблон: %+v", created)
	}
	draft, err := domain.NewTemplate("Алгоритмы")
	if err != nil {
		t.Fatal(err)
	}
	draft.Description = "# Описание\n\n- Поиск\n- `sort`\nO'Reilly"
	second, err := store.CreateTemplate(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	templates, err = store.ListTemplates(ctx)
	if err != nil || !reflect.DeepEqual(templates, []domain.Template{created, second}) {
		t.Fatalf("данные шаблонов изменились: templates=%+v, err=%v", templates, err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != len(schemaMigrations) {
		t.Fatalf("миграции должны примениться ровно по одному разу: count=%d, err=%v", count, err)
	}
}
