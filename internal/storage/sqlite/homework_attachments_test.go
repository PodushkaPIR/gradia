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
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/attachments"
	"github.com/shesh/gradia/internal/domain"
)

func TestHomeworkAttachmentsRetainOriginalBytesAfterSourceRemovalAndRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "attachments.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	otherStudent, err := students.Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewHomeworkService(store).WithAttachments(attachments.NewLocalFiles(filepath.Join(dir, "exports")), nil)
	homework, err := service.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	input := domain.HomeworkTaskInput{Content: "Решите задания из файла", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyMedium}
	task, err := service.AddTask(ctx, student.ID, homework.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	otherTask, err := service.AddTask(ctx, student.ID, homework.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	task, err = service.SetCompleted(ctx, student.ID, homework.ID, task.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\nORIGINAL_PDF_BYTES\n%%EOF\n")
	pdfSource := filepath.Join(dir, "оригинал #1.pdf")
	if err := os.WriteFile(pdfSource, pdf, 0600); err != nil {
		t.Fatal(err)
	}
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	imageSource := filepath.Join(dir, "изображение.png")
	if err := os.WriteFile(imageSource, imageBytes.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	pdfAttachment, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, pdfSource, " 2 - 5 ")
	if err != nil {
		t.Fatal(err)
	}
	imageAttachment, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, imageSource, "")
	if err != nil {
		t.Fatal(err)
	}
	if pdfAttachment.Name != filepath.Base(pdfSource) || pdfAttachment.Pages != "2-5" || pdfAttachment.MediaType != "application/pdf" || imageAttachment.MediaType != "image/png" || imageAttachment.Pages != "" {
		t.Fatalf("wrong attachment metadata: %+v %+v", pdfAttachment, imageAttachment)
	}
	if _, err := service.Attachments(ctx, otherStudent.ID, homework.ID, task.ID); err == nil {
		t.Fatal("foreign student's attachments visible")
	}
	if _, err := service.ExportAttachment(ctx, student.ID, homework.ID, otherTask.ID, pdfAttachment.ID); err == nil {
		t.Fatal("attachment from another task exported")
	}
	if _, err := service.ExportAttachment(ctx, otherStudent.ID, homework.ID, task.ID, pdfAttachment.ID); err == nil {
		t.Fatal("foreign student's file exported")
	}
	if actual, err := service.Tasks(ctx, student.ID, homework.ID); err != nil || len(actual) != 2 || !reflect.DeepEqual(actual[0], task) {
		t.Fatalf("attaching changed task data/completion: %+v %v", actual, err)
	}
	input.Content = "Обновлённый текст задачи"
	if _, err := service.EditTask(ctx, student.ID, homework.ID, task.ID, input); err != nil {
		t.Fatal(err)
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
	service = app.NewHomeworkService(reopened).WithAttachments(attachments.NewLocalFiles(filepath.Join(dir, "exports-after-restart")), nil)
	list, err := service.Attachments(ctx, student.ID, homework.ID, task.ID)
	if err != nil || len(list) != 2 || !reflect.DeepEqual(list[0], pdfAttachment) || !reflect.DeepEqual(list[1], imageAttachment) {
		t.Fatalf("attachments lost after task editing/restart: %+v %v", list, err)
	}
	for _, test := range []struct {
		id   domain.HomeworkAttachmentID
		data []byte
	}{{pdfAttachment.ID, pdf}, {imageAttachment.ID, imageBytes.Bytes()}} {
		exported, err := service.ExportAttachment(ctx, student.ID, homework.ID, task.ID, test.id)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(exported)
		if err != nil || !bytes.Equal(actual, test.data) {
			t.Fatalf("original file changed: %v", err)
		}
	}
	firstCopy, err := service.ExportAttachment(ctx, student.ID, homework.ID, task.ID, pdfAttachment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(firstCopy, []byte("edited local copy"), 0600); err != nil {
		t.Fatal(err)
	}
	secondCopy, err := service.ExportAttachment(ctx, student.ID, homework.ID, task.ID, pdfAttachment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if firstCopy == secondCopy {
		t.Fatal("export overwrote an existing local copy")
	}
	if actual, err := os.ReadFile(secondCopy); err != nil || !bytes.Equal(actual, pdf) {
		t.Fatalf("editing a local copy changed the original: %v", err)
	}
	if actual, err := os.ReadFile(firstCopy); err != nil || string(actual) != "edited local copy" {
		t.Fatalf("previous local copy was overwritten: %v", err)
	}
}

func TestCanceledHomeworkFileImportDoesNotSavePartialAttachment(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(dir, "cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	student, err := app.NewStudentService(store).Create(context.Background(), "Иван")
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewHomeworkService(store).WithAttachments(attachments.NewLocalFiles(filepath.Join(dir, "exports")), nil)
	homework, err := service.Create(context.Background(), student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.AddTask(context.Background(), student.ID, homework.ID, domain.HomeworkTaskInput{Content: "Задача", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "large.pdf")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("%PDF-1.4\n")); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Truncate(32 * 1024 * 1024); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(time.Millisecond, cancel)
	defer timer.Stop()
	if _, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, source, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled import accepted: %v", err)
	}
	if items, err := service.Attachments(context.Background(), student.ID, homework.ID, task.ID); err != nil || len(items) != 0 {
		t.Fatalf("canceled import left partial data: %+v %v", items, err)
	}
}

type trackingAttachmentFiles struct {
	app.HomeworkAttachmentFiles
	reads int
}

func (f *trackingAttachmentFiles) Read(ctx context.Context, path string) (domain.AttachmentFile, error) {
	f.reads++
	return f.HomeworkAttachmentFiles.Read(ctx, path)
}

func TestHomeworkAttachmentValidationAndExportFailuresPreserveOriginals(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, filepath.Join(dir, "validation.db"))
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
	files := &trackingAttachmentFiles{HomeworkAttachmentFiles: attachments.NewLocalFiles(filepath.Join(dir, "exports"))}
	service := app.NewHomeworkService(store).WithAttachments(files, nil)
	homework, err := service.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.AddTask(ctx, student.ID, homework.ID, domain.HomeworkTaskInput{Content: "Задача", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	pdfSource := filepath.Join(dir, "source.pdf")
	pdf := []byte("%PDF-1.4\nORIGINAL_DATA\n%%EOF\n")
	if err := os.WriteFile(pdfSource, pdf, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AttachFile(ctx, other.ID, homework.ID, task.ID, pdfSource, ""); err == nil || files.reads != 0 {
		t.Fatal("foreign request read a local file")
	}
	if _, err := service.AttachFile(ctx, student.ID, homework.ID, 99999, pdfSource, ""); !errors.Is(err, domain.ErrHomeworkTaskNotFound) || files.reads != 0 {
		t.Fatalf("missing task accepted or read file: %v", err)
	}
	for _, pages := range []string{"0", "5-2", "1,3", "-1", "2-3-4", "999999999999999999999999"} {
		if _, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, pdfSource, pages); !errors.Is(err, domain.ErrAttachmentPages) {
			t.Fatalf("invalid pages %q accepted: %v", pages, err)
		}
	}
	if files.reads != 0 {
		t.Fatal("invalid page syntax caused file reads")
	}
	textSource := filepath.Join(dir, "not-a-pdf.pdf")
	if err := os.WriteFile(textSource, []byte("plain text"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, textSource, ""); !errors.Is(err, domain.ErrAttachmentType) {
		t.Fatalf("unsupported content accepted: %v", err)
	}
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	imageSource := filepath.Join(dir, "image.png")
	if err := os.WriteFile(imageSource, imageBytes.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, imageSource, "2"); !errors.Is(err, domain.ErrAttachmentPagesOnlyPDF) {
		t.Fatalf("image accepted page metadata: %v", err)
	}
	if _, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, filepath.Join(dir, "missing.pdf"), ""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file accepted: %v", err)
	}
	if items, err := service.Attachments(ctx, student.ID, homework.ID, task.ID); err != nil || len(items) != 0 {
		t.Fatalf("failed import left metadata: %+v %v", items, err)
	}
	metadata, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, pdfSource, "007")
	if err != nil || metadata.Pages != "7" {
		t.Fatalf("single page not normalized: %+v %v", metadata, err)
	}
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	broken := service.WithAttachments(attachments.NewLocalFiles(blocked), nil)
	if _, err := broken.ExportAttachment(ctx, student.ID, homework.ID, task.ID, metadata.ID); err == nil {
		t.Fatal("expected export failure")
	}
	exported, err := service.ExportAttachment(ctx, student.ID, homework.ID, task.ID, metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if actual, err := os.ReadFile(exported); err != nil || !bytes.Equal(actual, pdf) {
		t.Fatalf("export failure lost stored original: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.ExportAttachment(canceled, student.ID, homework.ID, task.ID, metadata.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not propagated: %v", err)
	}
}
