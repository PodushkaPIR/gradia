package tui

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type studentRepoStub struct {
	students    []domain.Student
	err         error
	calls       int
	createCalls int
	createErr   error
	renameCalls int
	renameErr   error
	deleteCalls int
	deleteErr   error
}

func (r *studentRepoStub) ListStudents(context.Context) ([]domain.Student, error) {
	r.calls++
	return append([]domain.Student(nil), r.students...), r.err
}

func (r *studentRepoStub) CreateStudent(_ context.Context, student domain.Student) (domain.Student, error) {
	r.createCalls++
	if r.createErr != nil {
		return domain.Student{}, r.createErr
	}
	student.ID = domain.StudentID(len(r.students) + 1)
	r.students = append(r.students, student)
	return student, nil
}

func (r *studentRepoStub) RenameStudent(_ context.Context, id domain.StudentID, name string, updatedAt time.Time) (domain.Student, error) {
	r.renameCalls++
	if r.renameErr != nil {
		return domain.Student{}, r.renameErr
	}
	for i, student := range r.students {
		if student.ID == id {
			student.Name = name
			student.UpdatedAt = updatedAt
			r.students[i] = student
			sort.Slice(r.students, func(i, j int) bool { return r.students[i].Name < r.students[j].Name })
			return student, nil
		}
	}
	return domain.Student{}, domain.ErrStudentNotFound
}

func (r *studentRepoStub) DeleteStudent(_ context.Context, id domain.StudentID) error {
	r.deleteCalls++
	if r.deleteErr != nil {
		return r.deleteErr
	}
	for i, student := range r.students {
		if student.ID == id {
			r.students = slices.Delete(slices.Clone(r.students), i, i+1)
			return nil
		}
	}
	return domain.ErrStudentNotFound
}

func TestStudentsLoadAndRetry(t *testing.T) {
	repo := &studentRepoStub{err: errors.New("database unavailable")}
	model := NewApp(context.Background(), app.NewStudentService(repo), nil)
	cmd := model.Init()
	if repo.calls != 0 || !strings.Contains(model.View().Content, "Загрузка") {
		t.Fatal("Init должен вернуть команду без синхронного обращения к репозиторию")
	}

	updated, _ := model.Update(cmd())
	model = updated.(App)
	if model.students.loading || model.students.err != repo.err || !strings.Contains(model.View().Content, "Не удалось") {
		t.Fatal("ошибка загрузки не отображена")
	}

	repo.err = nil
	repo.students = []domain.Student{{ID: 1, Name: "Иван"}, {ID: 2, Name: "Анна"}}
	updated, cmd = model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	model = updated.(App)
	if !model.students.loading || cmd == nil || repo.calls != 1 {
		t.Fatal("повторная загрузка должна выполняться через Cmd")
	}
	// Пока запрос выполняется, повторный r не запускает ещё один запрос.
	_, duplicate := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if duplicate != nil {
		t.Fatal("запущена дублирующая загрузка")
	}
	updated, _ = model.Update(cmd())
	model = updated.(App)
	if model.students.loading || model.students.err != nil || !strings.Contains(model.View().Content, "> • Иван") {
		t.Fatal("список не отображён после успешной повторной загрузки")
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	model = updated.(App)
	if model.students.cursor != 1 {
		t.Fatal("курсор не переместился")
	}
	updated, _ = model.Update(studentsLoadedMsg{})
	model = updated.(App)
	if model.students.cursor != 0 || !strings.Contains(model.View().Content, "Список пока пуст") {
		t.Fatal("пустой список должен сбросить курсор")
	}
	_, quit := model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if quit == nil {
		t.Fatal("нет команды выхода")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("ожидалась команда выхода Bubble Tea")
	}
}
