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

func TestExclusionMigrationPersistenceAndBranchRestoration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "version8.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	for i, migration := range schemaMigrations[:8] {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations VALUES (?)`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO students (id, name, created_at, updated_at) VALUES (1, 'Иван', 1, 1);
	INSERT INTO student_routes (id, student_id, name, description_md, created_at, updated_at) VALUES (1, 1, 'Python', '', 1, 1);
	INSERT INTO route_topics (id, route_id, parent_id, name, position, notes_md, created_at, updated_at) VALUES
	(1, 1, NULL, 'Циклы', 0, 'Заметка раздела', 1, 1),
	(2, 1, 1, 'for', 0, 'Заметка темы', 1, 1),
	(3, 1, 1, 'while', 1, '', 1, 1);
	INSERT INTO route_topic_progress (topic_id, status, revision, updated_at) VALUES (2, 'mastered', 1, 1);
	INSERT INTO route_progress_history (topic_id, before_status, after_status, before_revision, after_revision, created_at) VALUES (2, 'not_started', 'mastered', 0, 1, 1)`); err != nil {
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
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	service := app.NewRouteService(store)
	topics, err := service.Topics(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range topics {
		if topic.Excluded {
			t.Fatal("migration excluded an existing topic")
		}
	}
	history, err := service.ExclusionHistory(ctx, 1, 1)
	if err != nil || len(history) != 0 {
		t.Fatalf("migration invented history: %v %v", history, err)
	}
	// Отдельно исключённая тема не должна восстановиться вместе с родителем.
	for _, topic := range []domain.RouteTopicID{2, 1, 1} {
		if err := store.SetRouteTopicExcluded(ctx, 1, topic, true, at); err != nil {
			t.Fatal(err)
		}
	}
	topics, err = service.Topics(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if app.RouteProgressSummary(topics)[0].Total() != 0 {
		t.Fatal("excluded branch counted")
	}
	if err := store.SetRouteTopicExcluded(ctx, 1, 1, false, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	topics, err = service.Topics(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !topics[1].Excluded || topics[2].Excluded || app.RouteProgressSummary(topics)[0].Total() != 1 {
		t.Fatalf("branch restoration lost individual exclusions: %+v", topics)
	}
	if topics[1].Status != domain.ProgressMastered || !topics[1].Revision || topics[1].Notes != "Заметка темы" || topics[0].Notes != "Заметка раздела" {
		t.Fatal("exclusion lost learning data")
	}
	progressHistory, err := service.ProgressHistory(ctx, 1, 2)
	if err != nil || len(progressHistory) != 1 {
		t.Fatalf("progress history changed: %v %v", progressHistory, err)
	}
	history, err = service.ExclusionHistory(ctx, 1, 1)
	if err != nil || len(history) != 2 {
		t.Fatalf("no-op appended history: %v %v", history, err)
	}
	if history[0].BeforeExcluded || !history[0].AfterExcluded || !history[0].CreatedAt.Equal(at) || !history[1].BeforeExcluded || history[1].AfterExcluded {
		t.Fatalf("incorrect transitions: %+v", history)
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
	if topics[0].Excluded || !topics[1].Excluded || topics[2].Excluded || !topics[1].Revision {
		t.Fatal("exclusions lost after reopening")
	}
	if err := reopened.SetRouteTopicExcluded(ctx, 1, 2, false, at); err != nil {
		t.Fatal(err)
	}
	topics, err = reopened.ListRouteTopics(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if summary := app.RouteProgressSummary(topics)[0]; summary.Total() != 2 || summary.Mastered != 1 {
		t.Fatalf("restoration did not recover progress: %+v", summary)
	}
	history, err = reopened.ListRouteTopicExclusionHistory(ctx, 1, 1)
	if err != nil || len(history) != 2 {
		t.Fatalf("history lost after reopening: %v %v", history, err)
	}
}

func TestExclusionHistoryAtomicityAndIsolation(t *testing.T) {
	store, route, topic := revisionFixture(t)
	ctx := context.Background()
	at := time.Now()
	if err := store.SetRouteTopicExcluded(ctx, route+1, topic, true, at); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatalf("foreign route accepted: %v", err)
	}
	if _, err := store.ListRouteTopicExclusionHistory(ctx, route+1, topic); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatalf("foreign history exposed: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_exclusion_history BEFORE INSERT ON route_topic_exclusion_history BEGIN SELECT RAISE(ABORT, 'history unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRouteTopicExcluded(ctx, route, topic, true, at); err == nil {
		t.Fatal("expected history failure")
	}
	topics, err := store.ListRouteTopics(ctx, route)
	if err != nil {
		t.Fatal(err)
	}
	if topics[0].Excluded {
		t.Fatal("exclusion escaped rollback")
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER reject_exclusion_history`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRouteTopicExcluded(ctx, route, topic, true, at); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_exclusion_history BEFORE INSERT ON route_topic_exclusion_history BEGIN SELECT RAISE(ABORT, 'history unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetRouteTopicExcluded(ctx, route, topic, false, at); err == nil {
		t.Fatal("expected restoration failure")
	}
	topics, err = store.ListRouteTopics(ctx, route)
	if err != nil {
		t.Fatal(err)
	}
	if !topics[0].Excluded {
		t.Fatal("restoration escaped rollback")
	}
	history, err := store.ListRouteTopicExclusionHistory(ctx, route, topic)
	if err != nil || len(history) != 1 {
		t.Fatalf("failed writes changed history: %v %v", history, err)
	}
}
