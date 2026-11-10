package tui

import (
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/domain"
)

type homeworkBankMsg struct {
	screenID uint64
	payload  tea.Msg
}
type homeworkBankCopiedMsg struct{ tasks []domain.HomeworkTask }
type homeworkBankFailedMsg struct{ err error }

func (m homeworkModel) openBankPicker() (homeworkModel, tea.Cmd) {
	m.bankPickerSeq++
	details, parent, screen := m.detailsID, m.screenID, m.bankPickerSeq
	wrap := func(msg tea.Msg) tea.Msg {
		return routeToHomework(details, parent, homeworkBankMsg{screenID: screen, payload: msg})
	}
	bank := taskBankModel{
		ctx:      m.ctx,
		service:  m.bankService,
		width:    m.width,
		height:   max(1, m.height-6),
		keys:     newTaskBankKeyMap(),
		readOnly: true,
		wrap:     wrap,
	}
	bank.keys.Mark = newShortcut("Space", "отметить", "Отметить / снять задачу для ДЗ; выбор сохраняется между фильтрами", "space")
	for _, binding := range []*shortcut{
		&bank.keys.Create, &bank.keys.CreateImage, &bank.keys.Edit, &bank.keys.Rename,
		&bank.keys.Attachments, &bank.keys.Export, &bank.keys.Import,
	} {
		binding.SetEnabled(false)
	}
	bank, cmd := bank.reload()
	m.bankPicker, m.bankCopyErr = &bank, nil
	return m, cmd
}

func (m homeworkModel) updateBankReply(msg tea.Msg) (homeworkModel, tea.Cmd) {
	switch msg := msg.(type) {
	case homeworkBankCopiedMsg:
		if m.selected == nil || len(msg.tasks) == 0 {
			return m, nil
		}
		for _, task := range msg.tasks {
			if task.HomeworkID != m.selected.ID {
				return m, nil
			}
		}
		m.bankPicker, m.bankCopyErr, m.saving = nil, nil, false
		m.tasks = append(slices.Clone(m.tasks), msg.tasks...)
		m.cursor = len(m.tasks) - 1
		return m.syncSummary(), nil
	case homeworkBankFailedMsg:
		m.saving, m.bankCopyErr = false, msg.err
		return m, nil
	}
	bank, cmd := m.bankPicker.Update(msg)
	m.bankPicker = &bank
	return m, cmd
}

func (m homeworkModel) updateBankPicker(msg tea.Msg) (homeworkModel, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		size.Height = m.height + 2
		msg = size
	}
	if m.saving {
		if _, ok := msg.(tea.WindowSizeMsg); !ok {
			return m, nil
		}
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		bank := m.bankPicker
		if key.Matches(pressed, m.keys.Back.Binding) && bank.atRoot() {
			m.bankPicker, m.bankCopyErr = nil, nil
			return m, nil
		}
		if key.Matches(pressed, m.keys.AssignBank.Binding) {
			if bank.loading || bank.err != nil {
				return m, nil
			}
			sources := slices.Clone(bank.marked)
			if len(sources) == 0 {
				source, ok := bank.selectedTaskID()
				if !ok {
					return m, nil
				}
				sources = []domain.TaskBankTaskID{source}
			}
			m.saving, m.bankCopyErr = true, nil
			ctx, service, student, homework, route := m.ctx, m.service, m.student, m.selected.ID, bank.route
			return m, func() tea.Msg {
				tasks, err := service.AddManyFromBank(ctx, student, homework, sources)
				if err != nil {
					return route(homeworkBankFailedMsg{err: err})
				}
				return route(homeworkBankCopiedMsg{tasks: tasks})
			}
		}
	}
	bank, cmd := m.bankPicker.Update(msg)
	m.bankPicker = &bank
	return m, cmd
}
