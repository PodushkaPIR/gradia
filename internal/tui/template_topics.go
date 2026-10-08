package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type templateTopicsLoadedMsg struct {
	screenID uint64
	topics   []domain.TemplateTopic
}

type templateTopicsLoadFailedMsg struct {
	screenID uint64
	err      error
}

type templateTopicCreatedMsg struct {
	screenID uint64
	topic    domain.TemplateTopic
}

type templateTopicCreateFailedMsg struct {
	screenID uint64
	err      error
}

type templateTopicsModel struct {
	ctx        context.Context
	service    *app.TemplateService
	template   domain.Template
	screenID   uint64
	topics     []domain.TemplateTopic
	cursor     int
	loading    bool
	err        error
	width      int
	height     int
	form       *templateFormModel
	path       []domain.TemplateTopic
	selections map[domain.TemplateTopicID]domain.TemplateTopicID
}

func (m templateTopicsModel) reload() (templateTopicsModel, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	m.loading = true
	m.err = nil
	ctx, service, id, screenID := m.ctx, m.service, m.template.ID, m.screenID
	return m, func() tea.Msg {
		topics, err := service.ListTopics(ctx, id)
		if err != nil {
			return templateTopicsLoadFailedMsg{screenID: screenID, err: err}
		}
		return templateTopicsLoadedMsg{screenID: screenID, topics: topics}
	}
}

func createTemplateTopicCmd(ctx context.Context, service *app.TemplateService, id domain.TemplateID, parentID domain.TemplateTopicID, name string, screenID uint64) tea.Cmd {
	return func() tea.Msg {
		var topic domain.TemplateTopic
		var err error
		if parentID == 0 {
			topic, err = service.AddTopic(ctx, id, name)
		} else {
			topic, err = service.AddSubtopic(ctx, id, parentID, name)
		}
		if err != nil {
			return templateTopicCreateFailedMsg{screenID: screenID, err: err}
		}
		return templateTopicCreatedMsg{screenID: screenID, topic: topic}
	}
}

func (m templateTopicsModel) Update(msg tea.Msg) (templateTopicsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case templateTopicsLoadedMsg:
		if msg.screenID == m.screenID {
			m.topics, m.loading, m.err = msg.topics, false, nil
			m.cursor = max(0, min(m.cursor, len(m.rows())-1))
		}
		return m, nil
	case templateTopicsLoadFailedMsg:
		if msg.screenID == m.screenID {
			m.loading, m.err = false, msg.err
		}
		return m, nil
	case templateTopicCreatedMsg:
		if msg.screenID == m.screenID {
			m.form = nil
			m.topics = append(append([]domain.TemplateTopic(nil), m.topics...), msg.topic)
			m = m.selectTopic(msg.topic.ID)
		}
		return m, nil
	case templateTopicCreateFailedMsg:
		if msg.screenID != m.screenID {
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, max(1, msg.Height-8)
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
			rows := m.rows()
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(rows) {
				path, err := domain.TemplateTopicPath(m.topics, rows[m.cursor].topic.ID)
				if err == nil {
					m = m.rememberSelection()
					m.path, m.cursor = path, 0
					m = m.selectTopic(m.selections[m.currentParentID()])
				}
			}
		case "esc":
			if len(m.path) > 0 {
				m = m.rememberSelection()
				previous := m.currentParentID()
				m.path = m.path[:len(m.path)-1]
				selected := m.selections[m.currentParentID()]
				if selected == 0 {
					selected = previous
				}
				m = m.selectTopic(selected)
			}
		case "n":
			if !m.loading && m.err == nil && m.canCreate() {
				form := newTemplateForm(m.ctx, m.service, m.width)
				form.templateID, form.screenID = m.template.ID, m.screenID
				form.parentID = m.currentParentID()
				form.name.Placeholder = "Например, Циклы"
				form.name.CharLimit = domain.TopicNameMaxLength
				cmd := form.Init()
				m.form = &form
				return m, cmd
			}
		case "r":
			return m.reload()
		case "j", "down":
			m.cursor = max(0, min(m.cursor+1, len(m.rows())-1))
		case "k", "up":
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m templateTopicsModel) View() string {
	names := []string{m.template.Name}
	for _, topic := range m.path {
		names = append(names, topic.Name)
	}
	title := "Темы — " + strings.Join(names, " › ") + "\n\n"
	if m.form != nil {
		return title + m.form.View()
	}
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить темы. Нажмите r для повторной попытки."
	}
	if !m.canCreate() {
		return title + "Достигнут предел: 5 уровней тем."
	}
	rows := m.rows()
	if len(rows) == 0 {
		return title + "Тем пока нет. Нажмите n для создания."
	}
	var text strings.Builder
	text.WriteString(title)
	start := max(0, m.cursor-m.height+1)
	end := min(len(rows), start+m.height)
	for i := start; i < end; i++ {
		if i > start {
			text.WriteByte('\n')
		}
		if i == m.cursor {
			text.WriteString("> ")
		} else {
			text.WriteString("  ")
		}
		text.WriteString(strings.Repeat("  ", rows[i].depth))
		if i+1 < len(rows) && rows[i+1].depth > rows[i].depth {
			text.WriteString("▾ ")
		} else {
			text.WriteString("• ")
		}
		text.WriteString(rows[i].topic.Name)
	}
	return text.String()
}

func (m templateTopicsModel) ShortHelp() string {
	if m.form != nil {
		if m.form.saving {
			return ""
		}
		return "Enter — сохранить · Esc — отменить"
	}
	if !m.canCreate() {
		return "Esc — назад · ? — помощь"
	}
	return "Enter — открыть · n — создать · Esc — назад · ? — помощь"
}
