package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

var _ app.RouteRepository = (*Store)(nil)

func TestRouteSnapshotIsIndependentAndPersistent(t *testing.T) {
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
	students, templates, routes := app.NewStudentService(store), app.NewTemplateService(store), app.NewRouteService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	other, err := students.Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := domain.NewTemplate("Python'); DROP TABLE students; --")
	if err != nil {
		t.Fatal(err)
	}
	draft.Description = "# Программа\n\n- Циклы\n- Строки"
	template, err := store.CreateTemplate(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := templates.AddTopic(ctx, template.ID, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	first := parent
	for depth := 2; depth <= domain.TopicMaxDepth; depth++ {
		parent, err = templates.AddSubtopic(ctx, template.ID, parent.ID, "Подтема")
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := templates.AddTopic(ctx, template.ID, "Строки"); err != nil {
		t.Fatal(err)
	}
	route, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if route.Name != template.Name || route.Description != template.Description {
		t.Fatal("маршрут не скопировал данные шаблона")
	}
	before, err := routes.Topics(ctx, route.ID)
	if err != nil || len(before) != 6 {
		t.Fatalf("topics=%+v, err=%v", before, err)
	}
	byID := make(map[domain.RouteTopicID]domain.RouteTopic)
	for _, topic := range before {
		byID[topic.ID] = topic
	}
	var deepest int
	for _, topic := range before {
		depth := 1
		for topic.ParentID != nil {
			topic = byID[*topic.ParentID]
			depth++
		}
		deepest = max(deepest, depth)
	}
	if deepest != 5 {
		t.Fatalf("глубина копии=%d", deepest)
	}
	if _, err := templates.Rename(ctx, template.ID, "Новый шаблон"); err != nil {
		t.Fatal(err)
	}
	if _, err := templates.RenameTopic(ctx, template.ID, first.ID, "Новая тема"); err != nil {
		t.Fatal(err)
	}
	if _, err := templates.AddTopic(ctx, template.ID, "Дополнение"); err != nil {
		t.Fatal(err)
	}
	if _, err := routes.Create(ctx, student.ID, template.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := routes.Create(ctx, other.ID, template.ID); err != nil {
		t.Fatal(err)
	}
	if err := students.Delete(ctx, student.ID); !errors.Is(err, domain.ErrStudentInUse) {
		t.Fatalf("маршруты не должны удаляться вместе с учеником: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.ListStudentRoutes(ctx, student.ID)
	if err != nil || len(list) != 2 || !reflect.DeepEqual(list[0], route) {
		t.Fatalf("маршруты изменились: %+v, %v", list, err)
	}
	after, err := store.ListRouteTopics(ctx, route.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("шаблон изменил персональную копию: %+v, %v", after, err)
	}
	otherRoutes, err := store.ListStudentRoutes(ctx, other.ID)
	if err != nil || len(otherRoutes) != 1 {
		t.Fatalf("маршруты учеников смешались: %+v, %v", otherRoutes, err)
	}
}

func TestRouteCreationRollsBack(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	template, err := app.NewTemplateService(store).Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.NewTemplateService(store).AddTopic(ctx, template.ID, "Циклы"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_copy BEFORE INSERT ON route_topics BEGIN SELECT RAISE(ABORT, 'test failure'); END;`); err != nil {
		t.Fatal(err)
	}
	service := app.NewRouteService(store)
	if _, err := service.Create(ctx, student.ID, template.ID); err == nil {
		t.Fatal("ожидалась ошибка копирования")
	}
	list, err := service.List(ctx, student.ID)
	if err != nil || len(list) != 0 {
		t.Fatalf("сохранён частичный маршрут: %+v, %v", list, err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM route_topics`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("сохранены частичные темы: %d, %v", count, err)
	}
	if _, err := service.Create(ctx, 999, template.ID); !errors.Is(err, domain.ErrStudentNotFound) {
		t.Fatalf("ожидался student not found: %v", err)
	}
	if _, err := service.Create(ctx, student.ID, 999); !errors.Is(err, domain.ErrTemplateNotFound) {
		t.Fatalf("ожидался template not found: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER reject_copy`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, student.ID, template.ID); err != nil {
		t.Fatalf("после rollback создание должно работать: %v", err)
	}
}
