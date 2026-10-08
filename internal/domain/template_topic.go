package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TopicNameMaxLength = 200

const TopicMaxDepth = 5

var (
	ErrTopicNameRequired  = errors.New("введите название темы")
	ErrTopicNameTooLong   = errors.New("название темы должно содержать не больше 200 символов")
	ErrTemplateNotFound   = errors.New("шаблон не найден")
	ErrTopicNotFound      = errors.New("тема не найдена в этом шаблоне")
	ErrTopicDepthExceeded = errors.New("допустимо не больше 5 уровней тем")
)

type TemplateTopicID int64

type TemplateTopic struct {
	ID         TemplateTopicID
	TemplateID TemplateID
	ParentID   *TemplateTopicID
	Name       string
	Position   int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TemplateTopicPath возвращает путь от темы верхнего уровня до выбранной темы.
func TemplateTopicPath(topics []TemplateTopic, id TemplateTopicID) ([]TemplateTopic, error) {
	byID := make(map[TemplateTopicID]TemplateTopic, len(topics))
	for _, topic := range topics {
		byID[topic.ID] = topic
	}
	var path []TemplateTopic
	for id != 0 {
		topic, ok := byID[id]
		if !ok || (len(path) > 0 && topic.TemplateID != path[0].TemplateID) {
			return nil, ErrTopicNotFound
		}
		if len(path) >= TopicMaxDepth {
			return nil, ErrTopicDepthExceeded
		}
		path = append(path, topic)
		id = 0
		if topic.ParentID != nil {
			id = *topic.ParentID
		}
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, nil
}

func NewTemplateTopic(templateID TemplateID, name string) (TemplateTopic, error) {
	if templateID <= 0 {
		return TemplateTopic{}, ErrTemplateNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return TemplateTopic{}, ErrTopicNameRequired
	}
	if utf8.RuneCountInString(name) > TopicNameMaxLength {
		return TemplateTopic{}, ErrTopicNameTooLong
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	return TemplateTopic{TemplateID: templateID, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}
