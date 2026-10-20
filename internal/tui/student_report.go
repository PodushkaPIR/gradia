package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
)

// Request options apply only to this artifact, never to the next ordinary report.
type learningReportRequest struct {
	pdf     bool
	options app.ReportOptions
}

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

func (m studentDetailsModel) generateReport(request learningReportRequest) (studentDetailsModel, tea.Cmd) {
	if m.reportService == nil || m.loading || m.err != nil || m.cursor < 0 || m.cursor >= len(m.routes) {
		return m, nil
	}
	m.reportGenerating, m.reportPlanID, m.reportPath, m.reportErr, m.reportOpenErr = true, m.routes[m.cursor].ID, "", nil, nil
	m.reportPDFExporting, m.reportPDFPath, m.reportPDFErr = false, "", nil
	m.reportRequest = request
	ctx, service, student, plan, screen := m.ctx, m.reportService, m.student.ID, m.reportPlanID, m.screenID
	return m, func() tea.Msg {
		path, err := service.GenerateWithOptions(ctx, student, plan, request.options)
		if err != nil {
			return routeToStudentDetails(screen, learningReportFailedMsg{err: err})
		}
		return routeToStudentDetails(screen, learningReportGeneratedMsg{path: path, pdf: request.pdf})
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
		return "\n\nНе удалось сформировать HTML-отчёт. Нажмите " + m.reportShortcut(m.reportRequest.pdf).Help().Key + " для повторной попытки."
	}
	if m.reportPath == "" {
		return ""
	}
	text := "\n\nHTML-отчёт сохранён: " + m.reportPath
	if m.reportRequest.options.IncludeHomework {
		text += "\nСводка ДЗ включена: все домашние задания ученика."
	}
	if m.reportPDFErr != nil {
		text += "\nНе удалось экспортировать PDF: " + m.reportPDFErr.Error() + ". Можно повторить по " + m.reportShortcut(true).Help().Key + " или сохранить PDF из браузера по " + m.reportShortcut(false).Help().Key + "."
	}
	if m.reportPDFPath != "" {
		text += "\nPDF-отчёт сохранён: " + m.reportPDFPath
	}
	if m.reportOpenErr != nil {
		text += "\nНе удалось открыть браузер. Откройте файл вручную."
	}
	return text
}

func (m studentDetailsModel) reportShortcut(pdf bool) shortcut {
	if m.reportRequest.options.IncludeHomework {
		if pdf {
			return m.keys.PDFWithHomework
		}
		return m.keys.ReportWithHomework
	}
	if pdf {
		return m.keys.PDFReport
	}
	return m.keys.Report
}
