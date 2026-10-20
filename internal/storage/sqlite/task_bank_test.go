package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestTaskBankPersistsClassificationAndOverlappingCollectionsIndependentlyOfHomework(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bank.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	bank := app.NewTaskBankService(store)
	exam, err := bank.CreateCollection(ctx, "Экзамен")
	if err != nil {
		t.Fatal(err)
	}
	practice, err := bank.CreateCollection(ctx, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	firstInput := domain.TaskBankInput{Content: "# Циклы\r\nНапишите функцию", Subject: " Информатика ", Topics: []string{" Циклы ", "Функции; строки", "Класс\r\nи функции", "Циклы"}, Difficulty: domain.DifficultyMedium, CollectionIDs: []domain.TaskCollectionID{practice.ID, exam.ID, exam.ID}}
	first, err := bank.Create(ctx, firstInput)
	if err != nil {
		t.Fatal(err)
	}
	if first.Subject != "Информатика" || first.Content != "# Циклы\nНапишите функцию" || !reflect.DeepEqual(first.Topics, []string{"Циклы", "Функции; строки", "Класс\nи функции"}) || !reflect.DeepEqual(first.CollectionIDs, []domain.TaskCollectionID{exam.ID, practice.ID}) {
		t.Fatalf("bad normalized task: %+v", first)
	}
	second, err := bank.Create(ctx, domain.TaskBankInput{Content: "https://example.org/task?x='test'", Subject: "Математика", Topics: []string{"Дроби"}, Difficulty: domain.DifficultyHard, CollectionIDs: []domain.TaskCollectionID{exam.ID}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		filter domain.TaskBankFilter
		want   []domain.TaskBankTaskID
	}{
		{"all", domain.TaskBankFilter{}, []domain.TaskBankTaskID{first.ID, second.ID}},
		{"subject", domain.TaskBankFilter{Subject: "Информатика"}, []domain.TaskBankTaskID{first.ID}},
		{"topic", domain.TaskBankFilter{Topic: "Дроби"}, []domain.TaskBankTaskID{second.ID}},
		{"collection", domain.TaskBankFilter{CollectionID: practice.ID}, []domain.TaskBankTaskID{first.ID}},
		{"intersection", domain.TaskBankFilter{Subject: "Информатика", Topic: "Циклы", CollectionID: exam.ID}, []domain.TaskBankTaskID{first.ID}},
		{"no matches", domain.TaskBankFilter{Subject: "Информатика", Topic: "Дроби"}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			page, err := bank.Browse(ctx, test.filter)
			if err != nil {
				t.Fatal(err)
			}
			var ids []domain.TaskBankTaskID
			for _, task := range page.Tasks {
				ids = append(ids, task.ID)
			}
			if !reflect.DeepEqual(ids, test.want) {
				t.Fatalf("wrong filtered tasks: %v", ids)
			}
		})
	}
	page, err := bank.Browse(ctx, domain.TaskBankFilter{Subject: "Информатика"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(page.Tasks[0], first) || len(page.Subjects) != 2 || len(page.Topics) != 3 {
		t.Fatalf("metadata not retained: %+v", page)
	}
	counts := make(map[domain.TaskCollectionID]int)
	for _, collection := range page.Collections {
		counts[collection.ID] = collection.TaskCount
	}
	if counts[exam.ID] != 2 || counts[practice.ID] != 1 {
		t.Fatalf("overlap counts wrong: %v", counts)
	}
	student, err := app.NewStudentService(store).Create(ctx, "Иван")
	if err != nil {
		t.Fatal(err)
	}
	homework := app.NewHomeworkService(store)
	assignment, err := homework.Create(ctx, student.ID, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := homework.AddTask(ctx, student.ID, assignment.ID, domain.HomeworkTaskInput{Content: first.Content, Topics: first.Topics, Difficulty: first.Difficulty})
	if err != nil {
		t.Fatal(err)
	}
	firstInput.Content, firstInput.Difficulty, firstInput.CollectionIDs = "Изменённая задача банка", domain.DifficultyEasy, []domain.TaskCollectionID{practice.ID}
	edited, err := bank.Edit(ctx, first.ID, firstInput)
	if err != nil {
		t.Fatal(err)
	}
	if actual, err := homework.Tasks(ctx, student.ID, assignment.ID); err != nil || len(actual) != 1 || !reflect.DeepEqual(actual[0], assigned) {
		t.Fatalf("bank edit altered manual Homework: %+v %v", actual, err)
	}
	renamed, err := bank.RenameCollection(ctx, practice.ID, "Подготовка")
	if err != nil || renamed.TaskCount != 1 || renamed.Name != "Подготовка" {
		t.Fatalf("rename lost collection membership: %+v %v", renamed, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	bank = app.NewTaskBankService(reopened)
	page, err = bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || len(page.Tasks) != 2 || !reflect.DeepEqual(page.Tasks[0], edited) || !reflect.DeepEqual(page.Tasks[1], second) {
		t.Fatalf("bank data lost after restart: %+v %v", page, err)
	}
}

func TestTaskBankRejectsInvalidChangesAndRollsBackMetadataTogether(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	bank := app.NewTaskBankService(store)
	collection, err := bank.CreateCollection(ctx, "Практика")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bank.CreateCollection(ctx, " Практика "); !errors.Is(err, domain.ErrTaskCollectionNameExists) {
		t.Fatalf("duplicate collection accepted: %v", err)
	}
	input := domain.TaskBankInput{Content: "Текст", Subject: "Информатика", Topics: []string{"Циклы", "Функции"}, Difficulty: domain.DifficultyEasy, CollectionIDs: []domain.TaskCollectionID{collection.ID}}
	task, err := bank.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	before, err := bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		input domain.TaskBankInput
		want  error
	}{
		{"content", domain.TaskBankInput{Content: " ", Subject: input.Subject, Topics: input.Topics, Difficulty: input.Difficulty}, domain.ErrTaskContentRequired},
		{"subject", domain.TaskBankInput{Content: input.Content, Topics: input.Topics, Difficulty: input.Difficulty}, domain.ErrTaskSubjectRequired},
		{"topics", domain.TaskBankInput{Content: input.Content, Subject: input.Subject, Difficulty: input.Difficulty}, domain.ErrTaskTopicsRequired},
		{"difficulty", domain.TaskBankInput{Content: input.Content, Subject: input.Subject, Topics: input.Topics, Difficulty: "impossible"}, domain.ErrInvalidTaskDifficulty},
		{"collection", domain.TaskBankInput{Content: "Новый текст", Subject: input.Subject, Topics: input.Topics, Difficulty: input.Difficulty, CollectionIDs: []domain.TaskCollectionID{99999}}, domain.ErrTaskCollectionNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := bank.Edit(ctx, task.ID, test.input); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
	if _, err := bank.Edit(ctx, 99999, input); !errors.Is(err, domain.ErrTaskBankTaskNotFound) {
		t.Fatalf("missing task edited: %v", err)
	}
	if _, err := bank.Browse(ctx, domain.TaskBankFilter{CollectionID: 99999}); !errors.Is(err, domain.ErrTaskCollectionNotFound) {
		t.Fatalf("missing filter collection accepted: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER reject_bank_topic BEFORE INSERT ON task_bank_task_topics WHEN NEW.topic_name = 'reject' BEGIN SELECT RAISE(ABORT, 'topic unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	input.Content, input.Topics, input.CollectionIDs = "Обновлённый текст", []string{"Новая тема", "reject"}, nil
	if _, err := bank.Edit(ctx, task.ID, input); err == nil {
		t.Fatal("expected topic write failure")
	}
	if _, err := bank.Create(ctx, input); err == nil {
		t.Fatal("expected task creation rollback")
	}
	after, err := bank.Browse(ctx, domain.TaskBankFilter{})
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("failed edit lost content, metadata or membership: %+v %v", after, err)
	}
}
