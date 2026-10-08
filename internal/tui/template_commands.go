package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type templateCreatedMsg struct {
	template domain.Template
}

type templateCreateFailedMsg struct {
	err error
}

func createTemplateCmd(ctx context.Context, service *app.TemplateService, name string) tea.Cmd {
	return func() tea.Msg {
		template, err := service.Create(ctx, name)
		if err != nil {
			return routeToTemplates(templateCreateFailedMsg{err: err})
		}
		return routeToTemplates(templateCreatedMsg{template: template})
	}
}
