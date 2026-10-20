package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type identityPackWriter struct{ source app.TaskPackSource }

func (w *identityPackWriter) Write(_ context.Context, source app.TaskPackSource, _ func(context.Context, domain.TaskBankAttachment) ([]byte, error)) (string, error) {
	w.source = source
	return "identity-test.zip", nil
}

func TestTaskPackIdentityMigrationPreservesLegacyBankAndStableKeys(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy14.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range schemaMigrations[:14] {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations VALUES (?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	legacy := app.NewTaskBankService(&Store{db: db})
	input := domain.TaskBankInput{Content: "Существующая задача", Subject: "Информатика", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyMedium}
	task, err := legacy.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	before, err := legacy.Browse(ctx, domain.TaskBankFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	writer := &identityPackWriter{}
	bank := app.NewTaskBankService(store).WithTaskPackWriter(writer)
	if _, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{task.ID}); err != nil {
		t.Fatal(err)
	}
	key := writer.source.Tasks[0].Key
	if len(key) != 32 {
		t.Fatalf("legacy task has no portable key: %q", key)
	}
	if actual, err := bank.Browse(ctx, domain.TaskBankFilter{}); err != nil || !reflect.DeepEqual(actual, before) {
		t.Fatalf("identity backfill changed legacy task: %+v %v", actual, err)
	}
	created, err := bank.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bank.ExportPack(ctx, []domain.TaskBankTaskID{task.ID, created.ID}); err != nil {
		t.Fatal(err)
	}
	if writer.source.Tasks[0].Key != key || writer.source.Tasks[1].Key == key || len(writer.source.Tasks[1].Key) != 32 {
		t.Fatal("new task key missing or identity collision")
	}
}
