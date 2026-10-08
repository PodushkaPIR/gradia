package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

var _ app.StudentRepository = (*Store)(nil)

func TestStudentsPersistAcrossReopen(t *testing.T) {
	ctx := context.Background()
	// В том числе проверяем, что символы URI не изменяют путь к файлу.
	path := filepath.Join(t.TempDir(), "данные #1.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if store != nil {
			store.Close()
		}
	})

	students, err := store.ListStudents(ctx)
	if err != nil || len(students) != 0 {
		t.Fatalf("новая база: students=%v, err=%v", students, err)
	}
	var foreignKeys int
	if err := store.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys=%d, err=%v", foreignKeys, err)
	}

	// Проверяем настоящий INSERT репозитория: SQL-подобный текст должен остаться данными.
	name := "Иван'); DROP TABLE students; --"
	notes := "# Заметка\n\n- Повторить `while`\n- O'Reilly"
	draft, err := domain.NewStudent(name)
	if err != nil {
		t.Fatal(err)
	}
	draft.Notes = notes
	now := draft.CreatedAt
	created, err := store.CreateStudent(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatal("не назначен ID ученика")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	students, err = app.NewStudentService(store).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(students) != 1 {
		t.Fatalf("получено %d учеников вместо 1", len(students))
	}
	student := students[0]
	if student.ID != created.ID || student.Name != name || student.Notes != notes || student.Grade != nil {
		t.Fatalf("данные изменились после сохранения: %+v", student)
	}
	if !student.CreatedAt.Equal(now) || !student.UpdatedAt.Equal(now) {
		t.Fatalf("время изменилось: %+v", student)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("миграция должна применяться однократно: count=%d, err=%v", count, err)
	}
}

func TestNewerSchemaIsRejected(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := store.db.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	unexpected, err := Open(ctx, path)
	if err == nil {
		unexpected.Close()
		t.Fatal("ожидался отказ открытия более новой схемы")
	}
}
