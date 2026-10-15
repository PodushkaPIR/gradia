package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type templateSavedMsg struct {
	template domain.Template
}

type templateSaveFailedMsg struct {
	err error
}

func saveTemplateCmd(ctx context.Context, service *app.TemplateService, id domain.TemplateID, name string) tea.Cmd {
	return func() tea.Msg {
		var template domain.Template
		var err error
		if id == 0 {
			template, err = service.Create(ctx, name)
		} else {
			template, err = service.Rename(ctx, id, name)
		}
		if err != nil {
			return routeToTemplates(templateSaveFailedMsg{err: err})
		}
		return routeToTemplates(templateSavedMsg{template: template})
	}
}
