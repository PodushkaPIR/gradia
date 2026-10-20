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

type homeworkListTestStore struct {
	*sqlite.Store
	listErr error
	lists   int
}

func (s *homeworkListTestStore) ListHomework(ctx context.Context, student domain.StudentID) ([]domain.Homework, error) {
	s.lists++
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.Store.ListHomework(ctx, student)
}

func TestLearningReportIncludesHomeworkSummaryOnlyWhenExplicitlyRequested(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "report.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	other, err := students.Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	template, err := app.NewTemplateService(store).Create(ctx, "Python")
	if err != nil {
		t.Fatal(err)
	}
	routes := app.NewRouteService(store)
	plan, err := routes.Create(ctx, student.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	topic, err := routes.AddTopic(ctx, plan.ID, 0, "Циклы")
	if err != nil {
		t.Fatal(err)
	}
	if err := routes.SetStatus(ctx, plan.ID, topic.ID, domain.ProgressLearning); err != nil {
		t.Fatal(err)
	}
	if err := routes.SetNote(ctx, plan.ID, topic.ID, "PRIVATE_TUTOR_NOTE"); err != nil {
		t.Fatal(err)
	}
	homeworkRepo := &homeworkListTestStore{Store: store}
	homework := app.NewHomeworkService(homeworkRepo)
	assignment, err := homework.Create(ctx, student.ID, "Практика <script>bad()</script>")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		task, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: "PRIVATE_HOMEWORK_TASK_BODY", Topics: []string{"PRIVATE_TASK_TOPIC"}, Difficulty: domain.DifficultyMedium})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if _, err := homework.SetCompleted(ctx, student.ID, assignment.ID, task.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := homework.Create(ctx, student.ID, "Пустое ДЗ"); err != nil {
		t.Fatal(err)
	}
	if _, err := homework.Create(ctx, other.ID, "OTHER_STUDENT_HOMEWORK"); err != nil {
		t.Fatal(err)
	}
	beforeTopics, err := routes.Topics(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeHomework, err := homework.List(ctx, student.ID)
	if err != nil {
		t.Fatal(err)
	}
	homeworkRepo.lists = 0
	service := app.NewReportService(students, routes, reports.NewHTMLWriter(filepath.Join(dir, "reports")), nil).WithHomework(homework)
	plainPath, err := service.Generate(ctx, student.ID, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatal(err)
	}
	if homeworkRepo.lists != 0 || strings.Contains(string(plain), "Домашние задания ученика") || strings.Contains(string(plain), "bad()") || strings.Contains(string(plain), "Пустое ДЗ") {
		t.Fatal("default report read or exposed Homework")
	}
	path, err := service.GenerateWithOptions(ctx, student.ID, plan.ID, app.ReportOptions{IncludeHomework: true})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, want := range []string{"Домашние задания ученика", "Сводка по всем ДЗ ученика", "Выполнено 1 из 2 задач", "1/2 выполнено", "0/0 выполнено", "Пустое ДЗ", "Практика &lt;script&gt;bad()&lt;/script&gt;"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	for _, forbidden := range []string{"PRIVATE_TUTOR_NOTE", "PRIVATE_HOMEWORK_TASK_BODY", "PRIVATE_TASK_TOPIC", "OTHER_STUDENT_HOMEWORK", "<script>"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("summary exposed %q", forbidden)
		}
	}
	if homeworkRepo.lists != 1 {
		t.Fatalf("unexpected Homework lookups: %d", homeworkRepo.lists)
	}
	if actual, err := routes.Topics(ctx, plan.ID); err != nil || !reflect.DeepEqual(actual, beforeTopics) {
		t.Fatalf("report changed plan progress: %+v %v", actual, err)
	}
	if actual, err := homework.List(ctx, student.ID); err != nil || !reflect.DeepEqual(actual, beforeHomework) {
		t.Fatalf("report changed Homework state: %+v %v", actual, err)
	}
	otherPlan, err := routes.Create(ctx, other.ID, template.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherPath, err := service.GenerateWithOptions(ctx, other.ID, otherPlan.ID, app.ReportOptions{IncludeHomework: true})
	if err != nil {
		t.Fatal(err)
	}
	otherData, err := os.ReadFile(otherPath)
	if err != nil || !strings.Contains(string(otherData), "OTHER_STUDENT_HOMEWORK") || strings.Contains(string(otherData), "Практика") {
		t.Fatalf("student summaries mixed: %v", err)
	}
	homeworkRepo.listErr = errors.New("Homework unavailable")
	if _, err := service.Generate(ctx, student.ID, plan.ID); err != nil {
		t.Fatalf("unrequested Homework failure broke default report: %v", err)
	}
	if _, err := service.GenerateWithOptions(ctx, student.ID, plan.ID, app.ReportOptions{IncludeHomework: true}); !errors.Is(err, homeworkRepo.listErr) {
		t.Fatalf("requested summary silently omitted errors: %v", err)
	}
}
