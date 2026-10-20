package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/attachments"
	"github.com/shesh/gradia/internal/domain"
)

type importPackReader struct{ data app.TaskPackData }

func (r importPackReader) Read(context.Context, string) (app.TaskPackData, error) { return r.data, nil }

func TestTaskPackImportRollsBackLateWriteFailureAndPreservesHomework(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	homework := app.NewHomeworkService(store).WithAttachments(attachments.NewLocalFiles(dir), nil)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	task, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: "Личное задание", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := homework.SetCompleted(ctx, student.ID, assignment.ID, task.ID, true); err != nil {
		t.Fatal(err)
	}
	before, err := homework.Tasks(ctx, student.ID, assignment.ID)
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\nORIGINAL\n%%EOF\n")
	file := filepath.Join(dir, "homework.pdf")
	if err := os.WriteFile(file, pdf, 0600); err != nil {
		t.Fatal(err)
	}
	metadata, err := homework.AttachFile(ctx, student.ID, assignment.ID, task.ID, file, "3")
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(pdf)
	pack := app.TaskPackData{Assets: map[string][]byte{"assets/original.pdf": pdf}}
	for _, key := range []string{"a", "b"} {
		task := app.TaskPackTask{ID: strings.Repeat(key, 32), Content: "Импорт " + key, Subject: "Информатика", Topics: []string{"Функции"}, Difficulty: domain.DifficultyMedium, Collections: []string{"Новая коллекция"}, Attachments: []app.TaskPackAttachment{{Name: key + ".pdf", MediaType: "application/pdf", Path: "assets/original.pdf", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(pdf))}}}
		task.SHA256 = app.TaskPackFingerprint(task)
		pack.Tasks = append(pack.Tasks, task)
	}
	bank := app.NewTaskBankService(store).WithTaskPackReader(importPackReader{pack})
	// Fail only after the first task and its attachment were inserted.
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER fail_import BEFORE INSERT ON task_bank_attachments WHEN NEW.name = 'b.pdf' BEGIN SELECT RAISE(ABORT, 'injected disk failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if result, err := bank.ImportPack(ctx, "fixture.zip"); err == nil || result != (app.TaskPackImportResult{}) {
		t.Fatalf("failed import reported success: %+v %v", result, err)
	}
	page, err := bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks) != 0 || len(page.Collections) != 0 || len(page.Topics) != 0 {
		t.Fatalf("write failure left partial import: %+v %v", page, err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER fail_import`); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := bank.ImportPack(canceled, "fixture.zip"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if result, err := bank.ImportPack(ctx, "fixture.zip"); err != nil || result.Imported != 2 {
		t.Fatalf("retry failed: %+v %v", result, err)
	}
	if result, err := bank.ImportPack(ctx, "fixture.zip"); err != nil || result.Duplicates != 2 {
		t.Fatalf("repeated import: %+v %v", result, err)
	}
	if actual, err := homework.Tasks(ctx, student.ID, assignment.ID); err != nil || !reflect.DeepEqual(actual, before) {
		t.Fatalf("Homework changed: %+v %v", actual, err)
	}
	if actual, err := homework.Attachments(ctx, student.ID, assignment.ID, task.ID); err != nil || len(actual) != 1 || actual[0] != metadata {
		t.Fatalf("Homework attachments changed: %+v %v", actual, err)
	}
	exported, err := homework.ExportAttachment(ctx, student.ID, assignment.ID, task.ID, metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(exported); err != nil || string(data) != string(pdf) {
		t.Fatalf("Homework original changed: %v", err)
	}
}
