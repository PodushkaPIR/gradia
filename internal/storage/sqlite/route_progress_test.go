package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestRouteProgressPersistsAndIsIsolated(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if store != nil {
			store.Close()
		}
	})
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	templates := app.NewTemplateService(store)
	template, err := templates.Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := templates.AddTopic(ctx, template.ID, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := templates.AddSubtopic(ctx, template.ID, parent.ID, "Цикл for"); err != nil {
		t.Fatal(err)
	}
	routes := app.NewRouteService(store)
	first, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	topics, err := routes.Topics(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	var leaf, group domain.RouteTopicID
	for _, topic := range topics {
		if topic.ParentID == nil {
			group = topic.ID
		} else {
			leaf = topic.ID
		}
		if topic.Status != domain.ProgressNotStarted {
			t.Fatal("новая копия должна иметь not_started")
		}
	}
	if err := routes.SetStatus(ctx, first.ID, group, domain.ProgressMastered); !errors.Is(err, domain.ErrTopicHasSubtopics) {
		t.Fatalf("нельзя редактировать прогресс раздела: %v", err)
	}
	if err := store.SetRouteTopicStatus(ctx, first.ID, group, domain.ProgressMastered, time.Now()); !errors.Is(err, domain.ErrTopicHasSubtopics) {
		t.Fatal("репозиторий должен проверять конечность темы")
	}
	if err := store.SetRouteTopicStatus(ctx, second.ID, leaf, domain.ProgressMastered, time.Now()); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatal("тема из чужого маршрута должна отклоняться")
	}
	if err := routes.SetStatus(ctx, first.ID, leaf, "'); DROP TABLE route_topics; --"); !errors.Is(err, domain.ErrInvalidProgressStatus) {
		t.Fatal("произвольная строка статуса должна отклоняться")
	}
	for _, status := range []domain.ProgressStatus{domain.ProgressLearning, domain.ProgressNeedsPractice, domain.ProgressMastered} {
		if err := routes.SetStatus(ctx, first.ID, leaf, status); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	topics, err = store.ListRouteTopics(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range topics {
		if topic.ID == leaf && topic.Status != domain.ProgressMastered {
			t.Fatal("статус потерян после переоткрытия")
		}
	}
	other, err := store.ListRouteTopics(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range other {
		if topic.Status != domain.ProgressNotStarted {
			t.Fatal("изменился прогресс другой копии")
		}
	}
	if err := store.SetRouteTopicStatus(ctx, first.ID, leaf, domain.ProgressNotStarted, time.Now()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM route_topic_progress`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("default должен восстанавливаться без записи: count=%d, err=%v", count, err)
	}
	topics, err = store.ListRouteTopics(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range topics {
		if topic.Status != domain.ProgressNotStarted {
			t.Fatal("сброс статуса не сохранился")
		}
	}
}
