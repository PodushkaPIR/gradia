package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type helpModel struct {
	title  string
	lines  []string
	offset int
	height int
}

func (a App) contextHelp() helpModel {
	title := "Ученики"
	text := "↑/↓, j/k    Передвижение\nn           Создать ученика\ne           Редактировать имя\nd           Удалить с подтверждением\np           Открыть шаблоны\nr           Обновить список\n?           Помощь\nq           Выход"
	if a.showTemplates {
		title = "Шаблоны обучения"
		text = "↑/↓, j/k    Передвижение\nEnter       Открыть темы шаблона\nn           Создать шаблон\nr           Обновить список\nEsc         Вернуться к ученикам\n?           Помощь\nq           Выход"
		if a.templates.topics != nil {
			title = "Темы шаблона"
			text = "↑/↓, j/k    Передвижение\nEnter       Открыть тему"
			if a.templates.topics.canCreate() {
				text += "\nn           Создать тему на текущем уровне"
			}
			text += "\nr           Обновить список\nEsc         На уровень назад\n?           Помощь\nq           Выход"
		}
	} else if a.deleteDialog != nil {
		title = "Удаление ученика"
		text = "y           Подтвердить удаление\nn, Esc      Отменить удаление\n?           Помощь"
	}
	return helpModel{title: title, lines: strings.Split(text, "\n"), height: max(1, a.height-6)}
}

func (m helpModel) Update(msg tea.Msg) helpModel {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = max(1, msg.Height-6)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			m.offset++
		case "k", "up":
			m.offset--
		case "pgdown":
			m.offset += m.height
		case "pgup":
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
