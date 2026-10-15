package tui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
)

type App struct {
	students      studentsModel
	templates     templatesModel
	showTemplates bool
	width         int
	height        int
	help          *helpModel
	keys          globalKeyMap
}

func NewApp(ctx context.Context, students *app.StudentService, templates *app.TemplateService, routes *app.RouteService) App {
	screen := newStudentsModel(ctx, students)
	screen.routeService, screen.templateService = routes, templates
	return App{students: screen, templates: newTemplatesModel(ctx, templates), width: 80, height: 24, keys: newGlobalKeyMap()}
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
		var templateCmd, studentCmd tea.Cmd
		a.students, studentCmd = a.students.Update(msg)
		a.templates, templateCmd = a.templates.Update(msg)
		return a, tea.Batch(templateCmd, studentCmd)
	case screenMsg:
		var cmd tea.Cmd
		switch msg.target {
		case studentsScreen:
			a.students, cmd = a.students.Update(msg.payload)
		case templatesScreen:
			a.templates, cmd = a.templates.Update(msg.payload)
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
	if a.help != nil {
		body = a.help.View()
		help = shortHelp(a.help.keys.Close)
	}
	view := tea.NewView("Gradia\n\n" + body + "\n\n" + help)
	view.AltScreen = true
	return view
}
