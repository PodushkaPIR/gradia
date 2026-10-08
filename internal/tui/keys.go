package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
)

// shortcut хранит привязку, короткую подпись и полное описание одного действия.
type shortcut struct {
	key.Binding
	description string
}

func newShortcut(label, short, full string, keys ...string) shortcut {
	return shortcut{Binding: key.NewBinding(key.WithKeys(keys...), key.WithHelp(label, short)), description: full}
}

func shortHelp(bindings ...shortcut) string {
	var parts []string
	for _, binding := range bindings {
		if binding.Enabled() {
			help := binding.Help()
			parts = append(parts, help.Key+" — "+help.Desc)
		}
	}
	return strings.Join(parts, " · ")
}

func fullHelpLines(bindings []shortcut) []string {
	var lines []string
	for _, binding := range bindings {
		if binding.Enabled() {
			lines = append(lines, fmt.Sprintf("%-12s%s", binding.Help().Key, binding.description))
		}
	}
	return lines
}

type globalKeyMap struct {
	Help, Quit, Templates shortcut
	Interrupt             key.Binding
}

func newGlobalKeyMap() globalKeyMap {
	return globalKeyMap{
		Help:      newShortcut("?", "помощь", "Помощь", "?"),
		Quit:      newShortcut("q", "выход", "Выход", "q"),
		Templates: newShortcut("p", "шаблоны", "Открыть шаблоны", "p"),
		Interrupt: key.NewBinding(key.WithKeys("ctrl+c")),
	}
}

type movementKeyMap struct {
	Up, Down shortcut
}

func newMovementKeyMap() movementKeyMap {
	description := "Передвижение"
	return movementKeyMap{
		Up:   newShortcut("↑/k", "вверх", description, "up", "k"),
		Down: newShortcut("↓/j", "вниз", description, "down", "j"),
	}
}

func (k movementKeyMap) helpBinding() shortcut {
	if !k.Up.Enabled() {
		return k.Down
	}
	if !k.Down.Enabled() {
		return k.Up
	}
	label := k.Up.Help().Key + ", " + k.Down.Help().Key
	return newShortcut(label, "передвижение", k.Up.description, slices.Concat(k.Up.Keys(), k.Down.Keys())...)
}

type listKeyMap struct {
	movementKeyMap
	Reload shortcut
}

func newListKeyMap() listKeyMap {
	return listKeyMap{movementKeyMap: newMovementKeyMap(), Reload: newShortcut("r", "обновить", "Обновить список", "r")}
}

type studentsKeyMap struct {
	listKeyMap
	globalKeyMap
	Create, Edit, Delete shortcut
}

func newStudentsKeyMap() studentsKeyMap {
	return studentsKeyMap{
		listKeyMap: newListKeyMap(), globalKeyMap: newGlobalKeyMap(),
		Create: newShortcut("n", "создать", "Создать ученика", "n"),
		Edit:   newShortcut("e", "редактировать", "Редактировать имя", "e"),
		Delete: newShortcut("d", "удалить", "Удалить с подтверждением", "d"),
	}
}

type templatesKeyMap struct {
	listKeyMap
	globalKeyMap
	Create, Open, Back shortcut
}

func newTemplatesKeyMap() templatesKeyMap {
	return templatesKeyMap{
		listKeyMap: newListKeyMap(), globalKeyMap: newGlobalKeyMap(),
		Create: newShortcut("n", "создать", "Создать шаблон", "n"),
		Open:   newShortcut("Enter", "открыть", "Открыть темы шаблона", "enter"),
		Back:   newShortcut("Esc", "назад", "Вернуться к ученикам", "esc"),
	}
}

type topicsKeyMap struct {
	listKeyMap
	globalKeyMap
	Create, Edit, Open, Back shortcut
}

func newTopicsKeyMap() topicsKeyMap {
	return topicsKeyMap{
		listKeyMap: newListKeyMap(), globalKeyMap: newGlobalKeyMap(),
		Create: newShortcut("n", "создать", "Создать тему на текущем уровне", "n"),
		Edit:   newShortcut("e", "редактировать", "Редактировать название", "e"),
		Open:   newShortcut("Enter", "открыть", "Открыть тему", "enter"),
		Back:   newShortcut("Esc", "назад", "На уровень назад", "esc"),
	}
}

type formKeyMap struct {
	Save, Cancel shortcut
}

func newFormKeyMap() formKeyMap {
	return formKeyMap{
		Save:   newShortcut("Enter", "сохранить", "Сохранить", "enter"),
		Cancel: newShortcut("Esc", "отменить", "Отменить", "esc"),
	}
}

type confirmationKeyMap struct {
	globalKeyMap
	Confirm, Cancel shortcut
}

func newConfirmationKeyMap() confirmationKeyMap {
	return confirmationKeyMap{
		globalKeyMap: newGlobalKeyMap(),
		Confirm:      newShortcut("y", "удалить", "Подтвердить удаление", "y"),
		Cancel:       newShortcut("n / Esc", "отменить", "Отменить удаление", "n", "esc"),
	}
}

type helpKeyMap struct {
	movementKeyMap
	PageUp, PageDown, Close shortcut
}

func newHelpKeyMap() helpKeyMap {
	return helpKeyMap{
		movementKeyMap: newMovementKeyMap(),
		PageUp:         newShortcut("PgUp", "вверх", "На страницу вверх", "pgup"),
		PageDown:       newShortcut("PgDn", "вниз", "На страницу вниз", "pgdown"),
		Close:          newShortcut("Esc", "закрыть", "Закрыть помощь", "esc"),
	}
}
