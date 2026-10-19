package domain

import (
	"errors"
	"time"
)

var (
	ErrRouteNotFound = errors.New("маршрут не найден")
	ErrStudentInUse  = errors.New("у ученика есть связанные данные; удаление недоступно")
)

type RouteID int64
type RouteTopicID int64

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
