package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func studentsPayload(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	result, ok := cmd().(screenMsg)
	if !ok || result.target != studentsScreen {
		t.Fatal("команда экрана учеников должна адресовать результат своему экрану")
	}
	return result.payload
}

func TestStudentsScreenOwnsCreateAndDelete(t *testing.T) {
	repo := &studentRepoStub{}
	screen := newStudentsModel(context.Background(), app.NewStudentService(repo))
	screen, _ = screen.Update(studentsPayload(t, screen.Init()))
	screen, _ = screen.Update(keypress('n'))
	if screen.InputMode() != inputText || screen.InputMode().allowsHelp() || !strings.Contains(screen.View(), "Новый ученик") {
		t.Fatal("экран должен самостоятельно открывать форму и сообщать режим ввода")
	}
	screen, _ = screen.Update(tea.WindowSizeMsg{Width: 30, Height: 20})
	if screen.form.name.Width() != 26 {
		t.Fatal("экран должен передавать размер терминала своей форме")
	}
	screen.form.name.SetValue("Иван")
	screen, save := screen.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if save == nil || repo.createCalls != 0 {
		t.Fatal("создание должно выполняться через Cmd")
	}
	screen, reload := screen.Update(studentsPayload(t, save))
	if screen.form != nil || reload == nil || !screen.loading {
		t.Fatal("экран должен сам закрыть форму и запросить обновление списка")
	}
	screen, _ = screen.Update(studentsPayload(t, reload))
	if screen.InputMode() != inputNavigation || !screen.InputMode().allowsHelp() || !strings.Contains(screen.View(), "> Иван") {
		t.Fatal("созданный ученик не выбран в списке")
	}
	screen, _ = screen.Update(keypress('d'))
	if screen.InputMode() != inputConfirmation || !screen.InputMode().allowsHelp() || !strings.Contains(screen.ShortHelp(), "y — удалить") {
		t.Fatal("экран должен сам управлять подтверждением удаления")
	}
	screen, remove := screen.Update(keypress('y'))
	if remove == nil || screen.InputMode() != inputBusy || screen.InputMode().allowsHelp() || repo.deleteCalls != 0 {
		t.Fatal("удаление должно выполняться через Cmd с корректным режимом ввода")
	}
	screen, _ = screen.Update(studentsPayload(t, remove))
	if screen.deleteDialog != nil || len(screen.students) != 0 || !strings.Contains(screen.View(), "Список пока пуст") {
		t.Fatal("экран должен сам обработать результат удаления")
	}
}

func TestAppUsesStudentsScreenInputState(t *testing.T) {
	repo := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}
	model := NewApp(context.Background(), app.NewStudentService(repo), app.NewTemplateService(&templateRepoStub{}), nil)
	model, _ = updateApp(model, model.Init()())
	model, _ = updateApp(model, keypress('d'))
	model, cmd := updateApp(model, keypress('p'))
	if cmd != nil || model.showTemplates || model.students.deleteDialog == nil {
		t.Fatal("root не должен уходить с экрана с открытым подтверждением")
	}
	model, _ = updateApp(model, keypress('?'))
	if model.help == nil || !strings.Contains(model.View().Content, "Помощь — Удаление ученика") {
		t.Fatal("root должен получать контекст помощи от экрана учеников")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.help != nil || model.students.deleteDialog == nil {
		t.Fatal("закрытие помощи не должно отменять подтверждение")
	}
	model, _ = updateApp(model, keypress('n'))
	model, load := updateApp(model, keypress('p'))
	if !model.showTemplates || load == nil {
		t.Fatal("после отмены подтверждения переход к шаблонам должен работать")
	}
}
