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
	"github.com/shesh/gradia/internal/taskpacks"
)

func TestImageTaskPersistsOriginalAndSupportsHomeworkAndTaskPacks(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "image.db")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	files := attachments.NewLocalFiles(filepath.Join(dir, "exports"))
	bank := app.NewTaskBankService(store).WithAttachments(files, nil).WithPackWriter(taskpacks.NewZIPWriter(dir))
	collection, err := bank.CreateCollection(ctx, "Уравнения")
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 8))); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "задача #1.png")
	if err := os.WriteFile(source, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	input := domain.TaskBankInput{Content: "Уравнение 1", Subject: "Математика", Topics: []string{"Уравнения"}, Difficulty: domain.DifficultyMedium, CollectionIDs: []domain.TaskCollectionID{collection.ID}}
	task, err := bank.CreateFromImage(ctx, input, source)
	if err != nil {
		t.Fatal(err)
	}
	items, err := bank.Attachments(ctx, task.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("image missing: %+v %v", items, err)
	}
	metadata, original, err := bank.Attachment(ctx, task.ID, items[0].ID)
	if err != nil || !bytes.Equal(original, data.Bytes()) || metadata.Name != filepath.Base(source) || metadata.MediaType != "image/png" || metadata.Pages != "" {
		t.Fatalf("original changed: %+v %v", metadata, err)
	}
	if _, err := bank.AttachFile(ctx, task.ID, source, ""); err != nil {
		t.Fatal(err)
	}
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	homework := app.NewHomeworkService(store)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := homework.AddFromBank(ctx, student.ID, assignment.ID, task.ID)
	if err != nil || assigned.Content != input.Content || assigned.Completed {
		t.Fatalf("snapshot failed: %+v %v", assigned, err)
	}
	hwImages, err := homework.Attachments(ctx, student.ID, assignment.ID, assigned.ID)
	if err != nil || len(hwImages) != 2 {
		t.Fatalf("snapshot images missing: %+v %v", hwImages, err)
	}
	_, hwBytes, err := store.GetHomeworkAttachment(ctx, student.ID, assignment.ID, assigned.ID, hwImages[0].ID)
	if err != nil || !bytes.Equal(hwBytes, data.Bytes()) {
		t.Fatalf("snapshot bytes changed: %v", err)
	}
	input.Content = "Новое название"
	task, err = bank.Edit(ctx, task.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	packPath, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{task.ID})
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(ctx, filepath.Join(dir, "import.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	imported := app.NewTaskBankService(other).WithPackReader(taskpacks.NewZIPReader())
	if result, err := imported.ImportPack(ctx, packPath); err != nil || result.Imported != 1 {
		t.Fatalf("image pack import: %+v %v", result, err)
	}
	page, err := imported.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks) != 1 || page.Tasks[0].Content != input.Content {
		t.Fatalf("import lost title: %+v %v", page, err)
	}
	importImages, err := imported.Attachments(ctx, page.Tasks[0].ID)
	if err != nil || len(importImages) != 2 {
		t.Fatalf("import lost images: %+v %v", importImages, err)
	}
	_, importedBytes, err := imported.Attachment(ctx, page.Tasks[0].ID, importImages[0].ID)
	if err != nil || !bytes.Equal(importedBytes, data.Bytes()) {
		t.Fatalf("import changed image: %v", err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	bank = app.NewTaskBankService(store)
	page, err = bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks) != 1 || !reflect.DeepEqual(page.Tasks[0], task) {
		t.Fatalf("restart changed task: %+v %v", page, err)
	}
	_, original, err = bank.Attachment(ctx, task.ID, metadata.ID)
	if err != nil || !bytes.Equal(original, data.Bytes()) {
		t.Fatalf("restart lost image: %v", err)
	}
	hwTasks, err := app.NewHomeworkService(store).Tasks(ctx, student.ID, assignment.ID)
	if err != nil || len(hwTasks) != 1 || hwTasks[0].Content != assigned.Content {
		t.Fatalf("bank edit changed snapshot: %+v %v", hwTasks, err)
	}
}

func TestImageTaskValidationAndLateFailureLeaveNoPartialTask(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, filepath.Join(dir, "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bank := app.NewTaskBankService(store).WithAttachments(attachments.NewLocalFiles(dir), nil)
	input := domain.TaskBankInput{Content: "Название", Subject: "Математика", Topics: []string{"Уравнения"}, Difficulty: domain.DifficultyEasy}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "image.png")
	if err := os.WriteFile(source, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		data []byte
		want error
	}{
		{"text.png", []byte("not an image"), domain.ErrAttachmentType},
		{"pdf.png", []byte("%PDF-1.4\n%%EOF"), app.ErrTaskImageRequired},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, test.name)
			if err := os.WriteFile(path, test.data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := bank.CreateFromImage(ctx, input, path); !errors.Is(err, test.want) {
				t.Fatalf("wrong validation: %v", err)
			}
		})
	}
	for _, path := range []string{"", filepath.Join(dir, "missing.png"), dir} {
		if _, err := bank.CreateFromImage(ctx, input, path); err == nil {
			t.Fatalf("accepted invalid path %q", path)
		}
	}
	if _, err := app.NewTaskBankService(store).CreateFromImage(ctx, input, source); !errors.Is(err, app.ErrAttachmentFilesUnavailable) {
		t.Fatalf("missing adapter: %v", err)
	}
	invalid := input
	invalid.Content = ""
	if _, err := bank.CreateFromImage(ctx, invalid, source); !errors.Is(err, domain.ErrTaskContentRequired) {
		t.Fatalf("missing title: %v", err)
	}
	invalid = input
	invalid.CollectionIDs = []domain.TaskCollectionID{999}
	if _, err := bank.CreateFromImage(ctx, invalid, source); !errors.Is(err, domain.ErrTaskCollectionNotFound) {
		t.Fatalf("missing collection: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := bank.CreateFromImage(canceled, input, source); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER fail_image BEFORE INSERT ON task_bank_attachments BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.CreateFromImage(ctx, input, source); err == nil {
		t.Fatal("late failure ignored")
	}
	for _, table := range []string{"task_bank_tasks", "task_bank_task_topics", "task_bank_collection_tasks", "task_bank_task_identity", "task_bank_attachments"} {
		var count int
		if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial data in %s: %d %v", table, count, err)
		}
	}
	if original, err := os.ReadFile(source); err != nil || !bytes.Equal(original, data.Bytes()) {
		t.Fatalf("source changed: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER fail_image`); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.CreateFromImage(ctx, input, source); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
}
