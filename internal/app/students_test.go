package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type studentRepositoryStub struct {
	created []domain.Student
	renamed []domain.Student
	deleted []domain.StudentID
	err     error
}

func (r *studentRepositoryStub) ListStudents(context.Context) ([]domain.Student, error) {
	return nil, nil
}

func (r *studentRepositoryStub) CreateStudent(_ context.Context, student domain.Student) (domain.Student, error) {
	r.created = append(r.created, student)
	student.ID = 1
	return student, r.err
}

func (r *studentRepositoryStub) RenameStudent(_ context.Context, id domain.StudentID, name string, updatedAt time.Time) (domain.Student, error) {
	student := domain.Student{ID: id, Name: name, UpdatedAt: updatedAt}
	r.renamed = append(r.renamed, student)
	return student, r.err
}

func (r *studentRepositoryStub) DeleteStudent(_ context.Context, id domain.StudentID) error {
	r.deleted = append(r.deleted, id)
	return r.err
}

func TestDeleteStudent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		id      domain.StudentID
		repoErr error
		wantErr error
		calls   int
	}{
		{name: "valid", id: 5, calls: 1},
		{name: "zero ID", id: 0, wantErr: domain.ErrStudentNotFound},
		{name: "negative ID", id: -1, wantErr: domain.ErrStudentNotFound},
		{name: "missing", id: 5, repoErr: domain.ErrStudentNotFound, wantErr: domain.ErrStudentNotFound, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &studentRepositoryStub{err: tc.repoErr}
			err := NewStudentService(repo).Delete(context.Background(), tc.id)
			if !errors.Is(err, tc.wantErr) || len(repo.deleted) != tc.calls {
				t.Fatalf("err=%v, calls=%d", err, len(repo.deleted))
			}
			if tc.calls == 1 && repo.deleted[0] != tc.id {
				t.Fatal("в репозиторий передан другой ID")
			}
		})
	}
	want := errors.New("storage failed")
	err := NewStudentService(&studentRepositoryStub{err: want}).Delete(context.Background(), 5)
	if !errors.Is(err, want) {
		t.Fatalf("ошибка репозитория потеряна: %v", err)
	}
}

func TestCreateStudentValidation(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		err               error
	}{
		{name: "trim edges", input: "  Иван Петров  ", want: "Иван Петров"},
		{name: "empty", input: "", err: domain.ErrStudentNameRequired},
		{name: "whitespace", input: " \t\n ", err: domain.ErrStudentNameRequired},
		{name: "too long", input: strings.Repeat("Я", 201), err: domain.ErrStudentNameTooLong},
		{name: "Unicode limit counts characters", input: strings.Repeat("Я", 200), want: strings.Repeat("Я", 200)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &studentRepositoryStub{}
			student, err := NewStudentService(repo).Create(context.Background(), tc.input)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v, want=%v", err, tc.err)
			}
			if tc.err != nil {
				if len(repo.created) != 0 {
					t.Fatal("невалидные данные переданы в репозиторий")
				}
				return
			}
			if len(repo.created) != 1 || student.ID == 0 || student.Name != tc.want {
				t.Fatalf("student=%+v, writes=%d", student, len(repo.created))
			}
			if student.CreatedAt.IsZero() || !student.CreatedAt.Equal(student.UpdatedAt) {
				t.Fatalf("неверные timestamps: %+v", student)
			}
		})
	}
}

func TestCreateStudentRepositoryError(t *testing.T) {
	want := errors.New("storage failed")
	repo := &studentRepositoryStub{err: want}
	_, err := NewStudentService(repo).Create(context.Background(), "Иван")
	if !errors.Is(err, want) {
		t.Fatalf("ошибка репозитория потеряна: %v", err)
	}
}

func TestRenameStudentValidation(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		id          domain.StudentID
		err         error
	}{
		{name: "valid", id: 5, input: "  Иван Петров  "},
		{name: "empty", id: 5, input: "  ", err: domain.ErrStudentNameRequired},
		{name: "too long", id: 5, input: strings.Repeat("Я", 201), err: domain.ErrStudentNameTooLong},
		{name: "invalid ID", id: 0, input: "Иван", err: domain.ErrStudentNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &studentRepositoryStub{}
			student, err := NewStudentService(repo).Rename(context.Background(), tc.id, tc.input)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v, want=%v", err, tc.err)
			}
			if tc.err != nil {
				if len(repo.renamed) != 0 {
					t.Fatal("невалидные данные переданы в репозиторий")
				}
				return
			}
			if student.ID != tc.id || student.Name != "Иван Петров" || student.UpdatedAt.IsZero() || len(repo.renamed) != 1 || len(repo.created) != 0 {
				t.Fatalf("rename: student=%+v, repo=%+v", student, repo)
			}
		})
	}
}

func TestRenameStudentRepositoryError(t *testing.T) {
	for _, want := range []error{errors.New("storage failed"), domain.ErrStudentNotFound} {
		repo := &studentRepositoryStub{err: want}
		_, err := NewStudentService(repo).Rename(context.Background(), 5, "Иван")
		if !errors.Is(err, want) {
			t.Fatalf("ошибка репозитория потеряна: %v", err)
		}
	}
}
