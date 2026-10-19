package tui

import (
	"context"
	"errors"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type studentRoutesLoadedMsg struct{ routes []domain.Route }
type studentRoutesFailedMsg struct{ err error }
type routeDetailsSavedMsg struct{ route domain.Route }
type routeDetailsFailedMsg struct{ err error }

type studentDetailsModel struct {
	ctx                          context.Context
	routesService                *app.RouteService
	templatesService             *app.TemplateService
	student                      domain.Student
	screenID, pickerSeq, viewSeq uint64
	routes                       []domain.Route
	cursor, height               int
	width                        int
	selectID                     domain.RouteID
	loading                      bool
	err                          error
	picker                       *routePickerModel
	view                         *routeViewModel
	routeName                    *nameForm
	routeDescription             *noteForm
	editRouteID                  domain.RouteID
	keys                         studentDetailsKeyMap
}

func (m studentDetailsModel) reload() (studentDetailsModel, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	m.loading, m.err = true, nil
	ctx, svc, student, screen := m.ctx, m.routesService, m.student.ID, m.screenID
	return m, func() tea.Msg {
		routes, err := svc.List(ctx, student)
		if err != nil {
			return routeToStudentDetails(screen, studentRoutesFailedMsg{err: err})
		}
		return routeToStudentDetails(screen, studentRoutesLoadedMsg{routes: routes})
	}
}

func (m studentDetailsModel) Update(msg tea.Msg) (studentDetailsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case studentRoutesLoadedMsg:
		m.routes, m.loading, m.err = msg.routes, false, nil
		m.cursor = max(0, min(m.cursor, len(m.routes)-1))
		for i, route := range m.routes {
			if route.ID == m.selectID {
				m.cursor = i
				break
			}
		}
		m.selectID = 0
		return m, nil
	case studentRoutesFailedMsg:
		m.loading, m.err = false, msg.err
		return m, nil
	case routeDetailsSavedMsg:
		m.routeName, m.routeDescription, m.editRouteID = nil, nil, 0
		m.routes = slices.Clone(m.routes)
		for i := range m.routes {
			if m.routes[i].ID == msg.route.ID {
				m.routes[i] = msg.route
				m.cursor = i
				break
			}
		}
		return m, nil
	case routeDetailsFailedMsg:
		if m.routeName != nil {
			message := "Не удалось сохранить название. Нажмите " + m.routeName.keys.Save.Help().Key + " для повторной попытки."
			if errors.Is(msg.err, domain.ErrRouteNameRequired) || errors.Is(msg.err, domain.ErrRouteNameTooLong) || errors.Is(msg.err, domain.ErrRouteNotFound) {
				message = msg.err.Error()
			}
			form, cmd := m.routeName.fail(msg.err, message)
			m.routeName = &form
			return m, cmd
		}
		if m.routeDescription != nil {
			form, cmd := m.routeDescription.fail(msg.err)
			m.routeDescription = &form
			return m, cmd
		}
		return m, nil
	case routeCreatedMsg:
		m.picker = nil
		m.selectID = msg.route.ID
		return m.reload()
	case routeCreateFailedMsg:
		if m.picker != nil {
			picker := *m.picker
			picker.creating, picker.err = false, msg.err
			m.picker = &picker
		}
		return m, nil
	case routePickerMsg:
		if m.picker == nil || msg.screenID != m.picker.screenID {
			return m, nil
		}
		picker, cmd := m.picker.Update(msg.payload)
		m.picker = &picker
		return m, cmd
	case routeViewMsg:
		if m.view == nil || msg.screenID != m.view.screenID {
			return m, nil
		}
		view, cmd := m.view.Update(msg.payload)
		m.view = &view
		return m, cmd
	case tea.WindowSizeMsg:
		m.height = max(1, msg.Height-12)
		m.width = msg.Width
	}
	if m.routeName != nil {
		form, action, cmd := m.routeName.Update(msg)
		m.routeName = &form
		switch action {
		case formCancel:
			m.routeName, m.editRouteID = nil, 0
			return m, nil
		case formSubmit:
			ctx, svc, id, name, screen := m.ctx, m.routesService, m.editRouteID, form.name.Value(), m.screenID
			return m, func() tea.Msg {
				route, err := svc.Rename(ctx, id, name)
				if err != nil {
					return routeToStudentDetails(screen, routeDetailsFailedMsg{err: err})
				}
				return routeToStudentDetails(screen, routeDetailsSavedMsg{route: route})
			}
		}
		return m, cmd
	}
	if m.routeDescription != nil {
		form, action, cmd := m.routeDescription.Update(msg)
		m.routeDescription = &form
		switch action {
		case formCancel:
			m.routeDescription, m.editRouteID = nil, 0
			return m, nil
		case formSubmit:
			ctx, svc, id, markdown, screen := m.ctx, m.routesService, m.editRouteID, form.text.Value(), m.screenID
			return m, func() tea.Msg {
				route, err := svc.SetDescription(ctx, id, markdown)
				if err != nil {
					return routeToStudentDetails(screen, routeDetailsFailedMsg{err: err})
				}
				return routeToStudentDetails(screen, routeDetailsSavedMsg{route: route})
			}
		}
		return m, cmd
	}
	if m.picker != nil {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, m.picker.keys.Cancel.Binding) && m.InputMode().allowsCancel() {
			m.picker = nil
			return m, nil
		}
		picker, cmd := m.picker.Update(msg)
		m.picker = &picker
		return m, cmd
	}
	if m.view != nil {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, m.view.keys.Back.Binding) && m.view.InputMode() == inputNavigation {
			m.view = nil
			return m, nil
		}
		view, cmd := m.view.Update(msg)
		m.view = &view
		return m, cmd
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.EditName.Binding):
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(m.routes) {
				route := m.routes[m.cursor]
				form := newNameForm(nameFormConfig{Title: "Название учебного плана", Label: "Название", Limit: domain.RouteNameMaxLength, Width: m.width})
				form.name.SetValue(route.Name)
				form.name.CursorEnd()
				cmd := form.Init()
				m.routeName, m.editRouteID = &form, route.ID
				return m, cmd
			}
		case key.Matches(pressed, m.keys.EditDescription.Binding):
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(m.routes) {
				route := m.routes[m.cursor]
				form := newNoteForm(route.Name, route.Description, m.width, m.height)
				form.title = "Описание учебного плана — " + route.Name + " (Markdown)"
				form.text.Placeholder = "Описание плана в Markdown"
				form.saveErrorText = "Не удалось сохранить описание."
				cmd := form.Init()
				m.routeDescription, m.editRouteID = &form, route.ID
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Create.Binding):
			if !m.loading && m.err == nil && m.templatesService != nil {
				m.pickerSeq++
				picker := routePickerModel{ctx: m.ctx, routesService: m.routesService, templatesService: m.templatesService, studentID: m.student.ID, detailsID: m.screenID, screenID: m.pickerSeq, height: m.height, keys: newRoutePickerKeyMap()}
				picker, cmd := picker.reload()
				m.picker = &picker
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Open.Binding):
			if !m.loading && m.err == nil && len(m.routes) > 0 {
				m.viewSeq++
				view := routeViewModel{ctx: m.ctx, service: m.routesService, route: m.routes[m.cursor], detailsID: m.screenID, screenID: m.viewSeq, height: m.height, width: m.width, keys: newRouteViewKeyMap()}
				view, cmd := view.reload()
				m.view = &view
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Reload.Binding):
			return m.reload()
		case key.Matches(pressed, m.keys.Down.Binding):
			m.cursor = max(0, min(m.cursor+1, len(m.routes)-1))
		case key.Matches(pressed, m.keys.Up.Binding):
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m studentDetailsModel) atRoot() bool {
	return m.picker == nil && m.view == nil && m.routeName == nil && m.routeDescription == nil
}
func (m studentDetailsModel) InputMode() inputMode {
	if m.routeName != nil {
		return m.routeName.InputMode()
	}
	if m.routeDescription != nil {
		return m.routeDescription.InputMode()
	}
	if m.picker != nil {
		return m.picker.InputMode()
	}
	if m.view != nil {
		return m.view.InputMode()
	}
	return inputNavigation
}

