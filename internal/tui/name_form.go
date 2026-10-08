package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type formAction uint8

const (
	formNoAction formAction = iota
	formSubmit
	formCancel
)

type nameFormConfig struct {
	Title, Label, Placeholder string
	Limit, Width              int
}

// nameForm управляет только вводом. Операцию и её результат обрабатывает экран.
type nameForm struct {
	title     string
	label     string
	name      textinput.Model
	saving    bool
	err       error
	errorText string
	keys      formKeyMap
}

func newNameForm(config nameFormConfig) nameForm {
	name := textinput.New()
	name.Placeholder = config.Placeholder
	name.CharLimit = config.Limit
	name.SetWidth(max(1, min(40, config.Width-4)))
	return nameForm{title: config.Title, label: config.Label, name: name, keys: newFormKeyMap()}
}

func (m *nameForm) Init() tea.Cmd {
	return m.name.Focus()
}

func (m nameForm) Update(msg tea.Msg) (nameForm, formAction, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.name.SetWidth(max(1, min(40, size.Width-4)))
	}
	if m.InputMode() == inputBusy {
		return m, formNoAction, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Cancel.Binding):
			return m, formCancel, nil
		case key.Matches(pressed, m.keys.Save.Binding):
			m.saving, m.err, m.errorText = true, nil, ""
			return m, formSubmit, nil
		}
	}
	before := m.name.Value()
	var cmd tea.Cmd
	m.name, cmd = m.name.Update(msg)
	if m.name.Value() != before {
		m.err, m.errorText = nil, ""
	}
	return m, formNoAction, cmd
}

func (m nameForm) fail(err error, message string) (nameForm, tea.Cmd) {
	m.saving, m.err, m.errorText = false, err, message
	return m, m.name.Focus()
}

func (m nameForm) View() string {
	text := m.title + "\n\n" + m.label + "\n" + m.name.View()
	if m.InputMode() == inputBusy {
		return text + "\n\nСохранение…"
	}
	if m.err != nil {
		text += "\n\n" + m.errorText
	}
	return text
}

func (m nameForm) ShortHelp() string {
	if m.InputMode() == inputBusy {
		return ""
	}
	return shortHelp(m.keys.Save, m.keys.Cancel)
}

func (m nameForm) InputMode() inputMode {
	if m.saving {
		return inputBusy
	}
	return inputText
}
