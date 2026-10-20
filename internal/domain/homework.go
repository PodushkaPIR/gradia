package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type HomeworkID int64
type HomeworkTaskID int64

const HomeworkNameMaxLength = 200

var (
	ErrHomeworkNotFound     = errors.New("домашнее задание не найдено у этого ученика")
	ErrHomeworkTaskNotFound = errors.New("задача не найдена в этом домашнем задании")
	ErrHomeworkNameRequired = errors.New("введите название домашнего задания")
	ErrHomeworkNameTooLong  = errors.New("название домашнего задания должно содержать не больше 200 символов")
)

type Homework struct {
	ID                         HomeworkID
	StudentID                  StudentID
	Name                       string
	TotalTasks, CompletedTasks int // Derived from tasks, never persisted as Homework state.
	CreatedAt, UpdatedAt       time.Time
}

type HomeworkTask struct {
	ID                   HomeworkTaskID
	HomeworkID           HomeworkID
	Content              string   // Markdown or an external reference, without fetching its contents.
	Topics               []string // Independent classification labels, not personal Learning Plan IDs.
	Difficulty           TaskDifficulty
	Completed            bool
	Position             int
	CreatedAt, UpdatedAt time.Time
}

type HomeworkTaskInput = TaskContentInput

func NormalizeHomeworkName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrHomeworkNameRequired
	}
	if utf8.RuneCountInString(name) > HomeworkNameMaxLength {
		return "", ErrHomeworkNameTooLong
	}
	return name, nil
}

func NormalizeHomeworkTask(input HomeworkTaskInput) (HomeworkTaskInput, error) {
	return NormalizeTaskContent(input)
}
