package tui

import (
	"context"
	"strings"

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
	form      *templateFormModel
	topics    *templateTopicsModel
	topicsSeq uint64
}

func newTemplatesModel(ctx context.Context, service *app.TemplateService) templatesModel {
	return templatesModel{ctx: ctx, service: service, width: 80, height: 10}
}

func loadTemplatesCmd(ctx context.Context, service *app.TemplateService) tea.Cmd {
	return func() tea.Msg {
		templates, err := service.List(ctx)
		if err != nil {
			return templatesLoadFailedMsg{err: err}
		}
		return templatesLoadedMsg{templates: templates}
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
	case templateTopicsLoadedMsg, templateTopicsLoadFailedMsg, templateTopicCreatedMsg, templateTopicCreateFailedMsg:
		if m.topics == nil {
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = max(1, msg.Height-8)
	}
	if m.topics != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" && m.topics.form == nil && len(m.topics.path) == 0 {
			m.topics = nil
			return m, nil
		}
		topics, cmd := m.topics.Update(msg)
		m.topics = &topics
		return m, cmd
	}
	if m.form != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" && !m.form.saving {
			m.form = nil
			return m, nil
		}
		form, cmd := m.form.Update(msg)
		m.form = &form
		return m, cmd
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "enter":
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(m.templates) {
				m.topicsSeq++
				topics := templateTopicsModel{ctx: m.ctx, service: m.service, template: m.templates[m.cursor], screenID: m.topicsSeq, width: m.width, height: m.height}
				topics, cmd := topics.reload()
				m.topics = &topics
				return m, cmd
			}
		case "n":
			if !m.loading {
				form := newTemplateForm(m.ctx, m.service, m.width)
				cmd := form.Init()
				m.form = &form
				return m, cmd
			}
		case "r":
			return m.reload()
		case "j", "down":
			m.cursor = max(0, min(m.cursor+1, len(m.templates)-1))
		case "k", "up":
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
		return title + "Не удалось загрузить шаблоны. Нажмите r для повторной попытки."
	}
	if len(m.templates) == 0 {
		return title + "Шаблонов пока нет. Нажмите n для создания."
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
		text.WriteString("• ")
		text.WriteString(m.templates[i].Name)
	}
	return text.String()
}

func (m templatesModel) ShortHelp() string {
	if m.topics != nil {
		return m.topics.ShortHelp()
	}
	if m.form != nil {
		if m.form.saving {
			return ""
		}
		return "Enter — сохранить · Esc — отменить"
	}
	return "Enter — открыть · n — создать · Esc — назад · ? — помощь"
}

func (m templatesModel) textInputActive() bool {
	return m.form != nil || (m.topics != nil && m.topics.form != nil)
}
