package sqlite

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestHomeworkOrderPersists(t *testing.T) {
	ctx := context.Background()
	f := newBankBatchFixture(t)
	if _, err := f.homework.AddManyFromBank(ctx, f.student.ID, f.assignment.ID,
		[]domain.TaskBankTaskID{f.sources[0].ID, f.sources[1].ID},
	); err != nil {
		t.Fatal(err)
	}
	before, err := f.homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil {
		t.Fatal(err)
	}
	files, err := f.homework.Attachments(ctx, f.student.ID, f.assignment.ID, before[2].ID)
	if err != nil || len(files) != 1 {
		t.Fatalf("missing fixture attachment: %+v %v", files, err)
	}
	positions, err := f.homework.SwapTasks(ctx, f.student.ID, f.assignment.ID, before[0].ID, before[2].ID)
	if err != nil || len(positions) != 2 {
		t.Fatalf("swap failed: %+v %v", positions, err)
	}
	after, err := f.homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil || len(after) != 3 {
		t.Fatalf("tasks lost: %+v %v", after, err)
	}
	for i, originalIndex := range []int{2, 1, 0} {
		want := before[originalIndex]
		want.Position, want.UpdatedAt = i, after[i].UpdatedAt
		if !reflect.DeepEqual(after[i], want) {
			t.Fatalf("swap changed task data: %+v; want %+v", after[i], want)
		}
	}
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	homework := app.NewHomeworkService(reopened)
	persisted, err := homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil || !reflect.DeepEqual(persisted, after) {
		t.Fatalf("order did not persist: %+v %v", persisted, err)
	}
	preserved, err := homework.Attachments(ctx, f.student.ID, f.assignment.ID, before[2].ID)
	if err != nil || !reflect.DeepEqual(preserved, files) {
		t.Fatalf("swap changed attachment metadata: %+v %v", preserved, err)
	}
	_, data, err := reopened.GetHomeworkAttachment(ctx, f.student.ID, f.assignment.ID, before[2].ID, files[0].ID)
	if err != nil || !bytes.Equal(data, f.originals[1]) {
		t.Fatalf("swap changed original image: %v", err)
	}
	items, err := homework.List(ctx, f.student.ID)
	if err != nil || len(items) != 1 || items[0].CompletedTasks != 1 || items[0].TotalTasks != 3 {
		t.Fatalf("swap changed completion summary: %+v %v", items, err)
	}
	appended, err := homework.AddFromBank(ctx, f.student.ID, f.assignment.ID, f.sources[0].ID)
	if err != nil || appended.Position != 3 {
		t.Fatalf("append ignored reordered tasks: %+v %v", appended, err)
	}
}

func TestHomeworkOrderRollback(t *testing.T) {
	ctx := context.Background()
	f := newBankBatchFixture(t)
	if _, err := f.homework.AddManyFromBank(ctx, f.student.ID, f.assignment.ID,
		[]domain.TaskBankTaskID{f.sources[0].ID, f.sources[1].ID},
	); err != nil {
		t.Fatal(err)
	}
	before, err := f.homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherStudent, err := app.NewStudentService(f.store).Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	otherHomework, err := f.homework.Create(ctx, f.student.ID, "Другое ДЗ")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := f.homework.AddTask(ctx, f.student.ID, otherHomework.ID, domain.HomeworkTaskInput{
		Content: "Другая задача", Topics: []string{"Другая тема"}, Difficulty: domain.DifficultyMedium,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		student       domain.StudentID
		homework      domain.HomeworkID
		first, second domain.HomeworkTaskID
		want          error
	}{
		{0, f.assignment.ID, before[0].ID, before[2].ID, domain.ErrStudentNotFound},
		{f.student.ID, 0, before[0].ID, before[2].ID, domain.ErrHomeworkNotFound},
		{otherStudent.ID, f.assignment.ID, before[0].ID, before[2].ID, domain.ErrHomeworkNotFound},
		{f.student.ID, f.assignment.ID, 0, before[2].ID, domain.ErrHomeworkTaskNotFound},
		{f.student.ID, f.assignment.ID, before[0].ID, -1, domain.ErrHomeworkTaskNotFound},
		{f.student.ID, f.assignment.ID, before[0].ID, foreign.ID, domain.ErrHomeworkTaskNotFound},
		{f.student.ID, f.assignment.ID, foreign.ID, before[2].ID, domain.ErrHomeworkTaskNotFound},
		{f.student.ID, f.assignment.ID, 999, before[2].ID, domain.ErrHomeworkTaskNotFound},
		{f.student.ID, 999, before[0].ID, before[2].ID, domain.ErrHomeworkNotFound},
	} {
		if _, err := f.homework.SwapTasks(ctx, test.student, test.homework, test.first, test.second); !errors.Is(err, test.want) {
			t.Fatalf("wrong scoped error: %v; want %v", err, test.want)
		}
	}
	if _, err := f.store.db.ExecContext(ctx, `
		CREATE TRIGGER fail_reorder BEFORE UPDATE ON homework_tasks
		WHEN NEW.position = 2 BEGIN SELECT RAISE(ABORT, 'late swap failure'); END
	`); err != nil {
		t.Fatal(err)
	}
	if positions, err := f.homework.SwapTasks(ctx, f.student.ID, f.assignment.ID, before[0].ID, before[2].ID); err == nil || positions != nil {
		t.Fatalf("failed swap reported success: %+v %v", positions, err)
	}
	after, err := f.homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("swap left partial writes: %+v %v", after, err)
	}
	if _, err := f.store.db.ExecContext(ctx, `DROP TRIGGER fail_reorder`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.homework.SwapTasks(ctx, f.student.ID, f.assignment.ID, before[0].ID, before[0].ID); err != nil {
		t.Fatal(err)
	}
	after, err = f.homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("self-swap changed data: %+v %v", after, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := f.homework.SwapTasks(canceled, f.student.ID, f.assignment.ID, before[0].ID, before[2].ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if _, err := f.homework.SwapTasks(ctx, f.student.ID, f.assignment.ID, before[0].ID, before[2].ID); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	otherTasks, err := f.homework.Tasks(ctx, f.student.ID, otherHomework.ID)
	if err != nil || len(otherTasks) != 1 || !reflect.DeepEqual(otherTasks[0], foreign) {
		t.Fatalf("swap changed another Homework: %+v %v", otherTasks, err)
	}
}
