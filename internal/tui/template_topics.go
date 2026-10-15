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

type templateTopicsLoadedMsg struct {
	topics []domain.TemplateTopic
}

type templateTopicsLoadFailedMsg struct {
	err error
}

type templateTopicCreatedMsg struct {
	topic domain.TemplateTopic
}

type templateTopicCreateFailedMsg struct {
	err error
}

type templateTopicRenamedMsg struct {
	topic domain.TemplateTopic
}

type templateTopicRenameFailedMsg struct {
	err error
}

type templateTopicsModel struct {
	ctx         context.Context
	service     *app.TemplateService
	template    domain.Template
	screenID    uint64
	topics      []domain.TemplateTopic
	index       topicTreeIndex
	cursor      int
	loading     bool
	err         error
	width       int
	height      int
	form        *nameForm
	editTopicID domain.TemplateTopicID
	path        []domain.TemplateTopicID
	selections  map[domain.TemplateTopicID]domain.TemplateTopicID
	keys        topicsKeyMap
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
			return routeToTopics(screenID, templateTopicsLoadFailedMsg{err: err})
		}
		return routeToTopics(screenID, templateTopicsLoadedMsg{topics: topics})
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
			return routeToTopics(screenID, templateTopicCreateFailedMsg{err: err})
		}
		return routeToTopics(screenID, templateTopicCreatedMsg{topic: topic})
	}
}

func renameTemplateTopicCmd(ctx context.Context, service *app.TemplateService, templateID domain.TemplateID, topicID domain.TemplateTopicID, name string, screenID uint64) tea.Cmd {
	return func() tea.Msg {
		topic, err := service.RenameTopic(ctx, templateID, topicID, name)
		if err != nil {
			return routeToTopics(screenID, templateTopicRenameFailedMsg{err: err})
		}
		return routeToTopics(screenID, templateTopicRenamedMsg{topic: topic})
	}
}

func (m templateTopicsModel) newTopicForm() nameForm {
	return newNameForm(nameFormConfig{Title: "Новая тема", Label: "Название", Placeholder: "Например, Циклы", Limit: domain.TopicNameMaxLength, Width: m.width})
}

func (m templateTopicsModel) Update(msg tea.Msg) (templateTopicsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case templateTopicsLoadedMsg:
		m = m.setTopics(msg.topics)
		m.loading, m.err = false, nil
		m.cursor = max(0, min(m.cursor, len(m.rows())-1))
		return m, nil
	case templateTopicsLoadFailedMsg:
		m.loading, m.err = false, msg.err
		return m, nil
	case templateTopicCreatedMsg:
		m.form = nil
		m.editTopicID = 0
		m = m.setTopics(append(slices.Clone(m.topics), msg.topic))
		m = m.selectTopic(msg.topic.ID)
		return m, nil
	case templateTopicRenamedMsg:
		m.form = nil
		m.editTopicID = 0
		if i, ok := m.index.byID[msg.topic.ID]; ok {
			topics := slices.Clone(m.topics)
			topics[i] = msg.topic
			m = m.setTopics(topics)
		}
		m = m.selectTopic(msg.topic.ID)
		return m, nil
	case templateTopicCreateFailedMsg:
		return m.formFailed(msg.err)
	case templateTopicRenameFailedMsg:
		return m.formFailed(msg.err)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, max(1, msg.Height-8)
	}
	if m.form != nil {
		form, action, cmd := m.form.Update(msg)
		m.form = &form
		switch action {
		case formCancel:
			m.form = nil
			m.editTopicID = 0
			return m, nil
		case formSubmit:
			if m.editTopicID != 0 {
				return m, renameTemplateTopicCmd(m.ctx, m.service, m.template.ID, m.editTopicID, form.name.Value(), m.screenID)
			}
			return m, createTemplateTopicCmd(m.ctx, m.service, m.template.ID, m.currentParentID(), form.name.Value(), m.screenID)
		}
		return m, cmd
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Open.Binding):
			rows := m.rows()
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(rows) {
				path, err := domain.TemplateTopicPath(m.topics, rows[m.cursor].topic.ID)
				if err == nil {
					m = m.rememberSelection()
					m.path = make([]domain.TemplateTopicID, len(path))
					for i, topic := range path {
						m.path[i] = topic.ID
					}
					m.cursor = 0
					m = m.selectTopic(m.selections[m.currentParentID()])
				}
			}
		case key.Matches(pressed, m.keys.Back.Binding):
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
		case key.Matches(pressed, m.keys.Create.Binding):
			if !m.loading && m.err == nil && m.canCreate() {
				form := m.newTopicForm()
				cmd := form.Init()
				m.editTopicID = 0
				m.form = &form
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Edit.Binding):
			rows := m.rows()
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(rows) {
				form := m.newTopicForm()
				form.title = "Редактирование темы"
				form.name.SetValue(rows[m.cursor].topic.Name)
				form.name.CursorEnd()
				cmd := form.Init()
				m.editTopicID = rows[m.cursor].topic.ID
				m.form = &form
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Reload.Binding):
			return m.reload()
		case key.Matches(pressed, m.keys.Down.Binding):
			m.cursor = max(0, min(m.cursor+1, len(m.rows())-1))
		case key.Matches(pressed, m.keys.Up.Binding):
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m templateTopicsModel) formFailed(err error) (templateTopicsModel, tea.Cmd) {
	if m.form == nil {
		return m, nil
	}
	message := "Не удалось сохранить тему. Нажмите " + m.form.keys.Save.Help().Key + " для повторной попытки."
	if errors.Is(err, domain.ErrTopicNameRequired) || errors.Is(err, domain.ErrTopicNameTooLong) || errors.Is(err, domain.ErrTopicDepthExceeded) || errors.Is(err, domain.ErrTopicNotFound) {
		message = err.Error()
	}
	form, cmd := m.form.fail(err, message)
	m.form = &form
	return m, cmd
}

func (m templateTopicsModel) View() string {
	names := []string{m.template.Name}
	for _, id := range m.path {
		if topic, ok := m.topicByID(id); ok {
			names = append(names, topic.Name)
		}
	}
	title := "Темы — " + strings.Join(names, " › ") + "\n\n"
	if m.form != nil {
		return title + m.form.View()
	}
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить темы. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки."
	}
	if !m.canCreate() {
		return title + "Достигнут предел: 5 уровней тем."
	}
	rows := m.rows()
	if len(rows) == 0 {
		return title + "Тем пока нет. Нажмите " + m.keys.Create.Help().Key + " для создания."
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
	switch m.InputMode() {
	case inputBusy:
		return ""
	case inputText:
		return m.form.ShortHelp()
	}
	if !m.canCreate() {
		return shortHelp(m.keys.Back, m.keys.Help)
	}
	return shortHelp(m.keys.Open, m.keys.Create, m.keys.Back, m.keys.Help)
}

func (m templateTopicsModel) FullHelp() (string, []shortcut) {
	keys := m.keys
	bindings := []shortcut{keys.helpBinding(), keys.Open, keys.Edit}
	if m.canCreate() {
		bindings = append(bindings, keys.Create)
	}
	bindings = append(bindings, keys.Reload, keys.Back, keys.Help, keys.Quit)
	return "Темы шаблона", bindings
}

func (m templateTopicsModel) InputMode() inputMode {
	if m.form != nil {
		return m.form.InputMode()
	}
	return inputNavigation
}
