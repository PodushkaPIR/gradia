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

func TestHomeworkBankSnapshotPreservesOriginalsAndIndependentCompletion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	database := filepath.Join(dir, "snapshot.db")
	store, err := Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	template, err := app.NewTemplateService(store).Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	routes := app.NewRouteService(store)
	plan, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	topic, err := routes.AddTopic(ctx, plan.ID, 0, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	if err := routes.SetStatus(ctx, plan.ID, topic.ID, domain.ProgressLearning); err != nil {
		t.Fatal(err)
	}
	progress, err := routes.Topics(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	history, err := routes.ProgressHistory(ctx, plan.ID, topic.ID)
	if err != nil {
		t.Fatal(err)
	}
	files := attachments.NewLocalFiles(filepath.Join(dir, "copies"))
	homework := app.NewHomeworkService(store).WithAttachments(files, nil)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	bank := app.NewTaskBankService(store).WithAttachments(files, nil)
	input := domain.TaskBankInput{Content: "# Циклы\nНапишите функцию", Subject: "Информатика", Topics: []string{"Циклы", "Класс\nи функции"}, Difficulty: domain.DifficultyHard}
	source, err := bank.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\nORIGINAL\n%%EOF\n")
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name, pages string
		data        []byte
	}{{"source.pdf", "2-4", pdf}, {"source.png", "", imageData.Bytes()}} {
		path := filepath.Join(dir, item.name)
		if err := os.WriteFile(path, item.data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := bank.AttachFile(ctx, source.ID, path, item.pages); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	manual, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: "Ручная задача", Topics: []string{"Дроби"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	first, err := homework.AddFromBank(ctx, student.ID, assignment.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := homework.AddFromBank(ctx, student.ID, assignment.ID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.Completed || second.Completed || first.Position != manual.Position+1 || second.Position != first.Position+1 || first.Content != source.Content || first.Difficulty != source.Difficulty || !reflect.DeepEqual(first.Topics, source.Topics) {
		t.Fatalf("bad snapshots: %+v %+v", first, second)
	}
	if _, err := homework.SetCompleted(ctx, student.ID, assignment.ID, first.ID, true); err != nil {
		t.Fatal(err)
	}
	input.Content, input.Topics, input.Difficulty = "Изменено в банке", []string{"Другая тема"}, domain.DifficultyEasy
	if _, err := bank.Edit(ctx, source.ID, input); err != nil {
		t.Fatal(err)
	}
	assigned, err := homework.Tasks(ctx, student.ID, assignment.ID)
	if err != nil || len(assigned) != 3 || !assigned[1].Completed || assigned[2].Completed || assigned[1].Content != source.Content || !reflect.DeepEqual(assigned[1].Topics, source.Topics) {
		t.Fatalf("bank edit changed assigned tasks: %+v %v", assigned, err)
	}
	// Replacing/deleting a source row must not own the assigned snapshot's rows or BLOBs.
	if _, err := store.db.ExecContext(ctx, `DELETE FROM task_bank_attachments WHERE task_id = ?`, source.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM task_bank_tasks WHERE id = ?`, source.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	homework = app.NewHomeworkService(reopened).WithAttachments(files, nil)
	routes = app.NewRouteService(reopened)
	if actual, err := routes.Topics(ctx, plan.ID); err != nil || !reflect.DeepEqual(actual, progress) {
		t.Fatalf("Homework completion changed Learning Plan: %+v %v", actual, err)
	}
	if actual, err := routes.ProgressHistory(ctx, plan.ID, topic.ID); err != nil || !reflect.DeepEqual(actual, history) {
		t.Fatalf("Homework completion changed progress history: %+v %v", actual, err)
	}
	if actual, err := homework.Tasks(ctx, student.ID, assignment.ID); err != nil || !reflect.DeepEqual(actual, assigned) {
		t.Fatalf("snapshot did not persist: %+v %v", actual, err)
	}
	for _, snapshot := range []domain.HomeworkTask{first, second} {
		metadata, err := homework.Attachments(ctx, student.ID, assignment.ID, snapshot.ID)
		if err != nil || len(metadata) != 2 || metadata[0].Name != "source.pdf" || metadata[0].MediaType != "application/pdf" || metadata[0].Pages != "2-4" || metadata[1].MediaType != "image/png" || metadata[1].Pages != "" {
			t.Fatalf("attachment metadata lost: %+v %v", metadata, err)
		}
		for i, want := range [][]byte{pdf, imageData.Bytes()} {
			path, err := homework.ExportAttachment(ctx, student.ID, assignment.ID, snapshot.ID, metadata[i].ID)
			if err != nil {
				t.Fatal(err)
			}
			if actual, err := os.ReadFile(path); err != nil || !bytes.Equal(actual, want) {
				t.Fatalf("original lost: %v", err)
			}
		}
	}
	if _, err := homework.EditTask(ctx, student.ID, assignment.ID, first.ID, domain.HomeworkTaskInput{Content: "Правка копии", Topics: []string{"Функции"}, Difficulty: domain.DifficultyMedium}); err != nil {
		t.Fatal(err)
	}
	if tasks, err := homework.Tasks(ctx, student.ID, assignment.ID); err != nil || !tasks[1].Completed || tasks[2].Content != source.Content {
		t.Fatalf("editing snapshot affected another copy: %+v %v", tasks, err)
	}
}

func TestHomeworkBankSnapshotRejectsForeignTargetsAndRollsBackFailedCopies(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	other, err := students.Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	homework := app.NewHomeworkService(store)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bank := app.NewTaskBankService(store).WithAttachments(attachments.NewLocalFiles(dir), nil)
	source, err := bank.Create(ctx, domain.TaskBankInput{Content: "Задача", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.pdf", "two.pdf"} {
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte("%PDF-1.4\nORIGINAL\n%%EOF\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := bank.AttachFile(ctx, source.ID, file, "1"); err != nil {
			t.Fatal(err)
		}
	}
	beforeFiles, err := bank.Attachments(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		student  domain.StudentID
		homework domain.HomeworkID
		source   domain.TaskBankTaskID
		want     error
	}{
		{0, assignment.ID, source.ID, domain.ErrStudentNotFound},
		{other.ID, assignment.ID, source.ID, domain.ErrHomeworkNotFound},
		{student.ID, 99999, source.ID, domain.ErrHomeworkNotFound},
		{student.ID, assignment.ID, 99999, domain.ErrTaskBankTaskNotFound},
		{student.ID, assignment.ID, 0, domain.ErrTaskBankTaskNotFound},
	} {
		if _, err := homework.AddFromBank(ctx, test.student, test.homework, test.source); !errors.Is(err, test.want) {
			t.Fatalf("wrong scoped error: %v want %v", err, test.want)
		}
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER fail_snapshot BEFORE INSERT ON homework_attachments WHEN NEW.name = 'two.pdf' BEGIN SELECT RAISE(ABORT,'injected write failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := homework.AddFromBank(ctx, student.ID, assignment.ID, source.ID); err == nil {
		t.Fatal("injected failure ignored")
	}
	if tasks, err := homework.Tasks(ctx, student.ID, assignment.ID); err != nil || len(tasks) != 0 {
		t.Fatalf("partial snapshot: %+v %v", tasks, err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER fail_snapshot`); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := homework.AddFromBank(canceled, student.ID, assignment.ID, source.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if task, err := homework.AddFromBank(ctx, student.ID, assignment.ID, source.ID); err != nil || task.Position != 0 {
		t.Fatalf("retry failed: %+v %v", task, err)
	}
	if actual, err := bank.Attachments(ctx, source.ID); err != nil || !reflect.DeepEqual(actual, beforeFiles) {
		t.Fatalf("source attachments changed: %+v %v", actual, err)
	}
}
