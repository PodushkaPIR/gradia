//go:build unix

package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/attachments"
	"github.com/shesh/gradia/internal/domain"
)

func TestHomeworkRejectsNonRegularAttachmentSourcesWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(dir, "test.db"))
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
	fifo := filepath.Join(dir, "not-a-file.pdf")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := service.AttachFile(ctx, student.ID, homework.ID, task.ID, fifo, ""); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, app.ErrAttachmentRegularFileRequired) {
			t.Fatalf("wrong source error: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		// Release a regressed blocking reader before failing the test.
		writer, err := os.OpenFile(fifo, os.O_RDWR|syscall.O_NONBLOCK, 0600)
		if err == nil {
			_, _ = writer.Write([]byte("not a PDF"))
			_ = writer.Close()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("import blocked opening a non-regular source")
	}
	if _, err := service.AttachFile(context.Background(), student.ID, homework.ID, task.ID, dir, ""); !errors.Is(err, app.ErrAttachmentRegularFileRequired) {
		t.Fatalf("directory accepted as attachment: %v", err)
	}
}
