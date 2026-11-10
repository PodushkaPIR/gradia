package sqlite

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/attachments"
	"github.com/shesh/gradia/internal/domain"
)

type bankBatchFixture struct {
	store      *Store
	path       string
	student    domain.Student
	homework   *app.HomeworkService
	assignment domain.Homework
	bank       *app.TaskBankService
	sources    []domain.TaskBankTask
	originals  [][]byte
}

func newBankBatchFixture(t *testing.T) bankBatchFixture {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	homework := app.NewHomeworkService(store)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	manual, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{
		Content: "Ручная задача", Topics: []string{"Повторение"}, Difficulty: domain.DifficultyEasy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := homework.SetCompleted(ctx, student.ID, assignment.ID, manual.ID, true); err != nil {
		t.Fatal(err)
	}
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	fixture := bankBatchFixture{
		store: store, path: path, student: student, homework: homework, assignment: assignment,
		bank:      app.NewTaskBankService(store).WithAttachments(attachments.NewLocalFiles(dir), nil),
		originals: [][]byte{[]byte("%PDF-1.4\nORIGINAL\n%%EOF\n"), picture.Bytes()},
	}
	for i, name := range []string{"first.pdf", "second.png"} {
		source, err := fixture.bank.Create(ctx, domain.TaskBankInput{
			Content: name, Subject: "Математика", Topics: []string{name, "Практика"},
			Difficulty: []domain.TaskDifficulty{domain.DifficultyEasy, domain.DifficultyHard}[i],
		})
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, fixture.originals[i], 0600); err != nil {
			t.Fatal(err)
		}
		pages := ""
		if i == 0 {
			pages = "2-3"
		}
		if _, err := fixture.bank.AttachFile(ctx, source.ID, file, pages); err != nil {
			t.Fatal(err)
		}
		fixture.sources = append(fixture.sources, source)
	}
	return fixture
}

