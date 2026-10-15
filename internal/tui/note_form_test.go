package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNoteEditorMultilineAndActions(t *testing.T) {
	form := newNoteForm("Циклы", "", 80, 8)
	form.Init()
	for _, pressed := range []tea.KeyPressMsg{keypress('q'), keypress('?'), {Code: tea.KeyEnter}, keypress('x')} {
		form, _, _ = form.Update(pressed)
	}
	if form.text.Value() != "q?\nx" {
		t.Fatalf("Enter должен вставлять новую строку, q и ? — символы: %q", form.text.Value())
	}
	form, action, cmd := form.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if action != formSubmit || cmd != nil || form.InputMode() != inputBusy {
		t.Fatal("Ctrl+S должен возвращать намерение сохранения")
	}
	form, action, cmd = form.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if action != formNoAction || cmd != nil {
		t.Fatal("нельзя отменить выполняющееся сохранение")
	}
	form, _ = form.fail(errors.New("write failed"))
	if form.text.Value() != "q?\nx" || form.InputMode() != inputText || !strings.Contains(form.View(), "Не удалось сохранить") {
		t.Fatal("ошибка должна оставлять текст в редакторе")
	}
	_, action, cmd = form.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if action != formCancel || cmd != nil {
		t.Fatal("Esc должен отменять редактирование без сохранения")
	}
}
