package tui

import (
	"context"
	"errors"

	"charm.land/bubbles/v2/textinput"
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

type studentFormModel struct {
	ctx       context.Context
	service   *app.StudentService
	name      textinput.Model
	studentID domain.StudentID
	saving    bool
	err       error
}

func newStudentForm(ctx context.Context, service *app.StudentService, width int) studentFormModel {
	name := textinput.New()
	name.Placeholder = "Имя ученика"
	name.CharLimit = domain.StudentNameMaxLength
	name.SetWidth(max(1, min(40, width-4)))
	return studentFormModel{ctx: ctx, service: service, name: name}
}

func newStudentEditForm(ctx context.Context, service *app.StudentService, width int, student domain.Student) studentFormModel {
	form := newStudentForm(ctx, service, width)
	form.studentID = student.ID
	form.name.SetValue(student.Name)
	form.name.CursorEnd()
	return form
}

func (m *studentFormModel) Init() tea.Cmd {
	return m.name.Focus()
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
			return studentSaveFailedMsg{err: err}
		}
		return studentSavedMsg{student: student}
	}
}

func (m studentFormModel) Update(msg tea.Msg) (studentFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case studentSaveFailedMsg:
		m.saving = false
		m.err = msg.err
		return m, m.name.Focus()
	case tea.WindowSizeMsg:
		m.name.SetWidth(max(1, min(40, msg.Width-4)))
	}
	if m.saving {
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "enter" {
		m.saving = true
		m.err = nil
		return m, saveStudentCmd(m.ctx, m.service, m.studentID, m.name.Value())
	}
	before := m.name.Value()
	var cmd tea.Cmd
	m.name, cmd = m.name.Update(msg)
	if m.name.Value() != before {
		m.err = nil
	}
	return m, cmd
}

func (m studentFormModel) View() string {
	title := "Новый ученик"
	if m.studentID != 0 {
		title = "Редактирование ученика"
	}
	text := title + "\n\nИмя\n" + m.name.View()
	if m.saving {
		return text + "\n\nСохранение…"
	}
	if m.err != nil {
		if errors.Is(m.err, domain.ErrStudentNameRequired) || errors.Is(m.err, domain.ErrStudentNameTooLong) {
			text += "\n\n" + m.err.Error()
		} else if errors.Is(m.err, domain.ErrStudentNotFound) {
			text += "\n\nУченик больше не существует. Нажмите Esc и обновите список по r."
		} else {
			text += "\n\nНе удалось сохранить ученика. Нажмите Enter для повторной попытки."
		}
	}
	return text
}
