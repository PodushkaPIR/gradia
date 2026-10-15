package tui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type progressRepoStub struct {
	routeRepoStub
	topics   []domain.RouteTopic
	writes   int
	writeErr error
}

func (r *progressRepoStub) ListRouteTopics(context.Context, domain.RouteID) ([]domain.RouteTopic, error) {
	return slices.Clone(r.topics), nil
}
func (r *progressRepoStub) SetRouteTopicStatus(_ context.Context, route domain.RouteID, id domain.RouteTopicID, status domain.ProgressStatus, at time.Time) error {
	r.writes++
	if r.writeErr != nil {
		return r.writeErr
	}
	for i := range r.topics {
		if r.topics[i].ID == id {
			r.topics[i].Status = status
			return nil
		}
	}
	return domain.ErrRouteTopicNotFound
}

func TestProgressKeysAndErrors(t *testing.T) {
	parent := domain.RouteTopicID(1)
	repo := &progressRepoStub{topics: []domain.RouteTopic{{ID: 1, RouteID: 10, Name: "Циклы"}, {ID: 2, RouteID: 10, ParentID: &parent, Name: "Цикл for"}}}
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}), nil, app.NewRouteService(repo))
	repo.routes = []domain.Route{{ID: 10, StudentID: 1, Name: "Python"}}
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, load = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	space := tea.KeyPressMsg{Code: ' ', Text: " "}
	model, cmd := updateApp(model, space)
	if cmd != nil || repo.writes != 0 {
		t.Fatal("Space на разделе не должен менять прогресс")
	}
	model, _ = updateApp(model, keypress('j'))
	model, cmd = updateApp(model, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if cmd != nil {
		t.Fatal("not_started не должен переходить назад")
	}
	repo.writeErr = errors.New("write failed")
	model, save := updateApp(model, space)
	if save == nil || repo.writes != 0 || model.screenInputMode() != inputBusy || model.students.details.view.rows[1].topic.Status.Effective() != domain.ProgressNotStarted {
		t.Fatal("запись должна выполняться через Cmd без оптимистичного изменения статуса")
	}
	model, duplicate := updateApp(model, space)
	if duplicate != nil {
		t.Fatal("запущена повторная запись")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.details.view == nil {
		t.Fatal("нельзя уходить во время записи прогресса")
	}
	model, _ = updateApp(model, save())
	if model.screenInputMode() != inputNavigation || !strings.Contains(model.View().Content, "Не удалось сохранить прогресс") || model.students.details.view.rows[1].topic.Status.Effective() != domain.ProgressNotStarted {
		t.Fatal("ошибка должна сохранять прежнее состояние")
	}
	repo.writeErr = nil
	for _, status := range []domain.ProgressStatus{domain.ProgressLearning, domain.ProgressNeedsPractice, domain.ProgressMastered} {
		model, save = updateApp(model, space)
		model, _ = updateApp(model, save())
		if model.students.details.view.rows[1].topic.Status != status {
			t.Fatalf("неверный статус после Space: %v", status)
		}
	}
	if !strings.Contains(model.View().Content, "1/1 освоено") || !strings.Contains(model.View().Content, "Освоено 1/1 тем") {
		t.Fatal("сводка должна обновляться после изменения конечной темы")
	}
	model, cmd = updateApp(model, space)
	if cmd != nil {
		t.Fatal("mastered не должен сбрасываться по следующему Space")
	}
	model, save = updateApp(model, tea.KeyPressMsg{Code: tea.KeyBackspace})
	model, _ = updateApp(model, save())
	if model.students.details.view.rows[1].topic.Status != domain.ProgressNeedsPractice {
		t.Fatal("Backspace должен возвращать предыдущий статус")
	}
	model, _ = updateApp(model, keypress('?'))
	if !strings.Contains(model.View().Content, "Backspace") {
		t.Fatal("обратное изменение должно присутствовать в полной помощи")
	}
}
