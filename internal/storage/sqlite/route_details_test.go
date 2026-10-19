package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestLearningPlanDetailsPersistIndependentlyOfSourceAndOtherPlans(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "details.db")
	store, err := Open(ctx, path)
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
	routes := app.NewRouteService(store)
	plan, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	topicsBefore, err := routes.Topics(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := routes.Rename(ctx, plan.ID, "  Индивидуальный Python  ")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != plan.ID || renamed.StudentID != student.ID || renamed.Name != "Индивидуальный Python" || !renamed.CreatedAt.Equal(plan.CreatedAt) {
		t.Fatalf("bad rename result: %+v", renamed)
	}
	const description = "# Индивидуальный план\n\n- Практика `for`\n- O'Reilly\n'); DROP TABLE student_routes; --\n"
	updated, err := routes.SetDescription(ctx, plan.ID, description)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != renamed.Name || updated.Description != description {
		t.Fatalf("description overwrote name: %+v", updated)
	}
	if _, err := routes.Rename(ctx, plan.ID, "  "); !errors.Is(err, domain.ErrRouteNameRequired) {
		t.Fatalf("empty name accepted: %v", err)
	}
	if _, err := routes.Rename(ctx, plan.ID, strings.Repeat("я", domain.RouteNameMaxLength+1)); !errors.Is(err, domain.ErrRouteNameTooLong) {
		t.Fatalf("long name accepted: %v", err)
	}
	if _, err := routes.Rename(ctx, 99999, "Missing"); !errors.Is(err, domain.ErrRouteNotFound) {
		t.Fatalf("missing plan rename: %v", err)
	}
	if _, err := routes.SetDescription(ctx, 99999, "Missing"); !errors.Is(err, domain.ErrRouteNotFound) {
		t.Fatalf("missing plan description: %v", err)
	}
	if actual, err := templates.List(ctx); err != nil || len(actual) != 1 || !reflect.DeepEqual(actual[0], template) {
		t.Fatalf("source changed: %+v %v", actual, err)
	}
	if actual, err := routes.List(ctx, student.ID); err != nil || len(actual) != 2 || !reflect.DeepEqual(actual[1], other) {
		t.Fatalf("other plan changed: %+v %v", actual, err)
	}
	if actual, err := routes.Topics(ctx, plan.ID); err != nil || !reflect.DeepEqual(actual, topicsBefore) {
		t.Fatalf("topics changed: %+v %v", actual, err)
	}
	if _, err := templates.Rename(ctx, template.ID, "Новый шаблон"); err != nil {
		t.Fatal(err)
	}
	// Source removal is a fixture operation: there is no template-deletion UI yet.
	if _, err := store.db.ExecContext(ctx, `DELETE FROM template_topics WHERE template_id = ?`, template.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, template.ID); err != nil {
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
	routes = app.NewRouteService(reopened)
	plans, err := routes.List(ctx, student.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Name != renamed.Name || plans[0].Description != description || plans[0].SourceTemplateID != nil {
		t.Fatalf("details lost after source removal/reopen: %+v", plans[0])
	}
	if actual, err := routes.Topics(ctx, plan.ID); err != nil || !reflect.DeepEqual(actual, topicsBefore) {
		t.Fatalf("source removal lost personal topics: %+v %v", actual, err)
	}
	if _, err := routes.Rename(ctx, plan.ID, "После удаления шаблона"); err != nil {
		t.Fatal(err)
	}
	cleared, err := routes.SetDescription(ctx, plan.ID, "")
	if err != nil || cleared.Description != "" || cleared.Name != "После удаления шаблона" {
		t.Fatalf("cannot edit source-less plan: %+v %v", cleared, err)
	}
}
