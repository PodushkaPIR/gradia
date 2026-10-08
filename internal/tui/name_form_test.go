package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNameFormReturnsActionsWithoutBusinessCommands(t *testing.T) {
	form := newNameForm(nameFormConfig{Title: "Форма", Label: "Название", Limit: 200, Width: 80})
	form.Init()
	for _, pressed := range []rune{'q', '?', 'n'} {
		form, _, _ = form.Update(keypress(pressed))
	}
	if form.name.Value() != "q?n" || !form.name.Focused() {
		t.Fatal("форма должна управлять только вводом и фокусом")
	}
	form, action, cmd := form.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if action != formSubmit || cmd != nil || form.InputMode() != inputBusy {
		t.Fatal("Enter должен возвращать намерение сохранить, без бизнес-команды")
	}
	for _, pressed := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEscape}, keypress('x')} {
		var action formAction
		form, action, cmd = form.Update(pressed)
		if action != formNoAction || cmd != nil || form.name.Value() != "q?n" {
			t.Fatal("busy должен блокировать повторную отправку, отмену и изменение поля")
		}
	}
	form, _, _ = form.Update(tea.WindowSizeMsg{Width: 30, Height: 20})
	if form.name.Width() != 26 {
		t.Fatal("busy не должен блокировать изменение размера поля")
	}
	want := errors.New("operation failed")
	form, _ = form.fail(want, "Не удалось сохранить")
	if form.InputMode() != inputText || !errors.Is(form.err, want) || form.name.Value() != "q?n" || !strings.Contains(form.View(), "Не удалось сохранить") {
		t.Fatal("результат ошибки должен сохранять ввод и возвращать фокус")
	}
	form, _, _ = form.Update(keypress('x'))
	if form.err != nil || strings.Contains(form.View(), "Не удалось сохранить") {
		t.Fatal("редактирование должно убирать прежнюю ошибку")
	}
	_, action, cmd = form.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if action != formCancel || cmd != nil {
		t.Fatal("Esc должен возвращать намерение отмены, без побочных эффектов")
	}
}
