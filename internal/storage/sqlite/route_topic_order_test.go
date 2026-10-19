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

func TestLearningPlanTopicReorderingPersistsAndPreservesLearningData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "order.db")
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
	routes := app.NewRouteService(store)
	plan, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	group, err := routes.AddTopic(ctx, plan.ID, 0, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	first, err := routes.AddTopic(ctx, plan.ID, group.ID, "for")
	if err != nil {
		t.Fatal(err)
	}
	second, err := routes.AddTopic(ctx, plan.ID, group.ID, "while")
	if err != nil {
		t.Fatal(err)
	}
	otherRoot, err := routes.AddTopic(ctx, plan.ID, 0, "Функции")
	if err != nil {
		t.Fatal(err)
	}
	if err := routes.SetStatus(ctx, plan.ID, first.ID, domain.ProgressMastered); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetRevision(ctx, plan.ID, first.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetNote(ctx, plan.ID, first.ID, "Заметка"); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetExcluded(ctx, plan.ID, first.ID, true); err != nil {
		t.Fatal(err)
	}
	before, err := routes.Topics(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	progressHistory, err := routes.ProgressHistory(ctx, plan.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	exclusionHistory, err := routes.ExclusionHistory(ctx, plan.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := routes.SwapTopics(ctx, plan.ID, first.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes[0].TopicID != first.ID || changes[0].Position != 1 || changes[1].TopicID != second.ID || changes[1].Position != 0 {
		t.Fatalf("incorrect persisted positions: %+v", changes)
	}
	if _, err := routes.SwapTopics(ctx, plan.ID, group.ID, otherRoot.ID); err != nil {
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
	after, err := routes.Topics(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []domain.RouteTopicID{otherRoot.ID, group.ID, second.ID, first.ID}
	var actualOrder []domain.RouteTopicID
	for _, topic := range after {
		actualOrder = append(actualOrder, topic.ID)
	}
	if !reflect.DeepEqual(actualOrder, wantOrder) {
		t.Fatalf("order=%v, want=%v", actualOrder, wantOrder)
	}
	byID := make(map[domain.RouteTopicID]domain.RouteTopic)
	for _, topic := range after {
		byID[topic.ID] = topic
	}
	for _, topic := range before {
		actual := byID[topic.ID]
		actual.Position, actual.UpdatedAt = topic.Position, topic.UpdatedAt
		if !reflect.DeepEqual(actual, topic) {
			t.Fatalf("reordering changed topic data: got=%+v want=%+v", actual, topic)
		}
	}
	if actual, err := routes.ProgressHistory(ctx, plan.ID, first.ID); err != nil || !reflect.DeepEqual(actual, progressHistory) {
		t.Fatalf("progress history changed: %v, %v", actual, err)
	}
	if actual, err := routes.ExclusionHistory(ctx, plan.ID, first.ID); err != nil || !reflect.DeepEqual(actual, exclusionHistory) {
		t.Fatalf("exclusion history changed: %v, %v", actual, err)
	}
	if summary := app.RouteProgressSummary(after)[0]; summary.Total() != 2 || summary.NotStarted != 2 {
		t.Fatalf("summary changed: %+v", summary)
	}
}

func TestLearningPlanReorderingRejectsDifferentParentsAndForeignPlans(t *testing.T) {
	store, plan, root := revisionFixture(t)
	ctx := context.Background()
	routes := app.NewRouteService(store)
	child, err := routes.AddTopic(ctx, plan, root, "Подтема")
	if err != nil {
		t.Fatal(err)
	}
	before, err := routes.Topics(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	student, err := app.NewStudentService(store).Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	templates := app.NewTemplateService(store)
	template, err := templates.Create(ctx, "Другой шаблон")
	if err != nil {
		t.Fatal(err)
	}
	otherPlan, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := routes.AddTopic(ctx, otherPlan.ID, 0, "Чужая тема")
	if err != nil {
		t.Fatal(err)
	}
	otherBefore, err := routes.Topics(ctx, otherPlan.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name          string
		plan          domain.RouteID
		first, second domain.RouteTopicID
		want          error
	}{
		{"different parents", plan, root, child.ID, domain.ErrTopicsNotSiblings},
		{"foreign second", plan, root, foreign.ID, domain.ErrRouteTopicNotFound},
		{"foreign first", otherPlan.ID, root, foreign.ID, domain.ErrRouteTopicNotFound},
		{"missing topic", plan, root, 99999, domain.ErrRouteTopicNotFound},
		{"invalid plan", 0, root, child.ID, domain.ErrRouteNotFound},
		{"invalid topic", plan, 0, child.ID, domain.ErrRouteTopicNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := routes.SwapTopics(ctx, test.plan, test.first, test.second); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
	if _, err := routes.SwapTopics(ctx, plan, root, root); err != nil {
		t.Fatal(err)
	}
	if actual, err := routes.Topics(ctx, plan); err != nil || !reflect.DeepEqual(actual, before) {
		t.Fatalf("invalid/no-op reorder changed plan: %+v %v", actual, err)
	}
	if actual, err := routes.Topics(ctx, otherPlan.ID); err != nil || !reflect.DeepEqual(actual, otherBefore) {
		t.Fatalf("foreign plan changed: %+v %v", actual, err)
	}
}

func TestLearningPlanReorderingRollsBackBothPositionsOnFailure(t *testing.T) {
	store, plan, root := revisionFixture(t)
	ctx := context.Background()
	routes := app.NewRouteService(store)
	other, err := routes.AddTopic(ctx, plan, 0, "Other")
	if err != nil {
		t.Fatal(err)
	}
	before, err := routes.Topics(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_second_order_update BEFORE UPDATE OF position ON route_topics WHEN OLD.name = 'Other' BEGIN SELECT RAISE(ABORT, 'write failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := routes.SwapTopics(ctx, plan, root, other.ID); err == nil {
		t.Fatal("expected write failure")
	}
	if actual, err := routes.Topics(ctx, plan); err != nil || !reflect.DeepEqual(actual, before) {
		t.Fatalf("failed reorder changed plan: %+v %v", actual, err)
	}
}
