package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

type homeworkSummaryTestStore struct {
	*sqlite.Store
	listErr error
}

func (s *homeworkSummaryTestStore) ListHomework(ctx context.Context, student domain.StudentID) ([]domain.Homework, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.Store.ListHomework(ctx, student)
}

func TestTutorExplicitlyIncludesHomeworkInHTMLAndPDFWithoutChangingDefaults(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "report.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	template, err := app.NewTemplateService(store).Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	routes := app.NewRouteService(store)
	if _, err := routes.Create(ctx, student.ID, template.ID); err != nil {
		t.Fatal(err)
	}
	repo := &homeworkSummaryTestStore{Store: store}
	homework := app.NewHomeworkService(repo)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		task, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: "Задача", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if _, err := homework.SetCompleted(ctx, student.ID, assignment.ID, task.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	writer, pdf := &reportWriterStub{}, &pdfExporterStub{}
	service := app.NewReportService(students, routes, writer, func(context.Context, string) error { return nil }).WithPDFExporter(pdf).WithHomework(homework)
	model := NewApp(ctx, students, nil, routes).WithReports(service)
	model, _ = updateApp(model, model.Init()())
	model, load := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = updateApp(model, load())
	model, generate := updateApp(model, keypress('H'))
	if generate == nil || writer.writes != 0 || model.screenInputMode() != inputBusy {
		t.Fatal("explicit HTML summary action must run through Cmd")
	}
	model, open := updateApp(model, generate())
	model, _ = updateApp(model, open())
	if writer.report.Homework == nil || writer.report.Homework.TotalTasks != 2 || writer.report.Homework.CompletedTasks != 1 {
		t.Fatal("explicit HTML report omitted the derived summary")
	}
	if !strings.Contains(model.View().Content, "Сводка ДЗ включена") {
		t.Fatal("included report scope not visible")
	}
	model, generate = updateApp(model, keypress('h'))
	model, open = updateApp(model, generate())
	model, _ = updateApp(model, open())
	if writer.report.Homework != nil {
		t.Fatal("previous opt-in leaked into ordinary HTML report")
	}
	model, generate = updateApp(model, keypress('F'))
	model, convert := updateApp(model, generate())
	if writer.report.Homework == nil || convert == nil || pdf.exports != 0 {
		t.Fatal("explicit PDF report did not include Homework before conversion")
	}
	model, _ = updateApp(model, convert())
	if pdf.exports != 1 || !strings.Contains(model.View().Content, "PDF-отчёт сохранён:") {
		t.Fatal("PDF with summary was not exported")
	}
	model, generate = updateApp(model, keypress('f'))
	model, convert = updateApp(model, generate())
	model, _ = updateApp(model, convert())
	if writer.report.Homework != nil || pdf.exports != 2 {
		t.Fatal("previous opt-in leaked into ordinary PDF")
	}
	repo.listErr = errors.New("Homework temporarily unavailable")
	model, generate = updateApp(model, keypress('H'))
	model, _ = updateApp(model, generate())
	if model.screenInputMode() != inputNavigation || !strings.Contains(model.View().Content, "Нажмите H") {
		t.Fatal("summary failure should be retryable with the same explicit action")
	}
	model, generate = updateApp(model, keypress('h'))
	model, open = updateApp(model, generate())
	model, _ = updateApp(model, open())
	if writer.report.Homework != nil || strings.Contains(model.View().Content, "Не удалось") {
		t.Fatal("unrequested Homework failure blocked default report")
	}
}
