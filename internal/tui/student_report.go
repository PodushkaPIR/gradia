package tui

import tea "charm.land/bubbletea/v2"

type learningReportGeneratedMsg struct {
	path string
}

type learningReportOpenedMsg struct {
	path string
	err  error
}

type learningReportFailedMsg struct{ err error }

func (m studentDetailsModel) generateReport() (studentDetailsModel, tea.Cmd) {
	if m.reportService == nil || m.loading || m.err != nil || m.cursor < 0 || m.cursor >= len(m.routes) {
		return m, nil
	}
	m.reportGenerating, m.reportPlanID, m.reportPath, m.reportErr, m.reportOpenErr = true, m.routes[m.cursor].ID, "", nil, nil
	ctx, service, student, plan, screen := m.ctx, m.reportService, m.student.ID, m.reportPlanID, m.screenID
	return m, func() tea.Msg {
		path, err := service.Generate(ctx, student, plan)
		if err != nil {
			return routeToStudentDetails(screen, learningReportFailedMsg{err: err})
		}
		return routeToStudentDetails(screen, learningReportGeneratedMsg{path: path})
	}
}

func (m studentDetailsModel) openReport(path string) tea.Cmd {
	ctx, service, screen := m.ctx, m.reportService, m.screenID
	return func() tea.Msg {
		return routeToStudentDetails(screen, learningReportOpenedMsg{path: path, err: service.Open(ctx, path)})
	}
}

func (m studentDetailsModel) reportNotice() string {
	if m.cursor < 0 || m.cursor >= len(m.routes) || m.routes[m.cursor].ID != m.reportPlanID {
		return ""
	}
	if m.reportErr != nil {
		return "\n\nНе удалось сформировать HTML-отчёт. Нажмите " + m.keys.Report.Help().Key + " для повторной попытки."
	}
	if m.reportPath == "" {
		return ""
	}
	text := "\n\nHTML-отчёт сохранён: " + m.reportPath
	if m.reportOpenErr != nil {
		text += "\nНе удалось открыть браузер. Откройте файл вручную."
	}
	return text
}
