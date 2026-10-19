package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestHomeworkPersistsTaskCompletionWithoutChangingLearningPlan(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "homework.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	students := app.NewStudentService(store)
	student, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	other, err := students.Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	templates := app.NewTemplateService(store)
	template, err := templates.Create(ctx, "Python")
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
	before, err := routes.Topics(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	history, err := routes.ProgressHistory(ctx, plan.ID, topic.ID)
	if err != nil {
		t.Fatal(err)
	}
	homework := app.NewHomeworkService(store)
	assignment, err := homework.Create(ctx, student.ID, "  Циклы и функции  ")
	if err != nil {
		t.Fatal(err)
	}
	if assignment.Name != "Циклы и функции" || assignment.TotalTasks != 0 || assignment.CompletedTasks != 0 {
		t.Fatalf("bad new Homework: %+v", assignment)
	}
	first, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: "# Упражнение\r\n\r\nНапишите `for` и функцию.\n'); DROP TABLE students; --", Topics: []string{" Циклы ", "Функции", "Циклы", "Первая\r\nВторая"}, Difficulty: domain.DifficultyMedium})
	if err != nil {
		t.Fatal(err)
	}
	if first.Completed || strings.Contains(first.Content, "\r") || !reflect.DeepEqual(first.Topics, []string{"Циклы", "Функции", "Первая\nВторая"}) {
		t.Fatalf("bad new task: %+v", first)
	}
	second, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: "https://example.org/task?id=2", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyHard})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := homework.SetCompleted(ctx, student.ID, assignment.ID, first.ID, true); err != nil {
		t.Fatal(err)
	}
	edited, err := homework.EditTask(ctx, student.ID, assignment.ID, first.ID, domain.HomeworkTaskInput{Content: "https://example.org/revised?q='test'", Topics: []string{"Функции", "Строки"}, Difficulty: domain.DifficultyEasy})
	if err != nil {
		t.Fatal(err)
	}
	if !edited.Completed || edited.Position != first.Position {
		t.Fatalf("editing reset task completion/order: %+v", edited)
	}
	renamed, err := homework.Rename(ctx, student.ID, assignment.ID, "Практика")
	if err != nil || renamed.Name != "Практика" || renamed.TotalTasks != 2 || renamed.CompletedTasks != 1 {
		t.Fatalf("summary not derived after rename: %+v %v", renamed, err)
	}
	if _, err := homework.Tasks(ctx, other.ID, assignment.ID); !errors.Is(err, domain.ErrHomeworkNotFound) {
		t.Fatalf("foreign Homework visible: %v", err)
	}
	if _, err := homework.SetCompleted(ctx, other.ID, assignment.ID, first.ID, false); !errors.Is(err, domain.ErrHomeworkNotFound) {
		t.Fatalf("foreign task editable: %v", err)
	}
	if actual, err := routes.Topics(ctx, plan.ID); err != nil || !reflect.DeepEqual(actual, before) {
		t.Fatalf("Homework changed mastery: %+v %v", actual, err)
	}
	if actual, err := routes.ProgressHistory(ctx, plan.ID, topic.ID); err != nil || !reflect.DeepEqual(actual, history) {
		t.Fatalf("Homework changed progress history: %+v %v", actual, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	homework = app.NewHomeworkService(reopened)
	list, err := homework.List(ctx, student.ID)
	if err != nil || len(list) != 1 || list[0].CompletedTasks != 1 || list[0].TotalTasks != 2 {
		t.Fatalf("Homework summary lost after reopening: %+v %v", list, err)
	}
	tasks, err := homework.Tasks(ctx, student.ID, assignment.ID)
	if err != nil || len(tasks) != 2 || !reflect.DeepEqual(tasks[0], edited) || tasks[1].ID != second.ID || tasks[1].Completed {
		t.Fatalf("task data lost: %+v %v", tasks, err)
	}
	if _, err := homework.SetCompleted(ctx, student.ID, assignment.ID, first.ID, false); err != nil {
		t.Fatal(err)
	}
	list, err = homework.List(ctx, student.ID)
	if err != nil || list[0].CompletedTasks != 0 {
		t.Fatalf("completion reset not reflected: %+v %v", list, err)
	}
}

func TestHomeworkRejectsInvalidAndForeignTaskChanges(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "validation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	students := app.NewStudentService(store)
	first, err := students.Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	second, err := students.Create(ctx, "Анна")
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewHomeworkService(store)
	assignment, err := service.Create(ctx, first.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.Create(ctx, second.ID, "Другое задание")
	if err != nil {
		t.Fatal(err)
	}
	input := domain.HomeworkTaskInput{Content: "https://example.org/task", Topics: []string{"Циклы"}, Difficulty: domain.DifficultyEasy}
	task, err := service.AddTask(ctx, first.ID, assignment.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := service.AddTask(ctx, second.ID, other.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func() error
		want   error
	}{
		{"missing student", func() error { _, err := service.Create(ctx, 99999, "Новое"); return err }, domain.ErrStudentNotFound},
		{"empty title", func() error { _, err := service.Create(ctx, first.ID, " "); return err }, domain.ErrHomeworkNameRequired},
		{"foreign rename", func() error { _, err := service.Rename(ctx, second.ID, assignment.ID, "Чужое"); return err }, domain.ErrHomeworkNotFound},
		{"foreign creation", func() error { _, err := service.AddTask(ctx, second.ID, assignment.ID, input); return err }, domain.ErrHomeworkNotFound},
		{"foreign task edit", func() error { _, err := service.EditTask(ctx, first.ID, assignment.ID, foreign.ID, input); return err }, domain.ErrHomeworkTaskNotFound},
		{"foreign task completion", func() error {
			_, err := service.SetCompleted(ctx, first.ID, assignment.ID, foreign.ID, true)
			return err
		}, domain.ErrHomeworkTaskNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.change(); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
	for _, test := range []struct {
		name  string
		input domain.HomeworkTaskInput
		want  error
	}{
		{"empty content", domain.HomeworkTaskInput{Content: " ", Topics: input.Topics, Difficulty: input.Difficulty}, domain.ErrTaskContentRequired},
		{"empty topics", domain.HomeworkTaskInput{Content: input.Content, Topics: []string{" "}, Difficulty: input.Difficulty}, domain.ErrTaskTopicsRequired},
		{"invalid difficulty", domain.HomeworkTaskInput{Content: input.Content, Topics: input.Topics, Difficulty: "impossible"}, domain.ErrInvalidTaskDifficulty},
		{"long topic", domain.HomeworkTaskInput{Content: input.Content, Topics: []string{strings.Repeat("я", domain.TopicNameMaxLength+1)}, Difficulty: input.Difficulty}, domain.ErrTopicNameTooLong},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.EditTask(ctx, first.ID, assignment.ID, task.ID, test.input); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
	if actual, err := service.Tasks(ctx, first.ID, assignment.ID); err != nil || len(actual) != 1 || !reflect.DeepEqual(actual[0], task) {
		t.Fatalf("invalid edit changed task: %+v %v", actual, err)
	}
	if actual, err := service.Tasks(ctx, second.ID, other.ID); err != nil || len(actual) != 1 || !reflect.DeepEqual(actual[0], foreign) {
		t.Fatalf("foreign task changed: %+v %v", actual, err)
	}
	if err := students.Delete(ctx, first.ID); !errors.Is(err, domain.ErrStudentInUse) {
		t.Fatalf("student with Homework could be deleted: %v", err)
	}
}

func TestHomeworkTaskContentAndTopicsRollBackTogether(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewHomeworkService(store)
	assignment, err := service.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: "Исходный текст", Topics: []string{"Циклы", "Функции"}, Difficulty: domain.DifficultyMedium})
	if err != nil {
		t.Fatal(err)
	}
	task, err = service.SetCompleted(ctx, student.ID, assignment.ID, task.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_homework_topic BEFORE INSERT ON homework_task_topics WHEN NEW.topic_name = 'reject' BEGIN SELECT RAISE(ABORT, 'topic unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	input := domain.HomeworkTaskInput{Content: "Новый текст", Topics: []string{"Новая тема", "reject"}, Difficulty: domain.DifficultyHard}
	if _, err := service.EditTask(ctx, student.ID, assignment.ID, task.ID, input); err == nil {
		t.Fatal("expected topic persistence failure")
	}
	if _, err := service.AddTask(ctx, student.ID, assignment.ID, input); err == nil {
		t.Fatal("expected creation rollback")
	}
	if actual, err := service.Tasks(ctx, student.ID, assignment.ID); err != nil || len(actual) != 1 || !reflect.DeepEqual(actual[0], task) {
		t.Fatalf("partial task change escaped rollback: %+v %v", actual, err)
	}
	if actual, err := service.List(ctx, student.ID); err != nil || len(actual) != 1 || actual[0].TotalTasks != 1 || actual[0].CompletedTasks != 1 {
		t.Fatalf("failed save changed derived summary: %+v %v", actual, err)
	}
}
