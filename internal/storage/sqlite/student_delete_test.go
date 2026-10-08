package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestDeleteStudentPersistsAndUsesID(t *testing.T) {
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
	service := app.NewStudentService(store)
	// Одинаковые имена: удаление должно выбирать запись исключительно по ID.
	first, err := service.Create(ctx, "Иван'); DROP TABLE students; --")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, first.Name)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, first.ID); !errors.Is(err, domain.ErrStudentNotFound) {
		t.Fatalf("повторное удаление должно вернуть not found: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	students, err := store.ListStudents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(students) != 1 || !reflect.DeepEqual(students[0], second) {
		t.Fatalf("удалена не та запись или изменились её данные: %+v", students)
	}
	if err := app.NewStudentService(store).Delete(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	students, err = store.ListStudents(ctx)
	if err != nil || len(students) != 0 {
		t.Fatalf("последний ученик не удалён: students=%v, err=%v", students, err)
	}
}

func TestDeleteStudentRestrictLeavesDataIntact(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	service := app.NewStudentService(store)
	student, err := service.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	// Тестовая зависимая запись проверяет, что репозиторий не обходит FK-ограничения.
	if _, err := store.db.ExecContext(ctx, `CREATE TABLE dependent_record (
		student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE RESTRICT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO dependent_record (student_id) VALUES (?)`, student.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, student.ID); err == nil {
		t.Fatal("ожидалась ошибка внешнего ключа")
	}
	students, err := service.List(ctx)
	if err != nil || len(students) != 1 || students[0].ID != student.ID {
		t.Fatalf("ученик потерян после ошибки удаления: students=%v, err=%v", students, err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dependent_record`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("зависимая запись потеряна: count=%d, err=%v", count, err)
	}
}
