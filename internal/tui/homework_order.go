package tui

import (
	"cmp"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/domain"
)

type homeworkTasksMovedMsg struct {
	homework  domain.HomeworkID
	selected  domain.HomeworkTaskID
	positions []domain.HomeworkTaskPosition
}

func (m homeworkModel) moveTask(forward bool) (homeworkModel, tea.Cmd) {
	if m.selected == nil || m.cursor < 0 || m.cursor >= len(m.tasks) {
		return m, nil
	}
	neighbor := m.cursor - 1
	if forward {
		neighbor = m.cursor + 1
	}
	if neighbor < 0 || neighbor >= len(m.tasks) {
		return m, nil
	}
	first, second := m.tasks[m.cursor].ID, m.tasks[neighbor].ID
	m.saving, m.saveErr = true, nil
	ctx, service, student, homework := m.ctx, m.service, m.student, m.selected.ID
	details, screen := m.detailsID, m.screenID
	return m, func() tea.Msg {
		positions, err := service.SwapTasks(ctx, student, homework, first, second)
		if err != nil {
			return routeToHomework(details, screen, homeworkTaskFailedMsg{err: err})
		}
		return routeToHomework(details, screen, homeworkTasksMovedMsg{
			homework:  homework,
			selected:  first,
			positions: positions,
		})
	}
}

func (m homeworkModel) applyTaskOrder(msg homeworkTasksMovedMsg) homeworkModel {
	if m.selected == nil || msg.homework != m.selected.ID {
		return m
	}
	m.saving, m.saveErr = false, nil
	m.tasks = slices.Clone(m.tasks)
	for i := range m.tasks {
		for _, position := range msg.positions {
			if m.tasks[i].ID == position.TaskID {
				m.tasks[i].Position = position.Position
			}
		}
	}
	slices.SortFunc(m.tasks, func(a, b domain.HomeworkTask) int {
		return cmp.Compare(a.Position, b.Position)
	})
	m.cursor = max(0, slices.IndexFunc(m.tasks, func(task domain.HomeworkTask) bool {
		return task.ID == msg.selected
	}))
	return m
}
