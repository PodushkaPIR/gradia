package tui

import tea "charm.land/bubbletea/v2"

type screenTarget uint8

const (
	studentsScreen screenTarget = iota
	templatesScreen
	taskBankScreen
)

// screenMsg адресует результат операции постоянному экрану приложения.
// Клавиши, resize и служебные команды Bubble Tea не оборачиваются.
type screenMsg struct {
	target  screenTarget
	payload tea.Msg
}

type taskBankMsg struct {
	screenID uint64
	payload  tea.Msg
}

func routeToTaskBank(id uint64, msg tea.Msg) screenMsg {
	return screenMsg{target: taskBankScreen, payload: taskBankMsg{screenID: id, payload: msg}}
}

// topicsMsg адресует результат конкретному экземпляру вложенного экрана тем.
type topicsMsg struct {
	screenID uint64
	payload  tea.Msg
}

type studentDetailsMsg struct {
	screenID uint64
	payload  tea.Msg
}
type routePickerMsg struct {
	screenID uint64
	payload  tea.Msg
}
type routeViewMsg struct {
	screenID uint64
	payload  tea.Msg
}

type homeworkMsg struct {
	screenID uint64
	payload  tea.Msg
}

type homeworkAttachmentsMsg struct {
	screenID uint64
	payload  tea.Msg
}

func routeToHomeworkAttachments(detailsID, homeworkID, screenID uint64, msg tea.Msg) screenMsg {
	return routeToHomework(detailsID, homeworkID, homeworkAttachmentsMsg{screenID: screenID, payload: msg})
}

func routeToHomework(detailsID, homeworkID uint64, msg tea.Msg) screenMsg {
	return routeToStudentDetails(detailsID, homeworkMsg{screenID: homeworkID, payload: msg})
}

func routeToStudentDetails(id uint64, msg tea.Msg) screenMsg {
	return routeToStudents(studentDetailsMsg{screenID: id, payload: msg})
}

func routeToPicker(detailsID, pickerID uint64, msg tea.Msg) screenMsg {
	return routeToStudentDetails(detailsID, routePickerMsg{screenID: pickerID, payload: msg})
}

func routeToRouteView(detailsID, viewID uint64, msg tea.Msg) screenMsg {
	return routeToStudentDetails(detailsID, routeViewMsg{screenID: viewID, payload: msg})
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
