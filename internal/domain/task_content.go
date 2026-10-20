package domain

import (
	"errors"
	"strings"
)

type TaskDifficulty string

const (
	DifficultyEasy   TaskDifficulty = "easy"
	DifficultyMedium TaskDifficulty = "medium"
	DifficultyHard   TaskDifficulty = "hard"
)

var (
	ErrTaskContentRequired   = errors.New("введите текст задачи или ссылку")
	ErrTaskTopicsRequired    = errors.New("укажите хотя бы одну тему задачи")
	ErrInvalidTaskDifficulty = errors.New("выберите сложность: лёгкая, средняя или сложная")
)

type TaskContentInput struct {
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

func NormalizeTaskContent(input TaskContentInput) (TaskContentInput, error) {
	input.Content = normalizeTaskText(input.Content)
	if strings.TrimSpace(input.Content) == "" {
		return TaskContentInput{}, ErrTaskContentRequired
	}
	if !input.Difficulty.Valid() {
		return TaskContentInput{}, ErrInvalidTaskDifficulty
	}
	topics := make([]string, 0, len(input.Topics))
	seen := make(map[string]bool)
	for _, name := range input.Topics {
		if strings.TrimSpace(name) == "" {
			continue
		}
		name, err := NormalizeTaskTopic(name)
		if err != nil {
			return TaskContentInput{}, err
		}
		if !seen[name] {
			topics = append(topics, name)
			seen[name] = true
		}
	}
	if len(topics) == 0 {
		return TaskContentInput{}, ErrTaskTopicsRequired
	}
	input.Topics = topics
	return input, nil
}

func NormalizeTaskTopic(name string) (string, error) {
	return NormalizeTopicName(normalizeTaskText(name))
}

// Use canonical line endings for stable editor and service round trips.
func normalizeTaskText(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}
