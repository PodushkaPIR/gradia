package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type HomeworkID int64
type HomeworkTaskID int64
type TaskDifficulty string

const (
	DifficultyEasy        TaskDifficulty = "easy"
	DifficultyMedium      TaskDifficulty = "medium"
	DifficultyHard        TaskDifficulty = "hard"
	HomeworkNameMaxLength                = 200
)

var (
	ErrHomeworkNotFound      = errors.New("домашнее задание не найдено у этого ученика")
	ErrHomeworkTaskNotFound  = errors.New("задача не найдена в этом домашнем задании")
	ErrHomeworkNameRequired  = errors.New("введите название домашнего задания")
	ErrHomeworkNameTooLong   = errors.New("название домашнего задания должно содержать не больше 200 символов")
	ErrTaskContentRequired   = errors.New("введите текст задачи или ссылку")
	ErrTaskTopicsRequired    = errors.New("укажите хотя бы одну тему задачи")
	ErrInvalidTaskDifficulty = errors.New("выберите сложность: лёгкая, средняя или сложная")
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

type HomeworkTaskInput struct {
	Content    string
	Topics     []string
	Difficulty TaskDifficulty
}

func (d TaskDifficulty) Valid() bool {
	return d == DifficultyEasy || d == DifficultyMedium || d == DifficultyHard
}

func (d TaskDifficulty) Label() string {
	switch d {
	case DifficultyMedium:
		return "средняя"
	case DifficultyHard:
		return "сложная"
	default:
		return "лёгкая"
	}
}

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
	input.Content = normalizeHomeworkText(input.Content)
	if strings.TrimSpace(input.Content) == "" {
		return HomeworkTaskInput{}, ErrTaskContentRequired
	}
	if !input.Difficulty.Valid() {
		return HomeworkTaskInput{}, ErrInvalidTaskDifficulty
	}
	topics := make([]string, 0, len(input.Topics))
	seen := make(map[string]bool)
	for _, name := range input.Topics {
		name = normalizeHomeworkText(name)
		if strings.TrimSpace(name) == "" {
			continue
		}
		name, err := NormalizeTopicName(name)
		if err != nil {
			return HomeworkTaskInput{}, err
		}
		if !seen[name] {
			topics = append(topics, name)
			seen[name] = true
		}
	}
	if len(topics) == 0 {
		return HomeworkTaskInput{}, ErrTaskTopicsRequired
	}
	input.Topics = topics
	return input, nil
}

// Use canonical line endings so editor and service round trips agree across platforms.
func normalizeHomeworkText(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}
