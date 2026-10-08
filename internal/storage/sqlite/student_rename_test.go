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

func TestRenameStudentPersistsAndPreservesOtherFields(t *testing.T) {
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
	grade := "9"
	createdAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	student, err := store.CreateStudent(ctx, domain.Student{
		Name: "Иван", Grade: &grade, Notes: "# Заметка\n- Повторить циклы",
		CreatedAt: createdAt, UpdatedAt: createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := app.NewStudentService(store).Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	name := "Аарон', notes_md = 'changed'; DROP TABLE students; --"
	renamed, err := app.NewStudentService(store).Rename(ctx, student.ID, "  "+name+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != student.ID || renamed.Name != name || renamed.Grade == nil || *renamed.Grade != grade || renamed.Notes != student.Notes || !renamed.CreatedAt.Equal(createdAt) || !renamed.UpdatedAt.After(student.UpdatedAt) {
		t.Fatalf("обновление затронуло лишние поля: %+v", renamed)
	}
	_, err = app.NewStudentService(store).Rename(ctx, 999, "Не существует")
	if !errors.Is(err, domain.ErrStudentNotFound) {
		t.Fatalf("ожидалась ошибка отсутствующего ученика, получено %v", err)
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
	if len(students) != 2 || !reflect.DeepEqual(students[0], renamed) || !reflect.DeepEqual(students[1], other) {
		t.Fatalf("данные изменились после переоткрытия: %+v", students)
	}
}
