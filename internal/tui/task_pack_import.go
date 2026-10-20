package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
)

type taskPackImportedMsg struct{ result app.TaskPackImportResult }
type taskPackImportFailedMsg struct{ err error }

func (m taskBankModel) updatePackImport(msg tea.Msg) (taskBankModel, tea.Cmd) {
	form, action, cmd := m.importForm.Update(msg)
	m.importForm = &form
	switch action {
	case formCancel:
		m.importForm = nil
		return m, nil
	case formSubmit:
		ctx, service, path, screen := m.ctx, m.service, form.name.Value(), m.screenID
		return m, func() tea.Msg {
			result, err := service.ImportPack(ctx, path)
			if err != nil {
				return routeToTaskBank(screen, taskPackImportFailedMsg{err: err})
			}
			return routeToTaskBank(screen, taskPackImportedMsg{result: result})
		}
	}
	return m, cmd
}
