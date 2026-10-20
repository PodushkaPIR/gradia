package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type TaskBankTaskID int64
type TaskCollectionID int64

type TaskBankTask struct {
	ID                   TaskBankTaskID
	Content, Subject     string
	Topics               []string
	Difficulty           TaskDifficulty
	CollectionIDs        []TaskCollectionID
	CreatedAt, UpdatedAt time.Time
}

type TaskBankInput struct {
	Content, Subject string
	Topics           []string
	Difficulty       TaskDifficulty
	CollectionIDs    []TaskCollectionID
}

type TaskBankFilter struct {
	Subject, Topic string
	CollectionID   TaskCollectionID
}

type TaskCollection struct {
	ID                   TaskCollectionID
	Name                 string
	TaskCount            int
	CreatedAt, UpdatedAt time.Time
}

const TaskBankNameMaxLength = 200

var (
	ErrTaskBankTaskNotFound       = errors.New("задача не найдена в банке")
	ErrTaskSubjectRequired        = errors.New("укажите предмет задачи")
	ErrTaskBankNameTooLong        = errors.New("название предмета или коллекции должно содержать не больше 200 символов")
	ErrTaskCollectionNotFound     = errors.New("коллекция не найдена")
	ErrTaskCollectionNameRequired = errors.New("введите название коллекции")
	ErrTaskCollectionNameExists   = errors.New("коллекция с таким названием уже существует")
)

func normalizeBankName(name string, required error) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "", required
	}
	if utf8.RuneCountInString(name) > TaskBankNameMaxLength {
		return "", ErrTaskBankNameTooLong
	}
	return name, nil
}

func NormalizeTaskCollectionName(name string) (string, error) {
	return normalizeBankName(name, ErrTaskCollectionNameRequired)
}

func NormalizeTaskBankInput(input TaskBankInput) (TaskBankInput, error) {
	basic, err := NormalizeTaskContent(TaskContentInput{Content: input.Content, Topics: input.Topics, Difficulty: input.Difficulty})
	if err != nil {
		return TaskBankInput{}, err
	}
	subject, err := normalizeBankName(input.Subject, ErrTaskSubjectRequired)
	if err != nil {
		return TaskBankInput{}, err
	}
	var ids []TaskCollectionID
	seen := make(map[TaskCollectionID]bool)
	for _, id := range input.CollectionIDs {
		if id <= 0 {
			return TaskBankInput{}, ErrTaskCollectionNotFound
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	slices.Sort(ids)
	return TaskBankInput{Content: basic.Content, Subject: subject, Topics: basic.Topics, Difficulty: basic.Difficulty, CollectionIDs: ids}, nil
}

func NormalizeTaskBankFilter(filter TaskBankFilter) (TaskBankFilter, error) {
	if filter.CollectionID < 0 {
		return TaskBankFilter{}, ErrTaskCollectionNotFound
	}
	filter.Subject = strings.Join(strings.Fields(filter.Subject), " ")
	if utf8.RuneCountInString(filter.Subject) > TaskBankNameMaxLength {
		return TaskBankFilter{}, ErrTaskBankNameTooLong
	}
	filter.Topic = strings.TrimSpace(filter.Topic)
	if filter.Topic != "" {
		var err error
		filter.Topic, err = NormalizeTaskTopic(filter.Topic)
		if err != nil {
			return TaskBankFilter{}, err
		}
	}
	return filter, nil
}