func TestBankBatchOrderAndSnapshots(t *testing.T) {
	ctx := context.Background()
	f := newBankBatchFixture(t)
	ids := []domain.TaskBankTaskID{f.sources[1].ID, f.sources[0].ID, f.sources[1].ID}
	originalIDs := append([]domain.TaskBankTaskID(nil), ids...)
	copied, err := f.homework.AddManyFromBank(ctx, f.student.ID, f.assignment.ID, ids)
	if err != nil || len(copied) != 2 {
		t.Fatalf("batch failed: %+v %v", copied, err)
	}
	if !reflect.DeepEqual(ids, originalIDs) {
		t.Fatal("selection input was changed")
	}
	for i, sourceIndex := range []int{1, 0} {
		task, source := copied[i], f.sources[sourceIndex]
		if task.Position != i+1 || task.Completed || task.Content != source.Content ||
			task.Difficulty != source.Difficulty || !reflect.DeepEqual(task.Topics, source.Topics) {
			t.Fatalf("snapshot order or metadata changed: %+v", task)
		}
		files, err := f.homework.Attachments(ctx, f.student.ID, f.assignment.ID, task.ID)
		if err != nil || len(files) != 1 {
			t.Fatalf("missing snapshot file: %+v %v", files, err)
		}
		metadata, data, err := f.store.GetHomeworkAttachment(ctx, f.student.ID, f.assignment.ID, task.ID, files[0].ID)
		if err != nil || !bytes.Equal(data, f.originals[sourceIndex]) || metadata.Name != source.Content {
			t.Fatalf("original changed: %+v %v", metadata, err)
		}
		if sourceIndex == 0 && (metadata.MediaType != "application/pdf" || metadata.Pages != "2-3") {
			t.Fatalf("PDF metadata lost: %+v", metadata)
		}
		if sourceIndex == 1 && (metadata.MediaType != "image/png" || metadata.Pages != "") {
			t.Fatalf("image metadata lost: %+v", metadata)
		}
	}
	before, err := f.homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil || len(before) != 3 || !before[0].Completed {
		t.Fatalf("existing task changed: %+v %v", before, err)
	}
	if _, err := f.bank.Edit(ctx, f.sources[0].ID, domain.TaskBankInput{
		Content: "Изменено", Subject: "Математика", Topics: []string{"Другая тема"}, Difficulty: domain.DifficultyMedium,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.db.ExecContext(ctx, `DELETE FROM task_bank_attachments`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.db.ExecContext(ctx, `DELETE FROM task_bank_tasks`); err != nil {
		t.Fatal(err)
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
	after, err := homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("bank edit/deletion/restart changed snapshots: %+v %v", after, err)
	}
	files, err := homework.Attachments(ctx, f.student.ID, f.assignment.ID, copied[0].ID)
	if err != nil || len(files) != 1 {
		t.Fatalf("restart lost attachment: %+v %v", files, err)
	}
	_, data, err := reopened.GetHomeworkAttachment(ctx, f.student.ID, f.assignment.ID, copied[0].ID, files[0].ID)
	if err != nil || !bytes.Equal(data, f.originals[1]) {
		t.Fatalf("restart lost image: %v", err)
	}
}

func TestBankBatchRollbackAndRetry(t *testing.T) {
	ctx := context.Background()
	f := newBankBatchFixture(t)
	before, err := f.homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, err := app.NewStudentService(f.store).Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		student  domain.StudentID
		homework domain.HomeworkID
		sources  []domain.TaskBankTaskID
		want     error
	}{
		{0, f.assignment.ID, []domain.TaskBankTaskID{f.sources[0].ID}, domain.ErrStudentNotFound},
		{f.student.ID, 0, []domain.TaskBankTaskID{f.sources[0].ID}, domain.ErrHomeworkNotFound},
		{other.ID, f.assignment.ID, []domain.TaskBankTaskID{f.sources[0].ID}, domain.ErrHomeworkNotFound},
		{f.student.ID, f.assignment.ID, nil, app.ErrBankSelectionRequired},
		{f.student.ID, f.assignment.ID, []domain.TaskBankTaskID{f.sources[0].ID, 0}, domain.ErrTaskBankTaskNotFound},
		{f.student.ID, f.assignment.ID, []domain.TaskBankTaskID{f.sources[0].ID, 999}, domain.ErrTaskBankTaskNotFound},
	} {
		if copied, err := f.homework.AddManyFromBank(ctx, test.student, test.homework, test.sources); !errors.Is(err, test.want) || copied != nil {
			t.Fatalf("invalid batch: %+v %v; want %v", copied, err, test.want)
		}
	}
	if _, err := f.store.db.ExecContext(ctx, `
		CREATE TRIGGER fail_batch BEFORE INSERT ON homework_attachments
		WHEN NEW.name = 'second.png' BEGIN SELECT RAISE(ABORT, 'late batch failure'); END
	`); err != nil {
		t.Fatal(err)
	}
	ids := []domain.TaskBankTaskID{f.sources[0].ID, f.sources[1].ID}
	if copied, err := f.homework.AddManyFromBank(ctx, f.student.ID, f.assignment.ID, ids); err == nil || copied != nil {
		t.Fatalf("partial batch reported success: %+v %v", copied, err)
	}
	after, err := f.homework.Tasks(ctx, f.student.ID, f.assignment.ID)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("partial tasks left behind: %+v %v", after, err)
	}
	for table, want := range map[string]int{"homework_attachments": 0, "homework_task_topics": 1} {
		var count int
		if err := f.store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != want {
			t.Fatalf("partial rows in %s: %d %v", table, count, err)
		}
	}
	if _, err := f.store.db.ExecContext(ctx, `DROP TRIGGER fail_batch`); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := f.homework.AddManyFromBank(canceled, f.student.ID, f.assignment.ID, ids); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	copied, err := f.homework.AddManyFromBank(ctx, f.student.ID, f.assignment.ID, ids)
	if err != nil || len(copied) != 2 || copied[0].Position != 1 || copied[1].Position != 2 {
		t.Fatalf("retry failed: %+v %v", copied, err)
	}
	page, err := f.bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || !reflect.DeepEqual(page.Tasks, f.sources) {
		t.Fatalf("batch changed source tasks: %+v %v", page, err)
	}
}
