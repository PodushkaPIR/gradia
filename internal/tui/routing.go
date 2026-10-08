package tui

import tea "charm.land/bubbletea/v2"

type screenTarget uint8

const (
	studentsScreen screenTarget = iota
	templatesScreen
)

// screenMsg адресует результат операции постоянному экрану приложения.
// Клавиши, resize и служебные команды Bubble Tea не оборачиваются.
type screenMsg struct {
	target  screenTarget
	payload tea.Msg
}

// topicsMsg адресует результат конкретному экземпляру вложенного экрана тем.
type topicsMsg struct {
	screenID uint64
	payload  tea.Msg
}

func routeToStudents(msg tea.Msg) screenMsg {
	return screenMsg{target: studentsScreen, payload: msg}
}

func routeToTemplates(msg tea.Msg) screenMsg {
	return screenMsg{target: templatesScreen, payload: msg}
}

func routeToTopics(screenID uint64, msg tea.Msg) screenMsg {
	return routeToTemplates(topicsMsg{screenID: screenID, payload: msg})
}
