package tui

import tea "charm.land/bubbletea/v2"

type learningReportGeneratedMsg struct {
	path string
	pdf  bool
}

type learningReportOpenedMsg struct {
	path string
	err  error
}

type learningReportFailedMsg struct{ err error }

type learningReportPDFMsg struct {
	htmlPath, pdfPath string
	err               error
}

func (m studentDetailsModel) generateReport(pdf bool) (studentDetailsModel, tea.Cmd) {
	if m.reportService == nil || m.loading || m.err != nil || m.cursor < 0 || m.cursor >= len(m.routes) {
		return m, nil
	}
	m.reportGenerating, m.reportPlanID, m.reportPath, m.reportErr, m.reportOpenErr = true, m.routes[m.cursor].ID, "", nil, nil
	m.reportPDFExporting, m.reportPDFPath, m.reportPDFErr = false, "", nil
	ctx, service, student, plan, screen := m.ctx, m.reportService, m.student.ID, m.reportPlanID, m.screenID
	return m, func() tea.Msg {
		path, err := service.Generate(ctx, student, plan)
		if err != nil {
			return routeToStudentDetails(screen, learningReportFailedMsg{err: err})
		}
		return routeToStudentDetails(screen, learningReportGeneratedMsg{path: path, pdf: pdf})
	}
}

func (m studentDetailsModel) exportReportPDF(path string) tea.Cmd {
	ctx, service, screen := m.ctx, m.reportService, m.screenID
	return func() tea.Msg {
		pdfPath, err := service.ExportPDF(ctx, path)
		return routeToStudentDetails(screen, learningReportPDFMsg{htmlPath: path, pdfPath: pdfPath, err: err})
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
	if m.reportPDFErr != nil {
		text += "\nНе удалось экспортировать PDF: " + m.reportPDFErr.Error() + ". Можно повторить по " + m.keys.PDFReport.Help().Key + " или сохранить PDF из браузера по " + m.keys.Report.Help().Key + "."
	}
	if m.reportPDFPath != "" {
		text += "\nPDF-отчёт сохранён: " + m.reportPDFPath
	}
	if m.reportOpenErr != nil {
		text += "\nНе удалось открыть браузер. Откройте файл вручную."
	}
	return text
}
