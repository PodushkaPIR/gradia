package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type helpModel struct {
	title  string
	lines  []string
	offset int
	height int
	keys   helpKeyMap
}

func (a App) contextHelp() helpModel {
	title, bindings := a.students.FullHelp()
	if a.showTemplates {
		title, bindings = a.templates.FullHelp()
	}
	return helpModel{title: title, lines: fullHelpLines(bindings), height: max(1, a.height-6), keys: newHelpKeyMap()}
}

func (m helpModel) Update(msg tea.Msg) helpModel {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = max(1, msg.Height-6)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Down.Binding):
			m.offset++
		case key.Matches(msg, m.keys.Up.Binding):
			m.offset--
		case key.Matches(msg, m.keys.PageDown.Binding):
			m.offset += m.height
		case key.Matches(msg, m.keys.PageUp.Binding):
			m.offset -= m.height
		}
	}
	m.offset = max(0, min(m.offset, len(m.lines)-m.height))
	return m
}

func (m helpModel) View() string {
	end := min(len(m.lines), m.offset+m.height)
	return "Помощь — " + m.title + "\n\n" + strings.Join(m.lines[m.offset:end], "\n")
}
