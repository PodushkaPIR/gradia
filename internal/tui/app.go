package tui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
)

type App struct {
	students        studentsModel
	templates       templatesModel
	showTemplates   bool
	taskBank        *taskBankModel
	taskBankService *app.TaskBankService
	taskBankSeq     uint64
	width           int
	height          int
	help            *helpModel
	keys            globalKeyMap
}

func NewApp(ctx context.Context, students *app.StudentService, templates *app.TemplateService, routes *app.RouteService) App {
	screen := newStudentsModel(ctx, students)
	screen.routeService, screen.templateService = routes, templates
	screen.keys.TaskBank.SetEnabled(false)
	keys := newGlobalKeyMap()
	keys.TaskBank.SetEnabled(false)
	return App{students: screen, templates: newTemplatesModel(ctx, templates), width: 80, height: 24, keys: keys}
}

func (a App) WithTaskBank(service *app.TaskBankService) App {
	a.taskBankService = service
	a.keys.TaskBank.SetEnabled(service != nil)
	a.students.keys.TaskBank.SetEnabled(service != nil)
	return a
}

func (a App) WithReports(reports *app.ReportService) App {
	a.students.reportService = reports
	return a
}

func (a App) WithHomework(homework *app.HomeworkService) App {
	a.students.homeworkService = homework
	return a
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
		var templateCmd, studentCmd, bankCmd tea.Cmd
		a.students, studentCmd = a.students.Update(msg)
		a.templates, templateCmd = a.templates.Update(msg)
		if a.taskBank != nil {
			model, cmd := a.taskBank.Update(msg)
			a.taskBank, bankCmd = &model, cmd
		}
		return a, tea.Batch(templateCmd, studentCmd, bankCmd)
	case screenMsg:
		var cmd tea.Cmd
		switch msg.target {
		case studentsScreen:
			a.students, cmd = a.students.Update(msg.payload)
		case templatesScreen:
			a.templates, cmd = a.templates.Update(msg.payload)
		case taskBankScreen:
			address, ok := msg.payload.(taskBankMsg)
			if !ok || a.taskBank == nil || a.taskBank.screenID != address.screenID {
				return a, nil
			}
			model, command := a.taskBank.Update(address.payload)
			a.taskBank, cmd = &model, command
		}
		return a, cmd
	}

	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(pressed, a.keys.Interrupt) {
			return a, tea.Quit
		}
	}
	if a.help != nil {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, a.help.keys.Close.Binding, a.keys.Help.Binding) {
			a.help = nil
			return a, nil
		}
		help := a.help.Update(msg)
		a.help = &help
		return a, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		mode := a.screenInputMode()
		if key.Matches(pressed, a.keys.Help.Binding) && mode.allowsHelp() {
			help := a.contextHelp()
			a.help = &help
			return a, nil
		}
		if key.Matches(pressed, a.keys.Quit.Binding) && mode.allowsQuit() {
			return a, tea.Quit
		}
	}
	if a.taskBank != nil {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, a.taskBank.keys.Back.Binding) && a.taskBank.atRoot() && a.taskBank.InputMode() == inputNavigation {
			a.taskBank = nil
			return a, nil
		}
		model, cmd := a.taskBank.Update(msg)
		a.taskBank = &model
		return a, cmd
	}
	if a.showTemplates {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, a.templates.keys.Back.Binding) && a.templates.atRoot() {
			a.showTemplates = false
			return a, nil
		}
		var cmd tea.Cmd
		a.templates, cmd = a.templates.Update(msg)
		return a, cmd
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(pressed, a.keys.TaskBank.Binding) && a.screenInputMode().allowsScreenSwitch() && a.taskBankService != nil {
			a.taskBankSeq++
			model := taskBankModel{ctx: a.students.ctx, service: a.taskBankService, screenID: a.taskBankSeq, width: a.width, height: max(1, a.height-8), keys: newTaskBankKeyMap()}
			model.keys.Attachments.SetEnabled(a.taskBankService.AttachmentsEnabled())
			model.keys.Mark.SetEnabled(a.taskBankService.TaskPackExportEnabled())
			model.keys.Export.SetEnabled(a.taskBankService.TaskPackExportEnabled())
			model.keys.Import.SetEnabled(a.taskBankService.TaskPackImportEnabled())
			model, cmd := model.reload()
			a.taskBank = &model
			return a, cmd
		}
		if key.Matches(pressed, a.keys.Templates.Binding) && a.screenInputMode().allowsScreenSwitch() && a.templates.service != nil {
			a.showTemplates = true
			var cmd tea.Cmd
			a.templates, cmd = a.templates.reload()
			return a, cmd
		}
	}
	var cmd tea.Cmd
	a.students, cmd = a.students.Update(msg)
	return a, cmd
}

func (a App) screenInputMode() inputMode {
	if a.taskBank != nil {
		return a.taskBank.InputMode()
	}
	if a.showTemplates {
		return a.templates.InputMode()
	}
	return a.students.InputMode()
}

func (a App) View() tea.View {
	body := a.students.View()
	help := a.students.ShortHelp()
	if a.showTemplates {
		body = a.templates.View()
		help = a.templates.ShortHelp()
	}
	if a.taskBank != nil {
		body, help = a.taskBank.View(), a.taskBank.ShortHelp()
	}
	if a.help != nil {
		body = a.help.View()
		help = shortHelp(a.help.keys.Close)
	}
	view := tea.NewView("Gradia\n\n" + body + "\n\n" + help)
	view.AltScreen = true
	return view
}