func (m studentDetailsModel) View() string {
	title := "Ученик — " + m.student.Name + "\n\n"
	if m.routeName != nil {
		return title + m.routeName.View()
	}
	if m.routeDescription != nil {
		return title + m.routeDescription.View()
	}
	if m.picker != nil {
		return title + m.picker.View()
	}
	if m.view != nil {
		return title + m.view.View()
	}
	var text strings.Builder
	text.WriteString(title)
	if m.student.Grade != nil {
		text.WriteString("Класс: " + *m.student.Grade + "\n\n")
	}
	if m.student.Notes != "" {
		text.WriteString("Заметки\n" + m.student.Notes + "\n\n")
	}
	text.WriteString("Учебные маршруты\n\n")
	if m.loading {
		text.WriteString("Загрузка…")
		return text.String()
	}
	if m.err != nil {
		text.WriteString("Не удалось загрузить маршруты. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки.")
		return text.String()
	}
	if len(m.routes) == 0 {
		text.WriteString("Маршрутов пока нет. Нажмите " + m.keys.Create.Help().Key + " для создания.")
		return text.String()
	}
	start := max(0, m.cursor-m.height+1)
	for i := start; i < min(len(m.routes), start+m.height); i++ {
		if i > start {
			text.WriteByte('\n')
		}
		if i == m.cursor {
			text.WriteString("> ")
		} else {
			text.WriteString("  ")
		}
		text.WriteString(m.routes[i].Name)
	}
	if m.cursor >= 0 && m.cursor < len(m.routes) && m.routes[m.cursor].Description != "" {
		lines := strings.Split(m.routes[m.cursor].Description, "\n")
		if len(lines) > 3 {
			lines = append(lines[:3], "…")
		}
		text.WriteString("\n\nОписание плана (Markdown)\n" + strings.Join(lines, "\n"))
	}
	return text.String()
}

