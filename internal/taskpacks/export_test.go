package taskpacks_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/attachments"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
	"github.com/shesh/gradia/internal/taskpacks"
)

func readTaskPack(t *testing.T, filename string) (taskpacks.Manifest, map[string][]byte) {
	t.Helper()
	archive, err := zip.OpenReader(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files := make(map[string][]byte)
	for _, file := range archive.File {
		if path.IsAbs(file.Name) || strings.Contains(file.Name, "..") {
			t.Fatalf("unsafe archive name %q", file.Name)
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = data
	}
	var manifest taskpacks.Manifest
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest, files
}

func TestTaskPackExportsOnlySelectedTasksMetadataAndOriginalAssets(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	database := filepath.Join(dir, "bank.db")
	store, err := sqlite.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bank := app.NewTaskBankService(store).WithAttachments(attachments.NewLocalFiles(filepath.Join(dir, "copies")), nil).WithTaskPackWriter(taskpacks.NewZIPWriter(filepath.Join(dir, "packs")))
	exam, err := bank.CreateCollection(ctx, "Экзамен")
	if err != nil {
		t.Fatal(err)
	}
	practice, err := bank.CreateCollection(ctx, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bank.CreateCollection(ctx, "UNSELECTED_COLLECTION"); err != nil {
		t.Fatal(err)
	}
	first, err := bank.Create(ctx, domain.TaskBankInput{Content: "# Циклы\nНапишите функцию", Subject: "Информатика", Topics: []string{"Циклы", "Функции; строки", "Класс\nи функции"}, Difficulty: domain.DifficultyMedium, CollectionIDs: []domain.TaskCollectionID{exam.ID, practice.ID}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := bank.Create(ctx, domain.TaskBankInput{Content: "https://example.org/task?x='a'", Subject: "Математика", Topics: []string{"Дроби"}, Difficulty: domain.DifficultyHard, CollectionIDs: []domain.TaskCollectionID{exam.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bank.Create(ctx, domain.TaskBankInput{Content: "UNSELECTED_TASK", Subject: "UNSELECTED_SUBJECT", Topics: []string{"UNSELECTED_TOPIC"}, Difficulty: domain.DifficultyEasy}); err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\nORIGINAL_PDF_BYTES\n%%EOF\n")
	source := filepath.Join(dir, "оригинал #1.pdf")
	if err := os.WriteFile(source, pdf, 0600); err != nil {
		t.Fatal(err)
	}
	pdfMeta, err := bank.AttachFile(ctx, first.ID, source, "2-4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bank.AttachFile(ctx, second.ID, source, "7"); err != nil {
		t.Fatal(err)
	}
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(dir, "image.png")
	if err := os.WriteFile(imagePath, imageBytes.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.AttachFile(ctx, first.ID, imagePath, ""); err != nil {
		t.Fatal(err)
	}
	before, err := bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil {
		t.Fatal(err)
	}
	beforeFiles, err := bank.Attachments(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	filename, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{second.ID, first.ID, second.ID})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(filename) != ".zip" {
		t.Fatalf("pack is not a ZIP file: %q", filename)
	}
	manifest, files := readTaskPack(t, filename)
	if manifest.Format != "gradia-task-pack" || manifest.Version != 1 || len(manifest.Tasks) != 2 {
		t.Fatalf("wrong manifest: %+v", manifest)
	}
	for i, sourceTask := range []domain.TaskBankTask{first, second} {
		task := manifest.Tasks[i]
		if task.ID == "" || len(task.SHA256) != 64 || task.Content != sourceTask.Content || task.Subject != sourceTask.Subject || task.Difficulty != sourceTask.Difficulty || !reflect.DeepEqual(task.Topics, sourceTask.Topics) {
			t.Fatalf("task metadata lost: %+v", task)
		}
	}
	if !reflect.DeepEqual(manifest.Tasks[0].Collections, []string{"Практика", "Экзамен"}) || !reflect.DeepEqual(manifest.Tasks[1].Collections, []string{"Экзамен"}) {
		t.Fatal("collection membership not portable")
	}
	if len(files) != 3 {
		t.Fatalf("identical PDF bytes should be stored once alongside PNG and manifest: %d", len(files))
	}
	if len(manifest.Tasks[0].Attachments) != 2 || len(manifest.Tasks[1].Attachments) != 1 {
		t.Fatal("attachments missing")
	}
	pdfItem := manifest.Tasks[0].Attachments[0]
	if pdfItem.Name != pdfMeta.Name || pdfItem.MediaType != "application/pdf" || pdfItem.Pages != "2-4" || !bytes.Equal(files[pdfItem.Path], pdf) {
		t.Fatalf("original PDF or pages lost: %+v", pdfItem)
	}
	if !bytes.Equal(files[manifest.Tasks[0].Attachments[1].Path], imageBytes.Bytes()) || manifest.Tasks[1].Attachments[0].Pages != "7" || manifest.Tasks[1].Attachments[0].Path != pdfItem.Path {
		t.Fatal("original image or repeated PDF references lost")
	}
	for _, forbidden := range []string{"UNSELECTED_TASK", "UNSELECTED_SUBJECT", "UNSELECTED_TOPIC", "UNSELECTED_COLLECTION", dir} {
		if bytes.Contains(files["manifest.json"], []byte(forbidden)) {
			t.Fatalf("pack exposed %q", forbidden)
		}
	}
	repeated, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{first.ID, second.ID})
	if err != nil {
		t.Fatal(err)
	}
	if repeated == filename {
		t.Fatal("re-export overwrote previous pack")
	}
	secondManifest, _ := readTaskPack(t, repeated)
	if !reflect.DeepEqual(manifest, secondManifest) {
		t.Fatal("unchanged tasks changed identities or fingerprints between exports")
	}
	if actual, err := bank.Browse(ctx, domain.TaskBankFilter{}); err != nil || !reflect.DeepEqual(actual, before) {
		t.Fatalf("export changed source bank: %+v %v", actual, err)
	}
	if actual, err := bank.Attachments(ctx, first.ID); err != nil || !reflect.DeepEqual(actual, beforeFiles) {
		t.Fatalf("export changed attachments: %+v %v", actual, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	bank = app.NewTaskBankService(reopened).WithTaskPackWriter(taskpacks.NewZIPWriter(filepath.Join(dir, "packs")))
	afterRestart, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{first.ID, second.ID})
	if err != nil {
		t.Fatal(err)
	}
	restartedManifest, _ := readTaskPack(t, afterRestart)
	if !reflect.DeepEqual(manifest, restartedManifest) {
		t.Fatal("restart changed portable identities or metadata")
	}
	if _, err := bank.Edit(ctx, first.ID, domain.TaskBankInput{Content: first.Content + "\nОбновление", Subject: first.Subject, Topics: first.Topics, Difficulty: first.Difficulty, CollectionIDs: first.CollectionIDs}); err != nil {
		t.Fatal(err)
	}
	changed, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{first.ID})
	if err != nil {
		t.Fatal(err)
	}
	changedManifest, _ := readTaskPack(t, changed)
	if changedManifest.Tasks[0].ID != manifest.Tasks[0].ID || changedManifest.Tasks[0].SHA256 == manifest.Tasks[0].SHA256 {
		t.Fatal("editing should retain identity but change content fingerprint")
	}
}

type packReadTestStore struct {
	*sqlite.Store
	readErr error
	cancel  context.CancelFunc
}

func (s *packReadTestStore) GetBankAttachment(ctx context.Context, task domain.TaskBankTaskID, id domain.TaskBankAttachmentID) (domain.TaskBankAttachment, []byte, error) {
	if s.readErr != nil {
		return domain.TaskBankAttachment{}, nil, s.readErr
	}
	if s.cancel != nil {
		s.cancel()
	}
	return s.Store.GetBankAttachment(ctx, task, id)
}

func TestTaskPackFailuresDoNotPublishPartialZIPOrOverwriteEarlierPack(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "errors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	repo := &packReadTestStore{Store: store}
	packs := filepath.Join(dir, "packs")
	bank := app.NewTaskBankService(repo).WithAttachments(attachments.NewLocalFiles(filepath.Join(dir, "copies")), nil).WithTaskPackWriter(taskpacks.NewZIPWriter(packs))
	task, err := bank.Create(ctx, domain.TaskBankInput{Content: "Задача", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "source.pdf")
	if err := os.WriteFile(source, []byte("%PDF-1.4\nORIGINAL\n%%EOF\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.AttachFile(ctx, task.ID, source, ""); err != nil {
		t.Fatal(err)
	}
	previous, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{task.ID})
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(previous)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bank.ExportPack(ctx, nil); !errors.Is(err, app.ErrTaskPackSelectionRequired) {
		t.Fatalf("empty selection accepted: %v", err)
	}
	if _, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{99999}); !errors.Is(err, domain.ErrTaskBankTaskNotFound) {
		t.Fatalf("missing task accepted: %v", err)
	}
	repo.readErr = errors.New("attachment unavailable")
	if path, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{task.ID}); !errors.Is(err, repo.readErr) || path != "" {
		t.Fatalf("asset failure published partial pack: %q %v", path, err)
	}
	repo.readErr = nil
	canceled, cancel := context.WithCancel(ctx)
	defer cancel()
	repo.cancel = cancel
	if path, err := bank.ExportPack(canceled, []domain.TaskBankTaskID{task.ID}); !errors.Is(err, context.Canceled) || path != "" {
		t.Fatalf("canceled export published pack: %q %v", path, err)
	}
	repo.cancel = nil
	files, err := os.ReadDir(packs)
	if err != nil || len(files) != 1 {
		t.Fatalf("failed exports left partial files: %v %v", files, err)
	}
	if actual, err := os.ReadFile(previous); err != nil || !bytes.Equal(actual, original) {
		t.Fatalf("previous pack changed: %v", err)
	}
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.WithTaskPackWriter(taskpacks.NewZIPWriter(blocked)).ExportPack(ctx, []domain.TaskBankTaskID{task.ID}); err == nil {
		t.Fatal("expected destination failure")
	}
}
