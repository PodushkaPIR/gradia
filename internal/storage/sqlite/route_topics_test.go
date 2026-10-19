package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestPersonalPlanTopicsCanBeEditedWithoutLosingProgress(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	templates := app.NewTemplateService(store)
	template, err := templates.Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	root, err := templates.AddTopic(ctx, template.ID, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	routes := app.NewRouteService(store)
	firstPlan, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	secondPlan, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	firstTopics, err := routes.Topics(ctx, firstPlan.ID)
	if err != nil || len(firstTopics) != 1 {
		t.Fatalf("topics=%+v, err=%v", firstTopics, err)
	}
	parent := firstTopics[0]

	if err := routes.SetStatus(ctx, firstPlan.ID, parent.ID, domain.ProgressMastered); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetRevision(ctx, firstPlan.ID, parent.ID, true); err != nil {
		t.Fatal(err)
	}
	const note = "Сохранить заметку у родителя"
	if err := routes.SetNote(ctx, firstPlan.ID, parent.ID, note); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetExcluded(ctx, firstPlan.ID, parent.ID, true); err != nil {
		t.Fatal(err)
	}

	child, err := routes.AddTopic(ctx, firstPlan.ID, parent.ID, "Цикл for")
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentID == nil || *child.ParentID != parent.ID || child.Position != 0 || child.Status != domain.ProgressNotStarted || child.Revision || child.Excluded || child.Notes != "" {
		t.Fatalf("new child must be a clean terminal topic: %+v", child)
	}

	sibling, err := routes.AddTopic(ctx, firstPlan.ID, parent.ID, "Цикл while")
	if err != nil {
		t.Fatal(err)
	}
	if sibling.Position != 1 {
		t.Fatalf("second child position=%d, want=1", sibling.Position)
	}
	if err := routes.RenameTopic(ctx, firstPlan.ID, child.ID, "Цикл for обновлённый"); err != nil {
		t.Fatal(err)
	}

	updated, err := routes.Topics(ctx, firstPlan.ID)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[domain.RouteTopicID]domain.RouteTopic, len(updated))
	for _, topic := range updated {
		byID[topic.ID] = topic
	}
	parentAfter := byID[parent.ID]
	if parentAfter.Status != domain.ProgressMastered || !parentAfter.Revision || parentAfter.Notes != note || !parentAfter.Excluded {
		t.Fatalf("turning a terminal topic into a parent lost its learning data: %+v", parentAfter)
	}
	if byID[child.ID].Name != "Цикл for обновлённый" {
		t.Fatalf("rename not persisted: %+v", byID[child.ID])
	}
	if byID[child.ID].Status != domain.ProgressNotStarted {
		t.Fatalf("renaming changed progress: %+v", byID[child.ID])
	}
	progressHistory, err := routes.ProgressHistory(ctx, firstPlan.ID, parent.ID)
	if err != nil || len(progressHistory) != 2 {
		t.Fatalf("parent progress history was lost: %+v, %v", progressHistory, err)
	}
	exclusionHistory, err := routes.ExclusionHistory(ctx, firstPlan.ID, parent.ID)
	if err != nil || len(exclusionHistory) != 1 {
		t.Fatalf("parent exclusion history was lost: %+v, %v", exclusionHistory, err)
	}
	if err := routes.SetExcluded(ctx, firstPlan.ID, parent.ID, false); err != nil {
		t.Fatal(err)
	}
	updated, err = routes.Topics(ctx, firstPlan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary := app.RouteProgressSummary(updated); summary[0].Total() != 2 || summary[0].Mastered != 0 || summary[0].NotStarted != 2 || summary[parent.ID].Total() != 2 {
		t.Fatalf("former parent status must not count after adding children: %+v", summary)
	}
	if _, err := templates.ListTopics(ctx, template.ID); err != nil {
		t.Fatal(err)
	}
	planTopics, err := routes.Topics(ctx, secondPlan.ID)
	if err != nil || len(planTopics) != 1 || planTopics[0].Name != root.Name || planTopics[0].Status != domain.ProgressNotStarted {
		t.Fatalf("editing one plan changed another plan: %+v, %v", planTopics, err)
	}
	sourceTopics, err := templates.ListTopics(ctx, template.ID)
	if err != nil || len(sourceTopics) != 1 || sourceTopics[0].Name != root.Name {
		t.Fatalf("editing a personal plan changed its source template: %+v, %v", sourceTopics, err)
	}

	deepest := child
	for depth := 3; depth <= domain.TopicMaxDepth; depth++ {
		deepest, err = routes.AddTopic(ctx, firstPlan.ID, deepest.ID, "Вложенная тема")
		if err != nil {
			t.Fatalf("could not add level %d: %v", depth, err)
		}
	}
	if _, err := routes.AddTopic(ctx, firstPlan.ID, deepest.ID, "Слишком глубокая тема"); !errors.Is(err, domain.ErrTopicDepthExceeded) {
		t.Fatalf("expected depth limit error, got %v", err)
	}
	tooDeep, err := domain.NewRouteTopic(firstPlan.ID, "Прямой вызов хранилища")
	if err != nil {
		t.Fatal(err)
	}
	tooDeep.ParentID = &deepest.ID
	if _, err := store.CreateRouteTopic(ctx, tooDeep); !errors.Is(err, domain.ErrTopicDepthExceeded) {
		t.Fatalf("storage should enforce the depth limit too, got %v", err)
	}
}
