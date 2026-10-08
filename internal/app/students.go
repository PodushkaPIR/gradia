package app

import (
	"context"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

type StudentRepository interface {
	ListStudents(context.Context) ([]domain.Student, error)
	CreateStudent(context.Context, domain.Student) (domain.Student, error)
	RenameStudent(context.Context, domain.StudentID, string, time.Time) (domain.Student, error)
	DeleteStudent(context.Context, domain.StudentID) error
}

type StudentService struct {
	repo StudentRepository
}

func NewStudentService(repo StudentRepository) *StudentService {
	return &StudentService{repo: repo}
}

func (s *StudentService) List(ctx context.Context) ([]domain.Student, error) {
	return s.repo.ListStudents(ctx)
}

func (s *StudentService) Create(ctx context.Context, name string) (domain.Student, error) {
	student, err := domain.NewStudent(name)
	if err != nil {
		return domain.Student{}, err
	}
	return s.repo.CreateStudent(ctx, student)
}

func (s *StudentService) Rename(ctx context.Context, id domain.StudentID, name string) (domain.Student, error) {
	if id <= 0 {
		return domain.Student{}, domain.ErrStudentNotFound
	}
	name, err := domain.NormalizeStudentName(name)
	if err != nil {
		return domain.Student{}, err
	}
	return s.repo.RenameStudent(ctx, id, name, time.Now().UTC().Truncate(time.Millisecond))
}

func (s *StudentService) Delete(ctx context.Context, id domain.StudentID) error {
	if id <= 0 {
		return domain.ErrStudentNotFound
	}
	return s.repo.DeleteStudent(ctx, id)
}
