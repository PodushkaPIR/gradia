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
	CreatedAt, UpdatedAt time.Time
}
