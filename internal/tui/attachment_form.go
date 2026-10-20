package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"errors"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type attachmentForm struct {
	path, pages textinput.Model
	focus       int
	saving      bool
	err         string
	keys        attachmentFormKeyMap
}

func newAttachmentForm(width int) attachmentForm {
	path, pages := textinput.New(), textinput.New()
	path.Placeholder = "Полный путь или ~/Downloads/файл.pdf"
	pages.Placeholder = "Необязательно: 2 или 2-5"
	m := attachmentForm{path: path, pages: pages, keys: newAttachmentFormKeyMap()}
	return m.resize(width)
}

func (m attachmentForm) resize(width int) attachmentForm {
	m.path.SetWidth(max(1, min(70, width-4)))
	m.pages.SetWidth(max(1, min(30, width-4)))
	return m
}

func (m attachmentForm) focusField(field int) (attachmentForm, tea.Cmd) {
	m.focus = (field + 2) % 2
	m.path.Blur()
	m.pages.Blur()
	if m.focus == 0 {
		return m, m.path.Focus()
	}
	return m, m.pages.Focus()
}

func (m attachmentForm) Update(msg tea.Msg) (attachmentForm, formAction, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m = m.resize(size.Width)
	}
	if m.saving {
		return m, formNoAction, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Cancel.Binding):
			return m, formCancel, nil
		case key.Matches(pressed, m.keys.Save.Binding):
			m.saving, m.err = true, ""
			return m, formSubmit, nil
		case key.Matches(pressed, m.keys.Next.Binding):
			updated, cmd := m.focusField(m.focus + 1)
			return updated, formNoAction, cmd
		case key.Matches(pressed, m.keys.Previous.Binding):
			updated, cmd := m.focusField(m.focus - 1)
			return updated, formNoAction, cmd
		}
	}
	var cmd tea.Cmd
	if m.focus == 0 {
		before := m.path.Value()
		m.path, cmd = m.path.Update(msg)
		if m.path.Value() != before {
			m.err = ""
		}
	} else {
		before := m.pages.Value()
		m.pages, cmd = m.pages.Update(msg)
		if m.pages.Value() != before {
			m.err = ""
		}
	}
	return m, formNoAction, cmd
}

func (m attachmentForm) fail(err error) (attachmentForm, tea.Cmd) {
	m.saving, m.err = false, attachmentError(err, "Не удалось прикрепить файл. Проверьте путь и повторите Ctrl+S.")
	return m.focusField(m.focus)
}

func attachmentError(err error, fallback string) string {
	for _, known := range []error{domain.ErrAttachmentType, domain.ErrAttachmentPages, domain.ErrAttachmentPagesOnlyPDF, domain.ErrHomeworkNotFound, domain.ErrHomeworkTaskNotFound, domain.ErrHomeworkAttachmentNotFound, domain.ErrTaskBankTaskNotFound, domain.ErrTaskBankAttachmentNotFound, app.ErrAttachmentFilesUnavailable, app.ErrAttachmentPathRequired, app.ErrAttachmentRegularFileRequired} {
		if errors.Is(err, known) {
			return err.Error()
		}
	}
	return fallback
}

func (m attachmentForm) InputMode() inputMode {
	if m.saving {
		return inputBusy
	}
	return inputText
}
func (m attachmentForm) ShortHelp() string {
	if m.saving {
		return ""
	}
	return shortHelp(m.keys.Save, m.keys.Next, m.keys.Previous, m.keys.Cancel)
}
func (m attachmentForm) View() string {
	text := "Прикрепить оригинальный файл\n\nПуть к изображению или PDF:\n" + m.path.View() + "\n\nСтраница / диапазон PDF:\n" + m.pages.View()
	if m.saving {
		text += "\n\nСохранение…"
	}
	if m.err != "" {
		text += "\n\n" + m.err
	}
	return text
}
