package reports_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/reports"
	"github.com/shesh/gradia/internal/storage/sqlite"
)

func TestLocalHTMLReportUsesSelectedPlanAndOmitsPrivateAndExcludedData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	student, err := domain.NewStudent("Иван <Иванов>")
	if err != nil {
		t.Fatal(err)
	}
	student.Notes = "PRIVATE_STUDENT_NOTE"
	student, err = store.CreateStudent(ctx, student)
	if err != nil {
		t.Fatal(err)
	}
	templates := app.NewTemplateService(store)
	template, err := templates.Create(ctx, "Исходный шаблон")
	if err != nil {
		t.Fatal(err)
	}
	routes := app.NewRouteService(store)
	plan, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := routes.Rename(ctx, plan.ID, "Личный план"); err != nil {
		t.Fatal(err)
	}
	if _, err := routes.SetDescription(ctx, plan.ID, "Цель: <script>alert('x')</script>\nРабота с циклами"); err != nil {
		t.Fatal(err)
	}
	group, err := routes.AddTopic(ctx, plan.ID, 0, "Раздел")
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range []struct {
		name   string
		status domain.ProgressStatus
	}{
		{"Предстоящая А", domain.ProgressNotStarted},
		{"Освоенная", domain.ProgressMastered},
		{"Изучаемая", domain.ProgressLearning},
		{"Практика", domain.ProgressNeedsPractice},
	} {
		topic, err := routes.AddTopic(ctx, plan.ID, group.ID, item.name)
		if err != nil {
			t.Fatal(err)
		}
		if err := routes.SetStatus(ctx, plan.ID, topic.ID, item.status); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if err := routes.SetRevision(ctx, plan.ID, topic.ID, true); err != nil {
				t.Fatal(err)
			}
		}
		if err := routes.SetNote(ctx, plan.ID, topic.ID, "PRIVATE_TOPIC_NOTE"); err != nil {
			t.Fatal(err)
		}
	}
	next, err := routes.AddTopic(ctx, plan.ID, 0, "Предстоящая Б")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := routes.SwapTopics(ctx, plan.ID, group.ID, next.ID); err != nil {
		t.Fatal(err)
	}
	hidden, err := routes.AddTopic(ctx, plan.ID, 0, "EXCLUDED_BRANCH")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := routes.AddTopic(ctx, plan.ID, hidden.ID, "EXCLUDED_DESCENDANT"); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetExcluded(ctx, plan.ID, hidden.ID, true); err != nil {
		t.Fatal(err)
	}
	other, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := routes.Rename(ctx, other.ID, "OTHER_PLAN_SECRET"); err != nil {
		t.Fatal(err)
	}
	before, err := routes.Topics(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(dir, "reports")
	service := app.NewReportService(app.NewStudentService(store), routes, reports.NewHTMLWriter(outputDir), nil)
	path, err := service.Generate(ctx, student.ID, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != outputDir || filepath.Ext(path) != ".html" {
		t.Fatalf("unexpected report path %q", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(content)
	for _, want := range []string{"Иван &lt;Иванов&gt;", "Личный план", "Работа с циклами", "Освоено 1 из 5 тем", "Текущих: 2", "Предстоящих: 2", "нужна практика", "текущая", "предстоящая", "завершённая", "повторить", `name="viewport"`, "@media (max-width: 600px)", "Печать / сохранить PDF", `onclick="window.print()"`, "Сохранить как PDF"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in report", want)
		}
	}
	for _, forbidden := range []string{"PRIVATE_STUDENT_NOTE", "PRIVATE_TOPIC_NOTE", "EXCLUDED_BRANCH", "EXCLUDED_DESCENDANT", "OTHER_PLAN_SECRET", "Исходный шаблон", "<script>", "<form", "Домашнее задание", `src="https://`, `href="https://`} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("report leaked %q", forbidden)
		}
	}
	previous := -1
	for _, name := range []string{"Предстоящая Б", "Раздел", "Предстоящая А", "Освоенная", "Изучаемая", "Практика"} {
		index := strings.Index(html, ">"+name+"<")
		if index <= previous {
			t.Fatalf("wrong report order at %q", name)
		}
		previous = index
	}
	if actual, err := routes.Topics(ctx, plan.ID); err != nil || !reflect.DeepEqual(actual, before) {
		t.Fatalf("report generation changed plan: %+v %v", actual, err)
	}
	foreign, err := app.NewStudentService(store).Create(ctx, "Другой ученик")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Generate(ctx, foreign.ID, plan.ID); !errors.Is(err, domain.ErrRouteNotFound) {
		t.Fatalf("report exposed another student's plan: %v", err)
	}
	if _, err := service.Generate(ctx, 99999, plan.ID); !errors.Is(err, domain.ErrStudentNotFound) {
		t.Fatalf("missing student accepted: %v", err)
	}
	files, err := os.ReadDir(outputDir)
	if err != nil || len(files) != 1 {
		t.Fatalf("invalid requests generated reports: %v %v", files, err)
	}
}

func TestLocalHTMLReportHandlesEmptyPlansCancellationAndOutputErrors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writer := reports.NewHTMLWriter(dir)
	report := app.LearningReport{StudentName: "Иван", PlanName: "Пустой план"}
	path, err := writer.Write(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Освоено 0 из 0 тем", "В плане нет активных тем.", `aria-valuenow="0"`} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("missing empty-plan content %q", want)
		}
	}
	if strings.Contains(string(content), "#ZgotmplZ") {
		t.Fatal("template produced invalid escaped CSS")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := writer.Write(canceled, report); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("canceled request created output: %v %v", files, err)
	}
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.NewHTMLWriter(blocked).Write(ctx, report); err == nil {
		t.Fatal("expected output-directory failure")
	}
	if content, err := os.ReadFile(blocked); err != nil || string(content) != "preserve" {
		t.Fatalf("failed generation altered existing data: %s %v", content, err)
	}
}
