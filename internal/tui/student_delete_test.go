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

func TestDeleteStudentFlow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		count  int
		cursor int
		wantID domain.StudentID
	}{
		{name: "first", count: 3, cursor: 0, wantID: 2},
		{name: "middle", count: 3, cursor: 1, wantID: 3},
		{name: "last", count: 3, cursor: 2, wantID: 2},
		{name: "only", count: 1, cursor: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			students := []domain.Student{{ID: 1, Name: "Анна"}, {ID: 2, Name: "Иван"}, {ID: 3, Name: "Максим"}}
			repo := &studentRepoStub{students: students[:tc.count]}
			model := NewApp(context.Background(), app.NewStudentService(repo), nil)
			model, _ = updateApp(model, model.Init()())
			model.students.cursor = tc.cursor
			target := model.students.students[tc.cursor]
			model, cmd := updateApp(model, keypress('d'))
			if model.students.deleteDialog == nil || model.students.deleteDialog.student.ID != target.ID || cmd != nil || repo.deleteCalls != 0 || !strings.Contains(model.View().Content, target.Name) {
				t.Fatal("d должен открыть подтверждение, без удаления записи")
			}
			// Диалог перехватывает навигацию, формы и обновление списка.
			for _, key := range []rune{'j', 'k', 'e', 'r', 'q', 'd'} {
				model, cmd = updateApp(model, keypress(key))
				if cmd != nil || model.students.form != nil || model.students.cursor != tc.cursor {
					t.Fatalf("клавиша %q вышла за пределы диалога", key)
				}
			}
			model, remove := updateApp(model, keypress('y'))
			if remove == nil || !model.students.deleteDialog.deleting || repo.deleteCalls != 0 || len(model.students.students) != tc.count {
				t.Fatal("удаление должно выполняться только в Cmd, список пока не меняется")
			}
			model, duplicate := updateApp(model, keypress('y'))
			if duplicate != nil {
				t.Fatal("запущено дублирующее удаление")
			}
			model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
			if model.students.deleteDialog == nil {
				t.Fatal("нельзя отменить уже выполняющееся удаление")
			}
			model, _ = updateApp(model, remove())
			if model.students.deleteDialog != nil || repo.deleteCalls != 1 || len(model.students.students) != tc.count-1 {
				t.Fatal("после успеха диалог должен закрыться, а список обновиться")
			}
			selected, ok := model.students.selectedStudent()
			if tc.wantID == 0 {
				if ok || model.students.cursor != 0 || !strings.Contains(model.View().Content, "Список пока пуст") {
					t.Fatal("после удаления последнего ученика ожидается пустой список")
				}
			} else if !ok || selected.ID != tc.wantID {
				t.Fatalf("выбран не ближайший ученик: %+v", selected)
			}
		})
	}
}

func TestDeleteStudentCancelAndErrors(t *testing.T) {
	for _, cancel := range []tea.KeyPressMsg{keypress('n'), {Code: tea.KeyEscape}} {
		repo := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}
		model := NewApp(context.Background(), app.NewStudentService(repo), nil)
		model, _ = updateApp(model, model.Init()())
		model, _ = updateApp(model, keypress('d'))
		model, cmd := updateApp(model, cancel)
		if model.students.deleteDialog != nil || cmd != nil || model.students.form != nil || repo.deleteCalls != 0 || len(model.students.students) != 1 {
			t.Fatal("отмена не должна удалять запись или открывать форму")
		}
	}
	repo := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}, deleteErr: errors.New("storage failed")}
	model := NewApp(context.Background(), app.NewStudentService(repo), nil)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('d'))
	model, remove := updateApp(model, keypress('y'))
	model, _ = updateApp(model, remove())
	if model.students.deleteDialog == nil || model.students.deleteDialog.deleting || len(model.students.students) != 1 || !strings.Contains(model.View().Content, "Не удалось удалить") {
		t.Fatal("ошибка должна оставить диалог и запись в списке")
	}
	repo.deleteErr = nil
	model, remove = updateApp(model, keypress('y'))
	model, _ = updateApp(model, remove())
	if model.students.deleteDialog != nil || len(model.students.students) != 0 || repo.deleteCalls != 2 {
		t.Fatal("повторное удаление после ошибки не удалось")
	}
}

func TestDeleteStudentUnavailable(t *testing.T) {
	model := NewApp(context.Background(), app.NewStudentService(&studentRepoStub{}), nil)
	model, _ = updateApp(model, keypress('d'))
	if model.students.deleteDialog != nil {
		t.Fatal("нельзя открыть диалог во время загрузки")
	}
	model, _ = updateApp(model, model.Init()())
	model, cmd := updateApp(model, keypress('d'))
	if model.students.deleteDialog != nil || cmd != nil {
		t.Fatal("нельзя открыть диалог в пустом списке")
	}
	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, keypress('d'))
	if model.students.deleteDialog != nil || model.students.form.name.Value() != "d" {
		t.Fatal("d внутри поля имени должен вводиться как обычная буква")
	}
}
