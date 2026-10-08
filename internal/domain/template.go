package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TemplateNameMaxLength = 200

var (
	ErrTemplateNameRequired = errors.New("введите название шаблона")
	ErrTemplateNameTooLong  = errors.New("название шаблона должно содержать не больше 200 символов")
)

type TemplateID int64

type Template struct {
	ID          TemplateID
	Name        string
	Description string // Markdown-текст.
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewTemplate(name string) (Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, ErrTemplateNameRequired
	}
	if utf8.RuneCountInString(name) > TemplateNameMaxLength {
		return Template{}, ErrTemplateNameTooLong
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	return Template{Name: name, CreatedAt: now, UpdatedAt: now}, nil
}
