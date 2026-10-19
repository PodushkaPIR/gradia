package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrRouteNotFound     = errors.New("маршрут не найден")
	ErrStudentInUse      = errors.New("у ученика есть связанные данные; удаление недоступно")
	ErrRouteNameRequired = errors.New("введите название учебного плана")
	ErrRouteNameTooLong  = errors.New("название учебного плана должно содержать не больше 200 символов")
)

const RouteNameMaxLength = 200

func NormalizeRouteName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrRouteNameRequired
	}
	if utf8.RuneCountInString(name) > RouteNameMaxLength {
		return "", ErrRouteNameTooLong
	}
	return name, nil
}

type RouteID int64
type RouteTopicID int64

var ErrTopicsNotSiblings = errors.New("переставлять можно только темы одного родителя")

// RouteTopicPosition is a persisted position within a topic's sibling group.
type RouteTopicPosition struct {
	TopicID  RouteTopicID
	Position int
}

type Route struct {
	ID                   RouteID
	StudentID            StudentID
	SourceTemplateID     *TemplateID
	Name, Description    string
	CreatedAt, UpdatedAt time.Time
}

type RouteTopic struct {
	ID                   RouteTopicID
	RouteID              RouteID
	ParentID             *RouteTopicID
	Name                 string
	Notes                string // Markdown-текст личной заметки по теме.
	Position             int
	Status               ProgressStatus
	Revision             bool // Освоенная тема требует повторения; статус освоения сохраняется.
	Excluded             bool // Личное исключение темы; исключение предка не меняет эту отметку.
	CreatedAt, UpdatedAt time.Time
}

var ErrRevisionRequiresMastered = errors.New("повторение доступно только для освоенной темы")

// RouteProgressChange хранит фактический переход, включая автоматическое снятие повторения.
type RouteProgressChange struct {
	ID                            int64
	TopicID                       RouteTopicID
	BeforeStatus, AfterStatus     ProgressStatus
	BeforeRevision, AfterRevision bool
	CreatedAt                     time.Time
}

type RouteTopicExclusionChange struct {
	ID                            int64
	TopicID                       RouteTopicID
	BeforeExcluded, AfterExcluded bool
	CreatedAt                     time.Time
}

func NewRouteTopic(routeID RouteID, name string) (RouteTopic, error) {
	if routeID <= 0 {
		return RouteTopic{}, ErrRouteNotFound
	}
	name, err := NormalizeTopicName(name)
	if err != nil {
		return RouteTopic{}, err
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	return RouteTopic{RouteID: routeID, Name: name, Status: ProgressNotStarted, CreatedAt: now, UpdatedAt: now}, nil
}

// RouteTopicPath returns the path from a top-level topic to the requested topic.
func RouteTopicPath(topics []RouteTopic, id RouteTopicID) ([]RouteTopic, error) {
	byID := make(map[RouteTopicID]RouteTopic, len(topics))
	for _, topic := range topics {
		byID[topic.ID] = topic
	}
	var path []RouteTopic
	for id != 0 {
		topic, ok := byID[id]
		if !ok || (len(path) > 0 && topic.RouteID != path[0].RouteID) {
			return nil, ErrRouteTopicNotFound
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
