package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func revisionFixture(t *testing.T) (*Store, domain.RouteID, domain.RouteTopicID) {
	t.Helper()
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "revision.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	templates := app.NewTemplateService(store)
	template, err := templates.Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := templates.AddTopic(ctx, template.ID, "Циклы"); err != nil {
		t.Fatal(err)
	}
	route, err := app.NewRouteService(store).Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	topics, err := store.ListRouteTopics(ctx, route.ID)
	if err != nil {
		t.Fatal(err)
	}
	return store, route.ID, topics[0].ID
}

func TestRevisionSemanticsAndHistory(t *testing.T) {
	store, route, topic := revisionFixture(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	if err := store.SetRouteTopicRevision(ctx, route, topic, true, at); !errors.Is(err, domain.ErrRevisionRequiresMastered) {
		t.Fatalf("unmastered revision: %v", err)
	}
	if err := store.SetRouteTopicRevision(ctx, route+1, topic, true, at); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatalf("foreign route: %v", err)
	}
	if err := store.SetRouteTopicStatus(ctx, route, topic, domain.ProgressMastered, at); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := store.SetRouteTopicRevision(ctx, route, topic, true, at); err != nil {
			t.Fatal(err)
		}
	}
	topics, err := store.ListRouteTopics(ctx, route)
	if err != nil {
		t.Fatal(err)
	}
	if !topics[0].Revision || topics[0].Status != domain.ProgressMastered || app.RouteProgressSummary(topics)[0].Mastered != 1 {
		t.Fatal("revision must preserve mastery")
	}
	if err := store.SetRouteTopicRevision(ctx, route, topic, false, at); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRouteTopicRevision(ctx, route, topic, true, at); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRouteTopicStatus(ctx, route, topic, domain.ProgressNeedsPractice, at); err != nil {
		t.Fatal(err)
	}
	history, err := store.ListRouteTopicProgressHistory(ctx, route, topic)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 5 {
		t.Fatalf("history count (no-op must not append): %d", len(history))
	}
	last := history[4]
	if last.BeforeStatus != domain.ProgressMastered || last.AfterStatus != domain.ProgressNeedsPractice || !last.BeforeRevision || last.AfterRevision || !last.CreatedAt.Equal(at) {
		t.Fatalf("downgrade transition: %+v", last)
	}
	topics, err = store.ListRouteTopics(ctx, route)
	if err != nil {
		t.Fatal(err)
	}
	if topics[0].Revision {
		t.Fatal("downgrade must clear revision")
	}
	if _, err := store.ListRouteTopicProgressHistory(ctx, route+1, topic); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatal("history leaked across routes")
	}
}

func TestProgressAndHistoryRollbackTogether(t *testing.T) {
	store, route, topic := revisionFixture(t)
	ctx := context.Background()
	at := time.Now()
	if err := store.SetRouteTopicStatus(ctx, route, topic, domain.ProgressMastered, at); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRouteTopicRevision(ctx, route, topic, true, at); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_history BEFORE INSERT ON route_progress_history BEGIN SELECT RAISE(ABORT, 'history unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	for _, status := range []domain.ProgressStatus{domain.ProgressLearning, domain.ProgressNotStarted} {
		if err := store.SetRouteTopicStatus(ctx, route, topic, status, at); err == nil {
			t.Fatal("expected history failure")
		}
		topics, err := store.ListRouteTopics(ctx, route)
		if err != nil {
			t.Fatal(err)
		}
		if topics[0].Status != domain.ProgressMastered || !topics[0].Revision {
			t.Fatal("progress escaped rollback")
		}
	}
	history, err := store.ListRouteTopicProgressHistory(ctx, route, topic)
	if err != nil || len(history) != 2 {
		t.Fatalf("history changed on failure: %v, %v", history, err)
	}
}

func TestRevisionMigrationPreservesExistingProgress(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range schemaMigrations[:7] {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations VALUES (?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (id, name, created_at, updated_at) VALUES (1, 'Иван', 1, 1);
	INSERT INTO student_routes (id, student_id, name, description_md, created_at, updated_at) VALUES (1, 1, 'Python', '', 1, 1);
	INSERT INTO route_topics (id, route_id, name, position, notes_md, created_at, updated_at) VALUES (1, 1, 'Циклы', 0, 'Заметка', 1, 1);
	INSERT INTO route_topic_progress (topic_id, status, updated_at) VALUES (1, 'mastered', 1)`); err != nil {
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
	topics, err := store.ListRouteTopics(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 1 || topics[0].Status != domain.ProgressMastered || topics[0].Revision || topics[0].Notes != "Заметка" {
		t.Fatalf("migration lost data: %+v", topics)
	}
	history, err := store.ListRouteTopicProgressHistory(ctx, 1, 1)
	if err != nil || len(history) != 0 {
		t.Fatalf("migration invented history: %v, %v", history, err)
	}
	if err := store.SetRouteTopicRevision(ctx, 1, 1, true, time.Now()); err != nil {
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
	topics, err = reopened.ListRouteTopics(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !topics[0].Revision || topics[0].Status != domain.ProgressMastered {
		t.Fatal("revision lost after reopening")
	}
	history, err = reopened.ListRouteTopicProgressHistory(ctx, 1, 1)
	if err != nil || len(history) != 1 || !history[0].AfterRevision {
		t.Fatalf("history lost after reopening: %v, %v", history, err)
	}
}
