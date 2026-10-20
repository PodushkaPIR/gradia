package taskpacks_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/attachments"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
	"github.com/shesh/gradia/internal/taskpacks"
)

func TestTaskPackImportPreservesUnusualOriginalNamesWhenMaterializingFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("original filenames are valid on Unix")
	}
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "names.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bank := app.NewTaskBankService(store).WithAttachments(attachments.NewLocalFiles(dir), nil).WithTaskPackReader(taskpacks.NewZIPReader()).WithTaskPackWriter(taskpacks.NewZIPWriter(dir))
	task, err := bank.Create(ctx, domain.TaskBankInput{Content: "Задача", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	original := []byte("%PDF-1.4\nORIGINAL\n%%EOF\n")
	names := []string{"chapter\\notes.pdf", "notes\npage.pdf", "CON.pdf", "a:b?.pdf", "COM¹.pdf", "COM².pdf", "COM³.pdf", "LPT¹.pdf", "LPT².pdf", "LPT³.pdf"}
	for _, name := range names {
		filename := filepath.Join(dir, name)
		if err := os.WriteFile(filename, original, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := bank.AttachFile(ctx, task.ID, filename, ""); err != nil {
			t.Fatal(err)
		}
	}
	pack, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := bank.ImportPack(ctx, pack); err != nil || result.Duplicates != 1 {
		t.Fatalf("app-generated pack rejected: %+v %v", result, err)
	}
	targetStore, err := sqlite.Open(ctx, filepath.Join(dir, "target-names.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer targetStore.Close()
	target := app.NewTaskBankService(targetStore).WithTaskPackReader(taskpacks.NewZIPReader()).WithAttachments(attachments.NewLocalFiles(filepath.Join(dir, "copies")), nil)
	if _, err := target.ImportPack(ctx, pack); err != nil {
		t.Fatal(err)
	}
	page, err := target.Browse(ctx, domain.TaskBankFilter{})
	if err != nil {
		t.Fatal(err)
	}
	files, err := target.Attachments(ctx, page.Tasks[0].ID)
	if err != nil || len(files) != len(names) {
		t.Fatalf("attachments missing: %+v %v", files, err)
	}
	for i, file := range files {
		if file.Name != names[i] {
			t.Fatalf("original name changed: %q", file.Name)
		}
		exported, err := target.ExportAttachment(ctx, page.Tasks[0].ID, file.ID)
		if err != nil {
			t.Fatal(err)
		}
		if bytes, err := os.ReadFile(exported); err != nil || string(bytes) != string(original) {
			t.Fatalf("original unavailable: %v", err)
		}
		name := filepath.Base(exported)
		stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
		reserved := map[string]bool{"CON": true, "COM¹": true, "COM²": true, "COM³": true, "LPT¹": true, "LPT²": true, "LPT³": true}
		if strings.ContainsAny(name, "\\\r\n:\x00?*") || reserved[stem] {
			t.Fatalf("materialized filename is not portable: %q", name)
		}
	}
}

func importFixture() (taskpacks.Manifest, map[string][]byte) {
	data := []byte("%PDF-1.4\nPACK_PDF\n%%EOF\n")
	hash := sha256.Sum256(data)
	first := taskpacks.Task{ID: strings.Repeat("a", 32), Content: "Первая задача", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyMedium, Collections: []string{"Новая коллекция"}}
	second := first
	second.ID, second.Content = strings.Repeat("b", 32), "Вторая задача"
	second.Attachments = []taskpacks.Attachment{{Name: "source.pdf", MediaType: "application/pdf", Pages: "1-2", Path: "assets/source.pdf", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(data))}}
	first.SHA256, second.SHA256 = taskpacks.TaskFingerprint(first), taskpacks.TaskFingerprint(second)
	return taskpacks.Manifest{Format: taskpacks.Format, Version: taskpacks.Version, Tasks: []taskpacks.Task{first, second}}, map[string][]byte{"assets/source.pdf": data}
}

func writeImportFixture(t *testing.T, manifest taskpacks.Manifest, assets map[string][]byte, duplicateManifest bool) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "pack-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, data := range assets {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	count := 1
	if duplicateManifest {
		count++
	}
	for range count {
		entry, err := archive.Create("manifest.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewEncoder(entry).Encode(manifest); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

func TestTaskPackRejectsInvalidPacksWithoutImportingEarlierValidTasks(t *testing.T) {
	for _, test := range []struct {
		name              string
		mutate            func(*taskpacks.Manifest, map[string][]byte)
		duplicateManifest bool
	}{
		{"format", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Format = "other" }, false},
		{"version", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Version++ }, false},
		{"empty", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks = nil }, false},
		{"identity", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].ID = "invalid" }, false},
		{"repeated identity", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].ID = m.Tasks[0].ID }, false},
		{"task checksum", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Content = "Tampered" }, false},
		{"asset checksum", func(_ *taskpacks.Manifest, a map[string][]byte) { a["assets/source.pdf"][12] = 'X' }, false},
		{"missing asset", func(_ *taskpacks.Manifest, a map[string][]byte) { delete(a, "assets/source.pdf") }, false},
		{"size", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Attachments[0].Size++ }, false},
		{"subject", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Subject = "" }, false},
		{"difficulty", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Difficulty = "unknown" }, false},
		{"topics", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Topics = nil }, false},
		{"collections", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Collections = []string{""} }, false},
		{"pages", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Attachments[0].Pages = "4-2" }, false},
		{"media type", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Attachments[0].MediaType = "image/png" }, false},
		{"filename", func(m *taskpacks.Manifest, _ map[string][]byte) { m.Tasks[1].Attachments[0].Name = "../source.pdf" }, false},
		{"traversal", func(_ *taskpacks.Manifest, a map[string][]byte) { a["assets/../../outside.pdf"] = []byte("escape") }, false},
		{"windows path", func(_ *taskpacks.Manifest, a map[string][]byte) { a[`assets\..\outside.pdf`] = []byte("escape") }, false},
		{"duplicate manifest", nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, assets := importFixture()
			if test.mutate != nil {
				test.mutate(&manifest, assets)
			}
			// Invalid semantic metadata must be rejected even with a matching task fingerprint.
			if test.name != "task checksum" {
				for i := range manifest.Tasks {
					manifest.Tasks[i].SHA256 = taskpacks.TaskFingerprint(manifest.Tasks[i])
				}
			}
			filename := writeImportFixture(t, manifest, assets, test.duplicateManifest)
			ctx := context.Background()
			store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "invalid.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			bank := app.NewTaskBankService(store).WithTaskPackReader(taskpacks.NewZIPReader())
			if result, err := bank.ImportPack(ctx, filename); !errors.Is(err, app.ErrInvalidTaskPack) || result != (app.TaskPackImportResult{}) {
				t.Fatalf("bad pack accepted: %+v %v", result, err)
			}
			page, err := bank.Browse(ctx, domain.TaskBankFilter{})
			if err != nil || len(page.Tasks) != 0 || len(page.Collections) != 0 || len(page.Subjects) != 0 || len(page.Topics) != 0 {
				t.Fatalf("partial import: %+v %v", page, err)
			}
		})
	}
}

