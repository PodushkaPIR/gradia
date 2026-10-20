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

func TestBankAttachmentsPreserveOriginalsAndStayIndependentOfHomework(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "bank-attachments.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	files := attachments.NewLocalFiles(filepath.Join(dir, "exports"))
	bank := app.NewTaskBankService(store).WithAttachments(files, nil)
	input := domain.TaskBankInput{Content: "Задача с файлами", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyMedium}
	task, err := bank.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	other, err := bank.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\nBANK_ORIGINAL_PDF\n%%EOF\n")
	pdfSource := filepath.Join(dir, "банк #1.pdf")
	if err := os.WriteFile(pdfSource, pdf, 0600); err != nil {
		t.Fatal(err)
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	imageSource := filepath.Join(dir, "image.png")
	if err := os.WriteFile(imageSource, pngData.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	pdfItem, err := bank.AttachFile(ctx, task.ID, pdfSource, " 2 - 4 ")
	if err != nil {
		t.Fatal(err)
	}
	imageItem, err := bank.AttachFile(ctx, task.ID, imageSource, "")
	if err != nil {
		t.Fatal(err)
	}
	if pdfItem.Pages != "2-4" || pdfItem.MediaType != "application/pdf" || imageItem.MediaType != "image/png" || imageItem.Pages != "" {
		t.Fatalf("wrong metadata: %+v %+v", pdfItem, imageItem)
	}
	if _, _, err := bank.Attachment(ctx, other.ID, pdfItem.ID); !errors.Is(err, domain.ErrTaskBankAttachmentNotFound) {
		t.Fatalf("cross-task file visible: %v", err)
	}
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	hw := app.NewHomeworkService(store).WithAttachments(files, nil)
	assignment, err := hw.Create(ctx, student.ID, "ДЗ")
	if err != nil {
		t.Fatal(err)
	}
	hwTask, err := hw.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: "Ручная задача", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	hwFile, err := hw.AttachFile(ctx, student.ID, assignment.ID, hwTask.ID, imageSource, "")
	if err != nil {
		t.Fatal(err)
	}
	if int64(hwTask.ID) != int64(task.ID) || int64(hwFile.ID) != int64(pdfItem.ID) {
		t.Fatal("fixture must exercise colliding local IDs in independent tables")
	}
	input.Content = "Отредактированная задача"
	if _, err := bank.Edit(ctx, task.ID, input); err != nil {
		t.Fatal(err)
	}
	if actual, err := hw.Attachments(ctx, student.ID, assignment.ID, hwTask.ID); err != nil || len(actual) != 1 || !reflect.DeepEqual(actual[0], hwFile) {
		t.Fatalf("bank edit changed Homework attachments: %+v %v", actual, err)
	}
	if err := os.Remove(pdfSource); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(imageSource); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	bank = app.NewTaskBankService(reopened)
	items, err := bank.Attachments(ctx, task.ID)
	if err != nil || len(items) != 2 || !reflect.DeepEqual(items[0], pdfItem) || !reflect.DeepEqual(items[1], imageItem) {
		t.Fatalf("attachments lost after edit/restart: %+v %v", items, err)
	}
	for _, test := range []struct {
		id   domain.TaskBankAttachmentID
		data []byte
	}{{pdfItem.ID, pdf}, {imageItem.ID, pngData.Bytes()}} {
		// Portable metadata and bytes are available without a materialization adapter for Task Packs.
		_, actual, err := bank.Attachment(ctx, task.ID, test.id)
		if err != nil || !bytes.Equal(actual, test.data) {
			t.Fatalf("raw attachment changed: %v", err)
		}
	}
	bank = bank.WithAttachments(attachments.NewLocalFiles(filepath.Join(dir, "reopened-exports")), nil)
	exported, err := bank.ExportAttachment(ctx, task.ID, pdfItem.ID)
	if err != nil {
		t.Fatal(err)
	}
	if actual, err := os.ReadFile(exported); err != nil || !bytes.Equal(actual, pdf) {
		t.Fatalf("export lost original: %v", err)
	}
}

func TestBankAttachmentFailuresKeepTaskAndOriginalData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, filepath.Join(dir, "errors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	files := &trackingAttachmentFiles{HomeworkAttachmentFiles: attachments.NewLocalFiles(filepath.Join(dir, "exports"))}
	bank := app.NewTaskBankService(store).WithAttachments(files, nil)
	task, err := bank.Create(ctx, domain.TaskBankInput{Content: "Задача", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("%PDF-1.4\nORIGINAL_BYTES\n%%EOF\n")
	source := filepath.Join(dir, "source.pdf")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.AttachFile(ctx, 99999, source, ""); !errors.Is(err, domain.ErrTaskBankTaskNotFound) || files.reads != 0 {
		t.Fatalf("missing task caused file read: %v", err)
	}
	if _, err := bank.AttachFile(ctx, task.ID, source, "5-2"); !errors.Is(err, domain.ErrAttachmentPages) || files.reads != 0 {
		t.Fatalf("bad range accepted or read file: %v", err)
	}
	text := filepath.Join(dir, "text.pdf")
	if err := os.WriteFile(text, []byte("plain text"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.AttachFile(ctx, task.ID, text, ""); !errors.Is(err, domain.ErrAttachmentType) {
		t.Fatalf("unsupported input accepted: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_bank_attachment BEFORE INSERT ON task_bank_attachments BEGIN SELECT RAISE(ABORT, 'storage unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.AttachFile(ctx, task.ID, source, "1"); err == nil {
		t.Fatal("expected storage error")
	}
	if items, err := bank.Attachments(ctx, task.ID); err != nil || len(items) != 0 {
		t.Fatalf("failed import left a partial attachment: %+v %v", items, err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER reject_bank_attachment`); err != nil {
		t.Fatal(err)
	}
	metadata, err := bank.AttachFile(ctx, task.ID, source, "007")
	if err != nil || metadata.Pages != "7" {
		t.Fatalf("single page not retained: %+v %v", metadata, err)
	}
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	broken := bank.WithAttachments(attachments.NewLocalFiles(blocked), nil)
	if _, err := broken.ExportAttachment(ctx, task.ID, metadata.ID); err == nil {
		t.Fatal("expected materialization failure")
	}
	actualMeta, actual, err := bank.Attachment(ctx, task.ID, metadata.ID)
	if err != nil || !reflect.DeepEqual(actualMeta, metadata) || !bytes.Equal(actual, data) {
		t.Fatalf("export error lost original: %+v %v", actualMeta, err)
	}
	if page, err := bank.Browse(ctx, domain.TaskBankFilter{}); err != nil || len(page.Tasks) != 1 || !reflect.DeepEqual(page.Tasks[0], task) {
		t.Fatalf("attachment operation changed bank task: %+v %v", page, err)
	}
}
