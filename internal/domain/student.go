package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const StudentNameMaxLength = 200

var (
	ErrStudentNameRequired = errors.New("введите имя ученика")
	ErrStudentNameTooLong  = errors.New("имя ученика должно содержать не больше 200 символов")
	ErrStudentNotFound     = errors.New("ученик не найден")
)

type StudentID int64

type Student struct {
	ID        StudentID
	Name      string
	Grade     *string
	Notes     string // Markdown-текст.
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewStudent(name string) (Student, error) {
	name, err := NormalizeStudentName(name)
	if err != nil {
		return Student{}, err
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	return Student{Name: name, CreatedAt: now, UpdatedAt: now}, nil
}

func NormalizeStudentName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrStudentNameRequired
	}
	if utf8.RuneCountInString(name) > StudentNameMaxLength {
		return "", ErrStudentNameTooLong
	}
	return name, nil
}