func TestTaskPackImportRoundTripSkipsDuplicatesAndKeepsLocalEdits(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	sourceStore, err := sqlite.Open(ctx, filepath.Join(dir, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sourceStore.Close()
	source := app.NewTaskBankService(sourceStore).WithAttachments(attachments.NewLocalFiles(dir), nil).WithTaskPackWriter(taskpacks.NewZIPWriter(dir))
	collection, err := source.CreateCollection(ctx, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	input := domain.TaskBankInput{Content: "# Задача\nЦиклы", Subject: "Информатика", Topics: []string{"Python; циклы", "Класс\nи функции"}, Difficulty: domain.DifficultyMedium, CollectionIDs: []domain.TaskCollectionID{collection.ID}}
	task, err := source.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("%PDF-1.4\nORIGINAL\n%%EOF\n")
	file := filepath.Join(dir, "оригинал.pdf")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := source.AttachFile(ctx, task.ID, file, "2-4"); err != nil {
		t.Fatal(err)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	imageFile := filepath.Join(dir, "image.png")
	if err := os.WriteFile(imageFile, imageData.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := source.AttachFile(ctx, task.ID, imageFile, ""); err != nil {
		t.Fatal(err)
	}
	pack, err := source.ExportPack(ctx, []domain.TaskBankTaskID{task.ID})
	if err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(dir, "target.db")
	targetStore, err := sqlite.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer targetStore.Close()
	target := app.NewTaskBankService(targetStore).WithTaskPackReader(taskpacks.NewZIPReader()).WithTaskPackWriter(taskpacks.NewZIPWriter(dir))
	// Existing local IDs and collection IDs must not be treated as portable identities.
	if _, err := target.Create(ctx, domain.TaskBankInput{Content: "Локальная задача", Subject: "Математика", Topics: []string{"Дроби"}, Difficulty: domain.DifficultyEasy}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreateCollection(ctx, "Другая"); err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreateCollection(ctx, "Практика"); err != nil {
		t.Fatal(err)
	}
	result, err := target.ImportPack(ctx, pack)
	if err != nil || result.Imported != 1 || result.Duplicates != 0 || result.Conflicts != 0 {
		t.Fatalf("import: %+v %v", result, err)
	}
	page, err := target.Browse(ctx, domain.TaskBankFilter{Subject: input.Subject})
	if err != nil || len(page.Tasks) != 1 {
		t.Fatalf("imported task absent: %+v %v", page, err)
	}
	imported := page.Tasks[0]
	if imported.ID == task.ID || imported.Content != input.Content || imported.Difficulty != input.Difficulty || !reflect.DeepEqual(imported.Topics, input.Topics) {
		t.Fatalf("metadata not preserved: %+v", imported)
	}
	if len(imported.CollectionIDs) != 1 || imported.CollectionIDs[0] != page.Collections[1].ID {
		t.Fatalf("collection name was not reused: %+v", page)
	}
	files, err := target.Attachments(ctx, imported.ID)
	if err != nil || len(files) != 2 {
		t.Fatalf("attachments: %+v %v", files, err)
	}
	meta, original, err := target.Attachment(ctx, imported.ID, files[0].ID)
	if err != nil || string(original) != string(data) || meta.Pages != "2-4" || meta.Name != "оригинал.pdf" {
		t.Fatalf("original data lost: %+v %v", meta, err)
	}
	if _, original, err := target.Attachment(ctx, imported.ID, files[1].ID); err != nil || !bytes.Equal(original, imageData.Bytes()) {
		t.Fatalf("original image lost: %v", err)
	}
	roundTrip, err := target.ExportPack(ctx, []domain.TaskBankTaskID{imported.ID})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := readTaskPack(t, pack)
	actual, _ := readTaskPack(t, roundTrip)
	if !reflect.DeepEqual(want, actual) {
		t.Fatal("portable metadata/identity changed on import")
	}
	if result, err := target.ImportPack(ctx, pack); err != nil || result.Duplicates != 1 || result.Imported != 0 || result.Conflicts != 0 {
		t.Fatalf("repeat import: %+v %v", result, err)
	}
	input.Content = "Изменено локально"
	input.CollectionIDs = imported.CollectionIDs
	if _, err := target.Edit(ctx, imported.ID, input); err != nil {
		t.Fatal(err)
	}
	before, err := target.Browse(ctx, domain.TaskBankFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := target.ImportPack(ctx, pack); err != nil || result.Conflicts != 1 || result.Imported != 0 || result.Duplicates != 0 {
		t.Fatalf("conflict: %+v %v", result, err)
	}
	if after, err := target.Browse(ctx, domain.TaskBankFilter{}); err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("local task changed: %+v %v", after, err)
	}
	if after, err := target.Attachments(ctx, imported.ID); err != nil || !reflect.DeepEqual(after, files) {
		t.Fatalf("local attachments changed on conflict: %+v %v", after, err)
	}
	newTask, err := source.Create(ctx, domain.TaskBankInput{Content: "Новая задача из пакета", Subject: "Математика", Topics: []string{"Проценты"}, Difficulty: domain.DifficultyHard})
	if err != nil {
		t.Fatal(err)
	}
	mixedPack, err := source.ExportPack(ctx, []domain.TaskBankTaskID{task.ID, newTask.ID})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := target.ImportPack(ctx, mixedPack); err != nil || result.Imported != 1 || result.Conflicts != 1 || result.Duplicates != 0 {
		t.Fatalf("mixed import: %+v %v", result, err)
	}
	before, err = target.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(before.Tasks) != 3 || before.Tasks[1].Content != "Изменено локально" || before.Tasks[2].Content != "Новая задача из пакета" {
		t.Fatalf("mixed import lost tasks: %+v %v", before, err)
	}
	if err := os.Remove(pack); err != nil {
		t.Fatal(err)
	}
	if err := targetStore.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	target = app.NewTaskBankService(reopened)
	if after, err := target.Browse(ctx, domain.TaskBankFilter{}); err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("import/edit did not persist: %+v %v", after, err)
	}
	if _, original, err := target.Attachment(ctx, imported.ID, files[0].ID); err != nil || string(original) != string(data) {
		t.Fatalf("original lost after restart: %v", err)
	}
}

func TestTaskPackRejectsUnreadableTruncatedAndOversizedArchives(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "invalid-zip.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bank := app.NewTaskBankService(store).WithTaskPackReader(taskpacks.NewZIPReader())
	if _, err := bank.ImportPack(ctx, ""); !errors.Is(err, app.ErrTaskPackPathRequired) {
		t.Fatalf("empty path: %v", err)
	}
	if _, err := bank.ImportPack(ctx, filepath.Join(t.TempDir(), "absent.zip")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	if _, err := bank.ImportPack(ctx, t.TempDir()); !errors.Is(err, app.ErrInvalidTaskPack) {
		t.Fatalf("directory accepted: %v", err)
	}
	manifest, assets := importFixture()
	filename := writeImportFixture(t, manifest, assets, false)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := bank.ImportPack(canceled, filename); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if err := os.Truncate(filename, 15); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.ImportPack(ctx, filename); !errors.Is(err, app.ErrInvalidTaskPack) {
		t.Fatalf("truncated ZIP accepted: %v", err)
	}
	for _, test := range []struct {
		name string
		mode os.FileMode
		size uint64
	}{
		{"manifest.json", 0600, 513 << 20},
		{"manifest.json", 0600, 17 << 20},
		{"assets/link", os.ModeSymlink | 0600, 0},
	} {
		file, err := os.CreateTemp(t.TempDir(), "bad-*.zip")
		if err != nil {
			t.Fatal(err)
		}
		archive := zip.NewWriter(file)
		header := &zip.FileHeader{Name: test.name, Method: zip.Store, UncompressedSize64: test.size}
		header.SetMode(test.mode)
		if _, err := archive.CreateRaw(header); err != nil {
			t.Fatal(err)
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := bank.ImportPack(ctx, file.Name()); !errors.Is(err, app.ErrInvalidTaskPack) {
			t.Fatalf("bad ZIP entry accepted: %+v %v", test, err)
		}
	}
	page, err := bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks) != 0 || len(page.Collections) != 0 {
		t.Fatalf("failed reads changed bank: %+v %v", page, err)
	}
}
