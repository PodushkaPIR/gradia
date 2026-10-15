package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestRouteNotesPersistIndependentlyOfProgress(t *testing.T) {
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
	var group, leaf domain.RouteTopicID
	for _, topic := range topics {
		if topic.ParentID == nil {
			group = topic.ID
		} else {
			leaf = topic.ID
		}
	}
	note := "# Ошибки\n\n- Повторить `for`\n- O'Reilly\n'); DROP TABLE route_topics; --\n"
	if err := routes.SetNote(ctx, first.ID, leaf, note); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetNote(ctx, first.ID, group, "Заметка по разделу"); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetStatus(ctx, first.ID, leaf, domain.ProgressMastered); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetStatus(ctx, first.ID, leaf, domain.ProgressNotStarted); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetNote(ctx, second.ID, leaf, "Чужая запись"); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatalf("запись в чужой маршрут не должна допускаться: %v", err)
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
		if topic.ID == leaf && (topic.Notes != note || topic.Status != domain.ProgressNotStarted) {
			t.Fatalf("текст или прогресс изменились: %+v", topic)
		}
		if topic.ID == group && topic.Notes != "Заметка по разделу" {
			t.Fatal("не сохранилась заметка родительской темы")
		}
	}
	other, err := store.ListRouteTopics(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range other {
		if topic.Notes != "" {
			t.Fatal("заметка попала в другую копию")
		}
	}
	if err := app.NewRouteService(store).SetNote(ctx, first.ID, leaf, ""); err != nil {
		t.Fatal(err)
	}
	topics, err = store.ListRouteTopics(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range topics {
		if topic.ID == leaf && topic.Notes != "" {
			t.Fatal("очистка заметки не сохранилась")
		}
	}
}
