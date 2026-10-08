package tui

import (
	"context"
	"errors"
	"fmt"

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
}

func newStudentDelete(ctx context.Context, service *app.StudentService, student domain.Student) studentDeleteModel {
	return studentDeleteModel{ctx: ctx, service: service, student: student}
}

func deleteStudentCmd(ctx context.Context, service *app.StudentService, id domain.StudentID) tea.Cmd {
	return func() tea.Msg {
		if err := service.Delete(ctx, id); err != nil {
			return studentDeleteFailedMsg{err: err}
		}
		return studentDeletedMsg{id: id}
	}
}

func (m studentDeleteModel) Update(msg tea.Msg) (studentDeleteModel, tea.Cmd) {
	if failed, ok := msg.(studentDeleteFailedMsg); ok {
		m.deleting = false
		m.err = failed.err
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "y" && !m.deleting {
		m.deleting = true
		m.err = nil
		return m, deleteStudentCmd(m.ctx, m.service, m.student.ID)
	}
	return m, nil
}

func (m studentDeleteModel) View() string {
	text := fmt.Sprintf("Удаление ученика\n\nУдалить ученика «%s»?", m.student.Name)
	if m.deleting {
		return text + "\n\nУдаление…"
	}
	if errors.Is(m.err, domain.ErrStudentNotFound) {
		return text + "\n\nУченик больше не существует. Нажмите Esc и обновите список по r."
	}
	if m.err != nil {
		return text + "\n\nНе удалось удалить ученика. Нажмите y для повторной попытки."
	}
	return text
}
