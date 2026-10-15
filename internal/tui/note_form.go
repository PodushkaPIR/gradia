package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

type noteForm struct {
	title  string
	text   textarea.Model
	saving bool
	err    error
	keys   noteFormKeyMap
}

func newNoteForm(topic, value string, width, height int) noteForm {
	text := textarea.New()
	text.Prompt = ""
	text.ShowLineNumbers = false
	text.Placeholder = "Заметка в Markdown"
	text.SetWidth(max(1, min(78, width-4)))
	text.SetHeight(max(1, height-3))
	text.SetValue(value)
	return noteForm{title: "Заметка — " + topic + " (Markdown)", text: text, keys: newNoteFormKeyMap()}
}

func (m *noteForm) Init() tea.Cmd { return m.text.Focus() }

func (m noteForm) Update(msg tea.Msg) (noteForm, formAction, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.text.SetWidth(max(1, min(78, size.Width-4)))
		m.text.SetHeight(max(1, size.Height-13))
	}
	if m.saving {
		return m, formNoAction, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Cancel.Binding):
			return m, formCancel, nil
		case key.Matches(pressed, m.keys.Save.Binding):
			m.saving, m.err = true, nil
			return m, formSubmit, nil
		}
	}
	before := m.text.Value()
	var cmd tea.Cmd
	m.text, cmd = m.text.Update(msg)
	if m.text.Value() != before {
		m.err = nil
	}
	return m, formNoAction, cmd
}

func (m noteForm) fail(err error) (noteForm, tea.Cmd) {
	m.saving, m.err = false, err
	return m, m.text.Focus()
}
func (m noteForm) InputMode() inputMode {
	if m.saving {
		return inputBusy
	}
	return inputText
}
func (m noteForm) ShortHelp() string {
	if m.saving {
		return ""
	}
	return shortHelp(m.keys.Save, m.keys.Cancel)
}
func (m noteForm) View() string {
	text := m.title + "\n\n" + m.text.View()
	if m.saving {
		return text + "\n\nСохранение…"
	}
	if m.err != nil {
		text += "\n\nНе удалось сохранить заметку. Нажмите " + m.keys.Save.Help().Key + " для повторной попытки."
	}
	return text
}
