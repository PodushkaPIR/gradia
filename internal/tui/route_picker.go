package tui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type routeTemplatesLoadedMsg struct{ templates []domain.Template }
type routeTemplatesFailedMsg struct{ err error }
type routeCreatedMsg struct{ route domain.Route }
type routeCreateFailedMsg struct{ err error }

type routePickerModel struct {
	ctx                 context.Context
	templatesService    *app.TemplateService
	routesService       *app.RouteService
	studentID           domain.StudentID
	detailsID, screenID uint64
	templates           []domain.Template
	cursor, height      int
	loading, creating   bool
	err                 error
	keys                routePickerKeyMap
}

func (m routePickerModel) reload() (routePickerModel, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	m.loading, m.err = true, nil
	ctx, svc, details, picker := m.ctx, m.templatesService, m.detailsID, m.screenID
	return m, func() tea.Msg {
		templates, err := svc.List(ctx)
		if err != nil {
			return routeToPicker(details, picker, routeTemplatesFailedMsg{err: err})
		}
		return routeToPicker(details, picker, routeTemplatesLoadedMsg{templates: templates})
	}
}

func (m routePickerModel) Update(msg tea.Msg) (routePickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case routeTemplatesLoadedMsg:
		m.templates, m.loading, m.err = msg.templates, false, nil
		m.cursor = max(0, min(m.cursor, len(m.templates)-1))
		return m, nil
	case routeTemplatesFailedMsg:
		m.loading, m.err = false, msg.err
		return m, nil
	case tea.WindowSizeMsg:
		m.height = max(1, msg.Height-8)
	}
	if m.creating {
		return m, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Select.Binding):
			if !m.loading && len(m.templates) > 0 {
				m.creating, m.err = true, nil
				ctx, svc, student, template, details := m.ctx, m.routesService, m.studentID, m.templates[m.cursor].ID, m.detailsID
				return m, func() tea.Msg {
					route, err := svc.Create(ctx, student, template)
					if err != nil {
						return routeToStudentDetails(details, routeCreateFailedMsg{err: err})
					}
					return routeToStudentDetails(details, routeCreatedMsg{route: route})
				}
			}
		case key.Matches(pressed, m.keys.Reload.Binding):
			return m.reload()
		case key.Matches(pressed, m.keys.Down.Binding):
			m.cursor = max(0, min(m.cursor+1, len(m.templates)-1))
		case key.Matches(pressed, m.keys.Up.Binding):
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m routePickerModel) View() string {
	if m.creating {
		return "Создание персонального учебного плана…"
	}
	if m.loading {
		return "Выбор шаблона\n\nЗагрузка…"
	}
	var text strings.Builder
	text.WriteString("Выбор шаблона\n\n")
	if m.err != nil {
		text.WriteString("Не удалось выполнить операцию. " + m.keys.Reload.Help().Key + " — обновить, " + m.keys.Select.Help().Key + " — повторить создание.\n\n")
	}
	if len(m.templates) == 0 {
		text.WriteString("Шаблонов пока нет. Создайте шаблон в разделе шаблонов.")
		return text.String()
	}
	start := max(0, m.cursor-m.height+1)
	for i := start; i < min(len(m.templates), start+m.height); i++ {
		if i > start {
			text.WriteByte('\n')
		}
		if i == m.cursor {
			text.WriteString("> ")
		} else {
			text.WriteString("  ")
		}
		text.WriteString(m.templates[i].Name)
	}
	return text.String()
}

func (m routePickerModel) InputMode() inputMode {
	if m.creating {
		return inputBusy
	}
	return inputConfirmation
}
