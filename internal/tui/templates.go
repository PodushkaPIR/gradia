package tui

import (
	"context"
	"errors"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type templatesLoadedMsg struct {
	templates []domain.Template
}

type templatesLoadFailedMsg struct {
	err error
}

type templatesModel struct {
	ctx       context.Context
	service   *app.TemplateService
	templates []domain.Template
	cursor    int
	selectID  domain.TemplateID
	loading   bool
	err       error
	width     int
	height    int
	form      *nameForm
	topics    *templateTopicsModel
	topicsSeq uint64
	keys      templatesKeyMap
}

func newTemplatesModel(ctx context.Context, service *app.TemplateService) templatesModel {
	return templatesModel{ctx: ctx, service: service, width: 80, height: 10, keys: newTemplatesKeyMap()}
}

func loadTemplatesCmd(ctx context.Context, service *app.TemplateService) tea.Cmd {
	return func() tea.Msg {
		templates, err := service.List(ctx)
		if err != nil {
			return routeToTemplates(templatesLoadFailedMsg{err: err})
		}
		return routeToTemplates(templatesLoadedMsg{templates: templates})
	}
}

func (m templatesModel) reload() (templatesModel, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	m.loading = true
	m.err = nil
	return m, loadTemplatesCmd(m.ctx, m.service)
}

func (m templatesModel) Update(msg tea.Msg) (templatesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case templatesLoadedMsg:
		m.templates = msg.templates
		m.loading = false
		m.err = nil
		m.cursor = max(0, min(m.cursor, len(m.templates)-1))
		if m.selectID != 0 {
			for i, template := range m.templates {
				if template.ID == m.selectID {
					m.cursor = i
					break
				}
			}
			m.selectID = 0
		}
		return m, nil
	case templatesLoadFailedMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	case templateCreatedMsg:
		m.form = nil
		m.selectID = msg.template.ID
		return m.reload()
	case templateCreateFailedMsg:
		if m.form != nil {
			message := "Не удалось сохранить шаблон. Нажмите " + m.form.keys.Save.Help().Key + " для повторной попытки."
			if errors.Is(msg.err, domain.ErrTemplateNameRequired) || errors.Is(msg.err, domain.ErrTemplateNameTooLong) {
				message = msg.err.Error()
			}
			form, cmd := m.form.fail(msg.err, message)
			m.form = &form
			return m, cmd
		}
		return m, nil
	case topicsMsg:
		if m.topics == nil || msg.screenID != m.topics.screenID {
			return m, nil
		}
		topics, cmd := m.topics.Update(msg.payload)
		m.topics = &topics
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(1, msg.Height-8)
	}
	if m.topics != nil {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, m.topics.keys.Back.Binding) && m.topics.InputMode() == inputNavigation && len(m.topics.path) == 0 {
			m.topics = nil
			return m, nil
		}
		topics, cmd := m.topics.Update(msg)
		m.topics = &topics
		return m, cmd
	}
	if m.form != nil {
		form, action, cmd := m.form.Update(msg)
		m.form = &form
		switch action {
		case formCancel:
			m.form = nil
			return m, nil
		case formSubmit:
			return m, createTemplateCmd(m.ctx, m.service, form.name.Value())
		}
		return m, cmd
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Open.Binding):
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(m.templates) {
				m.topicsSeq++
				topics := templateTopicsModel{ctx: m.ctx, service: m.service, template: m.templates[m.cursor], screenID: m.topicsSeq, width: m.width, height: m.height, keys: newTopicsKeyMap()}
				topics, cmd := topics.reload()
				m.topics = &topics
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Create.Binding):
			if !m.loading {
				form := newNameForm(nameFormConfig{Title: "Новый шаблон", Label: "Название", Placeholder: "Например, Python с нуля", Limit: domain.TemplateNameMaxLength, Width: m.width})
				cmd := form.Init()
				m.form = &form
				return m, cmd
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

func (m templatesModel) View() string {
	if m.topics != nil {
		return m.topics.View()
	}
	if m.form != nil {
		return m.form.View()
	}
	const title = "Шаблоны обучения\n\n"
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить шаблоны. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки."
	}
	if len(m.templates) == 0 {
		return title + "Шаблонов пока нет. Нажмите " + m.keys.Create.Help().Key + " для создания."
	}
	var text strings.Builder
	text.WriteString(title)
	start := max(0, m.cursor-m.height+1)
	end := min(len(m.templates), start+m.height)
	for i := start; i < end; i++ {
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

func (m templatesModel) ShortHelp() string {
	if m.topics != nil {
		return m.topics.ShortHelp()
	}
	switch m.InputMode() {
	case inputBusy:
		return ""
	case inputText:
		return m.form.ShortHelp()
	}
	return shortHelp(m.keys.Open, m.keys.Create, m.keys.Back, m.keys.Help)
}

func (m templatesModel) FullHelp() (string, []shortcut) {
	if m.topics != nil {
		return m.topics.FullHelp()
	}
	keys := m.keys
	return "Шаблоны обучения", []shortcut{keys.helpBinding(), keys.Open, keys.Create, keys.Reload, keys.Back, keys.Help, keys.Quit}
}

func (m templatesModel) InputMode() inputMode {
	if m.topics != nil {
		return m.topics.InputMode()
	}
	if m.form != nil {
		return m.form.InputMode()
	}
	return inputNavigation
}

func (m templatesModel) atRoot() bool {
	return m.topics == nil && m.InputMode() == inputNavigation
}
