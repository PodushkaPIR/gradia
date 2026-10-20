package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type attachmentsLoadedMsg struct{ items []domain.HomeworkAttachment }
type attachmentsLoadFailedMsg struct{ err error }
type attachmentAddedMsg struct{ item domain.HomeworkAttachment }
type attachmentImportFailedMsg struct{ err error }
type attachmentExportedMsg struct {
	id   domain.HomeworkAttachmentID
	path string
}
type attachmentExportFailedMsg struct{ err error }
type attachmentOpenedMsg struct {
	path string
	err  error
}

type homeworkAttachmentsModel struct {
	ctx                                   context.Context
	service                               *app.HomeworkService
	student                               domain.StudentID
	homework                              domain.HomeworkID
	task                                  domain.HomeworkTaskID
	detailsID, homeworkScreenID, screenID uint64
	taskContent                           string
	items                                 []domain.HomeworkAttachment
	cursor, width, height                 int
	loading, saving                       bool
	err, saveErr, openErr                 error
	form                                  *attachmentForm
	openedID                              domain.HomeworkAttachmentID
	path                                  string
	keys                                  attachmentsKeyMap
}

func (m homeworkAttachmentsModel) reload() (homeworkAttachmentsModel, tea.Cmd) {
	if m.loading || m.saving || m.form != nil {
		return m, nil
	}
	m.loading, m.err, m.saveErr = true, nil, nil
	ctx, service, student, homework, task, details, parent, screen := m.ctx, m.service, m.student, m.homework, m.task, m.detailsID, m.homeworkScreenID, m.screenID
	return m, func() tea.Msg {
		items, err := service.Attachments(ctx, student, homework, task)
		if err != nil {
			return routeToHomeworkAttachments(details, parent, screen, attachmentsLoadFailedMsg{err: err})
		}
		return routeToHomeworkAttachments(details, parent, screen, attachmentsLoadedMsg{items: items})
	}
}

func (m homeworkAttachmentsModel) Update(msg tea.Msg) (homeworkAttachmentsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case attachmentsLoadedMsg:
		m.items, m.loading, m.err = slices.Clone(msg.items), false, nil
		m.cursor = max(0, min(m.cursor, len(m.items)-1))
		return m, nil
	case attachmentsLoadFailedMsg:
		m.loading, m.err = false, msg.err
		return m, nil
	case attachmentAddedMsg:
		if msg.item.TaskID != m.task {
			return m, nil
		}
		m.form = nil
		m.items = append(slices.Clone(m.items), msg.item)
		m.cursor = len(m.items) - 1
		return m, nil
	case attachmentImportFailedMsg:
		if m.form == nil {
			return m, nil
		}
		form, cmd := m.form.fail(msg.err)
		m.form = &form
		return m, cmd
	case attachmentExportedMsg:
		m.saving, m.saveErr, m.openErr, m.openedID, m.path = false, nil, nil, msg.id, msg.path
		ctx, service, path, details, parent, screen := m.ctx, m.service, msg.path, m.detailsID, m.homeworkScreenID, m.screenID
		return m, func() tea.Msg {
			return routeToHomeworkAttachments(details, parent, screen, attachmentOpenedMsg{path: path, err: service.OpenAttachment(ctx, path)})
		}
	case attachmentExportFailedMsg:
		m.saving, m.saveErr = false, msg.err
		return m, nil
	case attachmentOpenedMsg:
		if msg.path == m.path {
			m.openErr = msg.err
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, max(1, msg.Height-12)
	}
	if m.form != nil {
		form, action, cmd := m.form.Update(msg)
		m.form = &form
		switch action {
		case formCancel:
			m.form = nil
			return m, nil
		case formSubmit:
			ctx, service, student, homework, task, path, pages, details, parent, screen := m.ctx, m.service, m.student, m.homework, m.task, form.path.Value(), form.pages.Value(), m.detailsID, m.homeworkScreenID, m.screenID
			return m, func() tea.Msg {
				item, err := service.AttachFile(ctx, student, homework, task, path, pages)
				if err != nil {
					return routeToHomeworkAttachments(details, parent, screen, attachmentImportFailedMsg{err: err})
				}
				return routeToHomeworkAttachments(details, parent, screen, attachmentAddedMsg{item: item})
			}
		}
		return m, cmd
	}
	if m.saving {
		return m, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Create.Binding):
			if !m.loading && m.err == nil {
				form, cmd := newAttachmentForm(m.width).focusField(0)
				m.form = &form
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Open.Binding):
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(m.items) {
				id := m.items[m.cursor].ID
				m.saving, m.saveErr, m.openErr, m.path = true, nil, nil, ""
				ctx, service, student, homework, task, details, parent, screen := m.ctx, m.service, m.student, m.homework, m.task, m.detailsID, m.homeworkScreenID, m.screenID
				return m, func() tea.Msg {
					path, err := service.ExportAttachment(ctx, student, homework, task, id)
					if err != nil {
						return routeToHomeworkAttachments(details, parent, screen, attachmentExportFailedMsg{err: err})
					}
					return routeToHomeworkAttachments(details, parent, screen, attachmentExportedMsg{id: id, path: path})
				}
			}
		case key.Matches(pressed, m.keys.Reload.Binding):
			return m.reload()
		case key.Matches(pressed, m.keys.Down.Binding):
			m.cursor = max(0, min(m.cursor+1, len(m.items)-1))
		case key.Matches(pressed, m.keys.Up.Binding):
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m homeworkAttachmentsModel) InputMode() inputMode {
	if m.form != nil {
		return m.form.InputMode()
	}
	if m.saving {
		return inputBusy
	}
	return inputNavigation
}

