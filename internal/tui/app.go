package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
)

type App struct {
	students studentsModel
	form     *studentFormModel
	width    int
}

func NewApp(ctx context.Context, service *app.StudentService) App {
	return App{students: newStudentsModel(ctx, service), width: 80}
}

func (a App) Init() tea.Cmd {
	return a.students.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.students, _ = a.students.Update(msg)
	case studentsLoadedMsg, studentsLoadFailedMsg:
		var cmd tea.Cmd
		a.students, cmd = a.students.Update(msg)
		return a, cmd
	case studentSavedMsg:
		a.form = nil
		a.students.loading = true
		a.students.err = nil
		a.students.selectID = msg.student.ID
		return a, a.students.Init()
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "ctrl+c" || (key.String() == "q" && a.form == nil) {
			return a, tea.Quit
		}
		if key.String() == "esc" && a.form != nil && !a.form.saving {
			a.form = nil
			return a, nil
		}
		if key.String() == "n" && a.form == nil && !a.students.loading {
			form := newStudentForm(a.students.ctx, a.students.service, a.width)
			cmd := form.Init()
			a.form = &form
			return a, cmd
		}
		if key.String() == "e" && a.form == nil {
			if student, ok := a.students.selectedStudent(); ok {
				form := newStudentEditForm(a.students.ctx, a.students.service, a.width, student)
				cmd := form.Init()
				a.form = &form
				return a, cmd
			}
		}
	}
	if a.form != nil {
		form, cmd := a.form.Update(msg)
		a.form = &form
		return a, cmd
	}
	var cmd tea.Cmd
	a.students, cmd = a.students.Update(msg)
	return a, cmd
}

func (a App) View() tea.View {
	body := a.students.View()
	help := "n — новый ученик · e — редактировать · j/k / ↑/↓ — выбор · r — обновить · q / Ctrl+C — выход"
	if a.form != nil {
		body = a.form.View()
		help = "Enter — сохранить · Esc — отменить · Ctrl+C — выход"
		if a.form.saving {
			help = "Ctrl+C — выход"
		}
	}
	view := tea.NewView("Gradia\n\n" + body + "\n\n" + help)
	view.AltScreen = true
	return view
}
