package tui

import (
	"context"
	"errors"
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type studentDeletedMsg struct {
	id domain.StudentID
}

type studentDeleteFailedMsg struct {
	err error
}

type studentDeleteModel struct {
	ctx      context.Context
	service  *app.StudentService
	student  domain.Student
	deleting bool
	err      error
	keys     confirmationKeyMap
}

func newStudentDelete(ctx context.Context, service *app.StudentService, student domain.Student) studentDeleteModel {
	return studentDeleteModel{ctx: ctx, service: service, student: student, keys: newConfirmationKeyMap()}
}

func deleteStudentCmd(ctx context.Context, service *app.StudentService, id domain.StudentID) tea.Cmd {
	return func() tea.Msg {
		if err := service.Delete(ctx, id); err != nil {
			return routeToStudents(studentDeleteFailedMsg{err: err})
		}
		return routeToStudents(studentDeletedMsg{id: id})
	}
}

func (m studentDeleteModel) Update(msg tea.Msg) (studentDeleteModel, tea.Cmd) {
	if failed, ok := msg.(studentDeleteFailedMsg); ok {
		m.deleting = false
		m.err = failed.err
		return m, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, m.keys.Confirm.Binding) && m.InputMode() == inputConfirmation {
		m.deleting = true
		m.err = nil
		return m, deleteStudentCmd(m.ctx, m.service, m.student.ID)
	}
	return m, nil
}

func (m studentDeleteModel) View() string {
	text := fmt.Sprintf("Удаление ученика\n\nУдалить ученика «%s»?", m.student.Name)
	if m.InputMode() == inputBusy {
		return text + "\n\nУдаление…"
	}
	if errors.Is(m.err, domain.ErrStudentNotFound) {
		return text + "\n\nУченик больше не существует. Нажмите " + m.keys.Cancel.Help().Key + " и обновите список по " + newListKeyMap().Reload.Help().Key + "."
	}
	if errors.Is(m.err, domain.ErrStudentInUse) {
		return text + "\n\n" + m.err.Error()
	}
	if m.err != nil {
		return text + "\n\nНе удалось удалить ученика. Нажмите " + m.keys.Confirm.Help().Key + " для повторной попытки."
	}
	return text
}

func (m studentDeleteModel) InputMode() inputMode {
	if m.deleting {
		return inputBusy
	}
	return inputConfirmation
}