func (m homeworkAttachmentsModel) View() string {
	line := taskContentPreview(m.taskContent, m.width-10)
	title := "Вложения задачи — " + line + "\n\n"
	if m.form != nil {
		return title + m.form.View()
	}
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить вложения. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки."
	}
	if len(m.items) == 0 {
		return title + "Файлов пока нет. Нажмите " + m.keys.Create.Help().Key + " для прикрепления."
	}
	var text strings.Builder
	text.WriteString(title)
	height := max(1, m.height-6)
	start := max(0, m.cursor-height+1)
	for i := start; i < min(len(m.items), start+height); i++ {
		if i > start {
			text.WriteByte('\n')
		}
		if i == m.cursor {
			text.WriteString("> ")
		} else {
			text.WriteString("  ")
		}
		item := m.items[i]
		fmt.Fprintf(&text, "%s (%.1f КБ)", item.Name, float64(item.Size)/1024)
		if item.Pages != "" {
			text.WriteString(" — стр. " + item.Pages)
		}
	}
	if m.saving {
		text.WriteString("\n\nПодготовка файла…")
	}
	if m.saveErr != nil {
		text.WriteString("\n\nНе удалось подготовить файл. Повторите Enter.")
	}
	if m.cursor >= 0 && m.cursor < len(m.items) && m.items[m.cursor].ID == m.openedID && m.path != "" {
		text.WriteString("\n\nФайл сохранён: " + m.path)
		if m.openErr != nil {
			text.WriteString("\nНе удалось открыть просмотрщик. Откройте файл вручную.")
		}
	}
	return text.String()
}

func (m homeworkAttachmentsModel) ShortHelp() string {
	if m.form != nil {
		return m.form.ShortHelp()
	}
	if m.saving {
		return ""
	}
	return shortHelp(m.keys.Open, m.keys.Create, m.keys.Back, m.keys.Help)
}

func (m homeworkAttachmentsModel) FullHelp() (string, []shortcut) {
	keys := m.keys
	return "Вложения задачи", []shortcut{keys.helpBinding(), keys.Open, keys.Create, keys.Reload, keys.Back, keys.Help, keys.Quit}
}
