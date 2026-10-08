package tui

import (
	"context"
	"errors"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type templateCreatedMsg struct {
	template domain.Template
}

type templateCreateFailedMsg struct {
	err error
}

type templateFormModel struct {
	ctx        context.Context
	service    *app.TemplateService
	name       textinput.Model
	templateID domain.TemplateID
	parentID   domain.TemplateTopicID
	screenID   uint64
	saving     bool
	err        error
}

func newTemplateForm(ctx context.Context, service *app.TemplateService, width int) templateFormModel {
	name := textinput.New()
	name.Placeholder = "Например, Python с нуля"
	name.CharLimit = domain.TemplateNameMaxLength
	name.SetWidth(max(1, min(40, width-4)))
	return templateFormModel{ctx: ctx, service: service, name: name}
}

func (m *templateFormModel) Init() tea.Cmd {
	return m.name.Focus()
}

func createTemplateCmd(ctx context.Context, service *app.TemplateService, name string) tea.Cmd {
	return func() tea.Msg {
		template, err := service.Create(ctx, name)
		if err != nil {
			return templateCreateFailedMsg{err: err}
		}
		return templateCreatedMsg{template: template}
	}
}

func (m templateFormModel) Update(msg tea.Msg) (templateFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case templateCreateFailedMsg:
		m.saving = false
		m.err = msg.err
		return m, m.name.Focus()
	case templateTopicCreateFailedMsg:
		m.saving = false
		m.err = msg.err
		return m, m.name.Focus()
	case tea.WindowSizeMsg:
		m.name.SetWidth(max(1, min(40, msg.Width-4)))
	}
	if m.saving {
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "enter" {
		m.saving = true
		m.err = nil
		if m.templateID != 0 {
			return m, createTemplateTopicCmd(m.ctx, m.service, m.templateID, m.parentID, m.name.Value(), m.screenID)
		}
		return m, createTemplateCmd(m.ctx, m.service, m.name.Value())
	}
	before := m.name.Value()
	var cmd tea.Cmd
	m.name, cmd = m.name.Update(msg)
	if m.name.Value() != before {
		m.err = nil
	}
	return m, cmd
}

func (m templateFormModel) View() string {
	title := "Новый шаблон"
	if m.templateID != 0 {
		title = "Новая тема"
	}
	text := title + "\n\nНазвание\n" + m.name.View()
	if m.saving {
		return text + "\n\nСохранение…"
	}
	if m.err != nil {
		if errors.Is(m.err, domain.ErrTemplateNameRequired) || errors.Is(m.err, domain.ErrTemplateNameTooLong) || errors.Is(m.err, domain.ErrTopicNameRequired) || errors.Is(m.err, domain.ErrTopicNameTooLong) || errors.Is(m.err, domain.ErrTopicDepthExceeded) || errors.Is(m.err, domain.ErrTopicNotFound) {
			text += "\n\n" + m.err.Error()
		} else {
			entity := "шаблон"
			if m.templateID != 0 {
				entity = "тему"
			}
			text += "\n\nНе удалось сохранить " + entity + ". Нажмите Enter для повторной попытки."
		}
	}
	return text
}
