package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type studentSavedMsg struct {
	student domain.Student
}

type studentSaveFailedMsg struct {
	err error
}

func saveStudentCmd(ctx context.Context, service *app.StudentService, id domain.StudentID, name string) tea.Cmd {
	return func() tea.Msg {
		var student domain.Student
		var err error
		if id == 0 {
			student, err = service.Create(ctx, name)
		} else {
			student, err = service.Rename(ctx, id, name)
		}
		if err != nil {
			return routeToStudents(studentSaveFailedMsg{err: err})
		}
		return routeToStudents(studentSavedMsg{student: student})
	}
}
