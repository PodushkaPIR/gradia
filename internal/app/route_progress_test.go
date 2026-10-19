package app

import (
	"context"
	"errors"
	"testing"

	"github.com/shesh/gradia/internal/domain"
)

func TestRouteProgressCountsOnlyLeaves(t *testing.T) {
	parent := domain.RouteTopicID(1)
	nested := domain.RouteTopicID(2)
	topics := []domain.RouteTopic{
		{ID: 1, Status: domain.ProgressMastered},
		{ID: 2, ParentID: &parent, Status: domain.ProgressMastered},
		{ID: 3, ParentID: &nested, Status: domain.ProgressLearning},
		{ID: 4, ParentID: &parent, Status: domain.ProgressMastered},
		{ID: 5, Status: domain.ProgressNeedsPractice},
		{ID: 6},
	}
	summaries := RouteProgressSummary(topics)
	want := domain.ProgressSummary{NotStarted: 1, Learning: 1, NeedsPractice: 1, Mastered: 1}
	if summaries[0] != want || summaries[0].Total() != 4 || summaries[1].Total() != 2 || summaries[1].Mastered != 1 || summaries[2].Learning != 1 {
		t.Fatalf("неверные сводки: %+v", summaries)
	}
	if RouteProgressSummary(nil)[0].Total() != 0 {
		t.Fatal("пустой маршрут должен иметь нулевую сводку")
	}
}

func TestRouteProgressExcludesBranchesWithoutTurningParentsIntoLeaves(t *testing.T) {
	parent, nested := domain.RouteTopicID(1), domain.RouteTopicID(2)
	topics := []domain.RouteTopic{
		{ID: 1, Status: domain.ProgressMastered},
		{ID: 2, ParentID: &parent, Excluded: true},
		{ID: 3, ParentID: &nested, Status: domain.ProgressMastered, Revision: true},
		{ID: 4, ParentID: &parent, Status: domain.ProgressLearning, Excluded: true},
		{ID: 5, Status: domain.ProgressNeedsPractice},
	}
	summary := RouteProgressSummary(topics)
	if summary[0].Total() != 1 || summary[0].NeedsPractice != 1 || summary[1].Total() != 0 || summary[3].Total() != 0 {
		t.Fatalf("excluded descendants counted or parent became leaf: %+v", summary)
	}
	topics[1].Excluded = false
	summary = RouteProgressSummary(topics)
	if summary[0].Total() != 2 || summary[0].Mastered != 1 || summary[1].Mastered != 1 {
		t.Fatalf("restored branch missing: %+v", summary)
	}
}

func TestSetStatusRejectsParentAndUnknownTopic(t *testing.T) {
	parent := domain.RouteTopicID(1)
	repo := &routeRepositoryStub{topics: []domain.RouteTopic{{ID: 1, RouteID: 10}, {ID: 2, RouteID: 10, ParentID: &parent}}}
	service := NewRouteService(repo)
	if err := service.SetStatus(context.Background(), 10, 1, domain.ProgressLearning); !errors.Is(err, domain.ErrTopicHasSubtopics) {
		t.Fatal(err)
	}
	if err := service.SetStatus(context.Background(), 10, 99, domain.ProgressLearning); !errors.Is(err, domain.ErrRouteTopicNotFound) {
		t.Fatal(err)
	}
	if err := service.SetStatus(context.Background(), 10, 2, "invalid"); !errors.Is(err, domain.ErrInvalidProgressStatus) {
		t.Fatal(err)
	}
	if repo.writes != 0 {
		t.Fatal("невалидное изменение передано в хранилище")
	}
	if err := service.SetStatus(context.Background(), 10, 2, domain.ProgressMastered); err != nil || repo.writes != 1 {
		t.Fatalf("err=%v, writes=%d", err, repo.writes)
	}
	want := errors.New("storage failed")
	repo.err = want
	if err := service.SetStatus(context.Background(), 10, 2, domain.ProgressLearning); !errors.Is(err, want) {
		t.Fatal("потеряна ошибка хранилища")
	}
}