func (m studentDetailsModel) ShortHelp() string {
	if m.routeName != nil {
		return m.routeName.ShortHelp()
	}
	if m.routeDescription != nil {
		return m.routeDescription.ShortHelp()
	}
	if m.picker != nil {
		if m.picker.creating {
			return ""
		}
		return shortHelp(m.picker.keys.Select, m.picker.keys.Cancel, m.picker.keys.Help)
	}
	if m.view != nil {
		if m.view.topicForm != nil {
			return m.view.topicForm.ShortHelp()
		}
		if m.view.note != nil {
			return m.view.note.ShortHelp()
		}
		if m.view.InputMode() == inputBusy {
			return ""
		}
		return shortHelp(m.view.keys.CreateTopic, m.view.keys.AddSubtopic, m.view.keys.EditTopic, m.view.keys.NextStatus, m.view.keys.ShowExcluded, m.view.keys.Back, m.view.keys.Help)
	}
	return shortHelp(m.keys.Open, m.keys.Create, m.keys.EditName, m.keys.EditDescription, m.keys.Back, m.keys.Help)
}

func (m studentDetailsModel) FullHelp() (string, []shortcut) {
	if m.picker != nil {
		keys := m.picker.keys
		return "Выбор шаблона", []shortcut{keys.helpBinding(), keys.Select, keys.Reload, keys.Cancel, keys.Help}
	}
	if m.view != nil {
		keys := m.view.keys
		return "Маршрут ученика", []shortcut{keys.helpBinding(), keys.CreateTopic, keys.AddSubtopic, keys.EditTopic, keys.MoveTopicUp, keys.MoveTopicDown, keys.NextStatus, keys.PreviousStatus, keys.Revision, keys.Exclude, keys.ShowExcluded, keys.EditNote, keys.Reload, keys.Back, keys.Help, keys.Quit}
	}
	keys := m.keys
	return "Карточка ученика", []shortcut{keys.helpBinding(), keys.Open, keys.Create, keys.EditName, keys.EditDescription, keys.Reload, keys.Back, keys.Help, keys.Quit}
}
