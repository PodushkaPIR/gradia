package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
)

type App struct {
	students      studentsModel
	form          *studentFormModel
	deleteDialog  *studentDeleteModel
	templates     templatesModel
	showTemplates bool
	width         int
	height        int
	help          *helpModel
}

func NewApp(ctx context.Context, students *app.StudentService, templates *app.TemplateService) App {
	return App{students: newStudentsModel(ctx, students), templates: newTemplatesModel(ctx, templates), width: 80, height: 24}
}

func (a App) Init() tea.Cmd {
	return a.students.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.help != nil {
			help := a.help.Update(msg)
			a.help = &help
		}
		a.students, _ = a.students.Update(msg)
		var templateCmd, formCmd tea.Cmd
		a.templates, templateCmd = a.templates.Update(msg)
		if a.form != nil {
			form, cmd := a.form.Update(msg)
			a.form = &form
			formCmd = cmd
		}
		return a, tea.Batch(templateCmd, formCmd)
	case templatesLoadedMsg, templatesLoadFailedMsg, templateCreatedMsg, templateCreateFailedMsg:
		var cmd tea.Cmd
		a.templates, cmd = a.templates.Update(msg)
		return a, cmd
	case templateTopicsLoadedMsg, templateTopicsLoadFailedMsg, templateTopicCreatedMsg, templateTopicCreateFailedMsg:
		var cmd tea.Cmd
		a.templates, cmd = a.templates.Update(msg)
		return a, cmd
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
	case studentDeletedMsg:
		a.deleteDialog = nil
		a.students, _ = a.students.Update(msg)
		return a, nil
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "ctrl+c" {
			return a, tea.Quit
		}
	}
	if a.help != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok && (key.String() == "esc" || key.String() == "?") {
			a.help = nil
			return a, nil
		}
		help := a.help.Update(msg)
		a.help = &help
		return a, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		textInputActive := a.form != nil || (a.showTemplates && a.templates.textInputActive())
		if key.String() == "?" && !textInputActive && (a.deleteDialog == nil || !a.deleteDialog.deleting) {
			help := a.contextHelp()
			a.help = &help
			return a, nil
		}
		inputActive := a.form != nil || a.deleteDialog != nil || (a.showTemplates && a.templates.textInputActive())
		if key.String() == "q" && !inputActive {
			return a, tea.Quit
		}
	}
	if a.showTemplates {
		if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" && a.templates.form == nil && a.templates.topics == nil {
			a.showTemplates = false
			return a, nil
		}
		var cmd tea.Cmd
		a.templates, cmd = a.templates.Update(msg)
		return a, cmd
	}
	if a.deleteDialog != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok && !a.deleteDialog.deleting {
			if key.String() == "n" || key.String() == "esc" {
				a.deleteDialog = nil
				return a, nil
			}
		}
		dialog, cmd := a.deleteDialog.Update(msg)
		a.deleteDialog = &dialog
		return a, cmd
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "p" && a.form == nil && a.templates.service != nil {
			a.showTemplates = true
			var cmd tea.Cmd
			a.templates, cmd = a.templates.reload()
			return a, cmd
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
		if key.String() == "d" && a.form == nil {
			if student, ok := a.students.selectedStudent(); ok {
				dialog := newStudentDelete(a.students.ctx, a.students.service, student)
				a.deleteDialog = &dialog
				return a, nil
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
	help := "n — создать · p — шаблоны · ? — помощь"
	if a.form != nil {
		body = a.form.View()
		help = "Enter — сохранить · Esc — отменить"
		if a.form.saving {
			help = ""
		}
	}
	if a.deleteDialog != nil {
		body = a.deleteDialog.View()
		help = "y — удалить · n / Esc — отменить"
		if a.deleteDialog.deleting {
			help = ""
		}
	}
	if a.showTemplates {
		body = a.templates.View()
		help = a.templates.ShortHelp()
	}
	if a.help != nil {
		body = a.help.View()
		help = "Esc — закрыть"
	}
	view := tea.NewView("Gradia\n\n" + body + "\n\n" + help)
	view.AltScreen = true
	return view
}
