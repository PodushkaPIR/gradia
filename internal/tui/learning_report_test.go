package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type reportWriterStub struct {
	writes int
	err    error
	report app.LearningReport
}

type pdfExporterStub struct {
	exports int
	err     error
}

func (e *pdfExporterStub) Export(_ context.Context, htmlPath string) (string, error) {
	e.exports++
	if htmlPath != "/tmp/learning-report.html" {
		return "", errors.New("wrong source HTML")
	}
	if e.err != nil {
		return "", e.err
	}
	return "/tmp/learning-report.pdf", nil
}

func (w *reportWriterStub) Write(_ context.Context, report app.LearningReport) (string, error) {
	w.writes++
	w.report = report
	if w.err != nil {
		return "", w.err
	}
	return "/tmp/learning-report.html", nil
}

func TestTutorGeneratesReportThroughCommandAndKeepsFilePathWhenBrowserFails(t *testing.T) {
	ctx := context.Background()
	repo := &progressRepoStub{topics: []domain.RouteTopic{{ID: 2, RouteID: 10, Name: "Тема", Status: domain.ProgressMastered}}}
	repo.routes = []domain.Route{{ID: 10, StudentID: 1, Name: "Выбранный план"}, {ID: 11, StudentID: 1, Name: "Другой план"}}
	students := app.NewStudentService(&studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван", Notes: "Private"}}})
	routes := app.NewRouteService(repo)
	writer := &reportWriterStub{}
	opens := 0
	openErr := errors.New("browser unavailable")
	service := app.NewReportService(students, routes, writer, func(_ context.Context, path string) error {
		opens++
		if path != "/tmp/learning-report.html" {
			t.Fatalf("wrong path %q", path)
		}
		return openErr
	})
	model := NewApp(ctx, students, nil, routes).WithReports(service)
	model, _ = updateApp(model, model.Init()())
	model, cmd := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "Учебные планы") {
		t.Fatal("student card should use Learning Plan terminology")
	}
	model, cmd = updateApp(model, keypress('h'))
	if cmd == nil || writer.writes != 0 || opens != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("report work must run only inside Cmd")
	}
	model, duplicate := updateApp(model, keypress('h'))
	if duplicate != nil {
		t.Fatal("duplicate report generation")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.details == nil {
		t.Fatal("left student card during report generation")
	}
	model, open := updateApp(model, cmd())
	if open == nil || opens != 0 || model.screenInputMode() != inputNavigation || !strings.Contains(model.View().Content, "/tmp/learning-report.html") {
		t.Fatal("saved path and navigation must be available before opening browser")
	}
	model, _ = updateApp(model, open())
	view := model.View().Content
	if !strings.Contains(view, "/tmp/learning-report.html") || !strings.Contains(view, "Не удалось открыть браузер") || model.screenInputMode() != inputNavigation {
		t.Fatal("browser failure hid the exported file or blocked retry")
	}
	if writer.report.PlanName != "Выбранный план" || writer.report.StudentName != "Иван" || writer.writes != 1 || opens != 1 {
		t.Fatal("wrong report scope or unexpected writes")
	}
	writer.err = errors.New("disk unavailable")
	model, cmd = updateApp(model, keypress('h'))
	model, _ = updateApp(model, cmd())
	if !strings.Contains(model.View().Content, "Не удалось сформировать HTML-отчёт") || opens != 1 {
		t.Fatal("generation failure tried opening browser or hid error")
	}
	writer.err, openErr = nil, nil
	model, cmd = updateApp(model, keypress('h'))
	model, open = updateApp(model, cmd())
	model, _ = updateApp(model, open())
	if !strings.Contains(model.View().Content, "HTML-отчёт сохранён:") || strings.Contains(model.View().Content, "Не удалось") || opens != 2 {
		t.Fatal("retry did not recover")
	}
	model, _ = updateApp(model, keypress('j'))
	if strings.Contains(model.View().Content, "/tmp/learning-report.html") {
		t.Fatal("report for previous plan shown as the selected plan's report")
	}
}

func TestTutorExportsPDFThroughUtilityAndCanRecoverWhenUtilityFails(t *testing.T) {
	ctx := context.Background()
	repo := &routeRepoStub{routes: []domain.Route{{ID: 10, StudentID: 1, Name: "Выбранный план"}}}
	students := app.NewStudentService(&studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}})
	routes := app.NewRouteService(repo)
	writer := &reportWriterStub{}
	pdf := &pdfExporterStub{err: errors.New("Chromium/Chrome не найден")}
	opens := 0
	service := app.NewReportService(students, routes, writer, func(context.Context, string) error { opens++; return nil }).WithPDFExporter(pdf)
	model := NewApp(ctx, students, nil, routes).WithReports(service)
	model, _ = updateApp(model, model.Init()())
	model, cmd := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, cmd())
	model, cmd = updateApp(model, keypress('f'))
	if cmd == nil || model.screenInputMode() != inputBusy || writer.writes != 0 || pdf.exports != 0 {
		t.Fatal("PDF work must start through Cmd")
	}
	model, convert := updateApp(model, cmd())
	if convert == nil || pdf.exports != 0 || !strings.Contains(model.View().Content, "/tmp/learning-report.html") {
		t.Fatal("HTML must be saved before invoking utility")
	}
	model, duplicate := updateApp(model, keypress('f'))
	if duplicate != nil {
		t.Fatal("duplicate PDF conversion")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.students.details == nil {
		t.Fatal("left card during conversion")
	}
	model, _ = updateApp(model, convert())
	if model.screenInputMode() != inputNavigation || !strings.Contains(model.View().Content, "Chromium/Chrome не найден") || !strings.Contains(model.View().Content, "/tmp/learning-report.html") || opens != 0 {
		t.Fatal("utility failure lost HTML or blocked recovery")
	}
	pdf.err = nil
	model, cmd = updateApp(model, keypress('f'))
	model, convert = updateApp(model, cmd())
	model, _ = updateApp(model, convert())
	if !strings.Contains(model.View().Content, "PDF-отчёт сохранён: /tmp/learning-report.pdf") || strings.Contains(model.View().Content, "Не удалось") || writer.writes != 2 || pdf.exports != 2 || opens != 0 {
		t.Fatal("retry did not export PDF independently from browser")
	}
	model, cmd = updateApp(model, keypress('h'))
	model, open := updateApp(model, cmd())
	model, _ = updateApp(model, open())
	if opens != 1 || pdf.exports != 2 || !strings.Contains(model.View().Content, "HTML-отчёт сохранён:") {
		t.Fatal("HTML/browser fallback no longer works")
	}
}
