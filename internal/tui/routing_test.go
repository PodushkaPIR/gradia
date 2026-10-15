package tui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func TestRoutingDeliversToInactiveScreenAndThroughHelp(t *testing.T) {
	students := &studentRepoStub{students: []domain.Student{{ID: 1, Name: "Иван"}}}
	templates := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}}
	model := NewApp(context.Background(), app.NewStudentService(students), app.NewTemplateService(templates))
	studentLoad := model.Init()
	model, templateLoad := updateApp(model, keypress('p'))
	model, _ = updateApp(model, studentLoad())
	if !model.showTemplates || model.students.loading || model.students.students[0].Name != "Иван" || !model.templates.loading {
		t.Fatal("результат должен обновлять свой экран, даже когда открыт другой")
	}
	model, _ = updateApp(model, keypress('?'))
	model, _ = updateApp(model, templateLoad())
	if model.help == nil || model.templates.loading || model.templates.templates[0].Name != "Python" {
		t.Fatal("помощь не должна блокировать доставку результатов команд")
	}
	model, cmd := updateApp(model, routeToTemplates(studentsLoadedMsg{students: []domain.Student{{Name: "Не тот адрес"}}}))
	if cmd != nil || model.students.students[0].Name != "Иван" {
		t.Fatal("маршрут должен определяться адресом, а не типом вложенного сообщения")
	}
	model, cmd = updateApp(model, screenMsg{target: screenTarget(255), payload: studentsLoadedMsg{}})
	if cmd != nil || len(model.students.students) != 1 {
		t.Fatal("сообщение с неизвестным адресом должно игнорироваться")
	}
}

func TestRoutingDropsAllRepliesForClosedTopics(t *testing.T) {
	repo := &templateRepoStub{templates: []domain.Template{{ID: 1, Name: "Python"}}}
	model := openTemplatesForTopics(t, repo)
	model, oldLoad := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	oldID := model.templates.topics.screenID
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, _ = updateApp(model, keypress('n'))
	model, _ = updateApp(model, oldLoad())
	model, _ = updateApp(model, routeToTopics(oldID, templateTopicCreateFailedMsg{err: errors.New("old failure")}))
	if model.templates.form == nil || model.templates.form.err != nil || model.templates.topics != nil {
		t.Fatal("ответ закрытого экрана тем попал в форму создания шаблона")
	}
	model, _ = updateApp(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	model, newLoad := updateApp(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, payload := range []tea.Msg{
		templateTopicsLoadedMsg{topics: []domain.TemplateTopic{{ID: 99, Name: "Старая тема"}}},
		templateTopicsLoadFailedMsg{err: errors.New("old load failure")},
		templateTopicCreatedMsg{topic: domain.TemplateTopic{ID: 99}},
		templateTopicCreateFailedMsg{err: errors.New("old create failure")},
		templateTopicRenamedMsg{topic: domain.TemplateTopic{ID: 99}},
		templateTopicRenameFailedMsg{err: errors.New("old rename failure")},
	} {
		var cmd tea.Cmd
		model, cmd = updateApp(model, routeToTopics(oldID, payload))
		if cmd != nil || !model.templates.topics.loading || model.templates.topics.err != nil || len(model.templates.topics.topics) != 0 {
			t.Fatalf("устаревший результат %T изменил новый экран", payload)
		}
	}
	model, _ = updateApp(model, newLoad())
	if model.templates.topics.loading {
		t.Fatal("ответ текущего экземпляра должен доставляться")
	}
}

func TestOperationCommandsAddressSuccessAndFailure(t *testing.T) {
	ctx := context.Background()
	for _, failed := range []bool{false, true} {
		for _, operation := range []string{"load students", "create student", "rename student", "delete student", "load templates", "create template", "rename template", "load topics", "create topic", "rename topic"} {
			name := operation + "/success"
			if failed {
				name = operation + "/failure"
			}
			t.Run(name, func(t *testing.T) {
				var err error
				if failed {
					err = errors.New("storage failed")
				}
				studentService := app.NewStudentService(&studentRepoStub{
					students: []domain.Student{{ID: 1, Name: "Иван"}}, err: err, createErr: err, renameErr: err, deleteErr: err,
				})
				templateService := app.NewTemplateService(&templateRepoStub{
					templates: []domain.Template{{ID: 1, Name: "Python"}}, topics: []domain.TemplateTopic{{ID: 1, TemplateID: 1, Name: "Циклы"}},
					loadErr: err, createErr: err, renameErr: err, topicsLoadErr: err, topicCreateErr: err, topicRenameErr: err,
				})
				var cmd tea.Cmd
				target := studentsScreen
				scoped := false
				switch operation {
				case "load students":
					cmd = loadStudentsCmd(ctx, studentService)
				case "create student":
					cmd = saveStudentCmd(ctx, studentService, 0, "Иван")
				case "rename student":
					cmd = saveStudentCmd(ctx, studentService, 1, "Иван")
				case "delete student":
					cmd = deleteStudentCmd(ctx, studentService, 1)
				case "load templates":
					target, cmd = templatesScreen, loadTemplatesCmd(ctx, templateService)
				case "create template":
					target, cmd = templatesScreen, saveTemplateCmd(ctx, templateService, 0, "Python")
				case "rename template":
					target, cmd = templatesScreen, saveTemplateCmd(ctx, templateService, 1, "Python")
				case "load topics":
					target, scoped = templatesScreen, true
					model := templateTopicsModel{ctx: ctx, service: templateService, template: domain.Template{ID: 1}, screenID: 42}
					_, cmd = model.reload()
				case "create topic":
					target, scoped = templatesScreen, true
					cmd = createTemplateTopicCmd(ctx, templateService, 1, 0, "Циклы", 42)
				case "rename topic":
					target, scoped = templatesScreen, true
					cmd = renameTemplateTopicCmd(ctx, templateService, 1, 1, "Циклы", 42)
				}
				result, ok := cmd().(screenMsg)
				if !ok || result.target != target {
					t.Fatalf("результат команды не адресован своему экрану: %+v", result)
				}
				if scoped {
					child, ok := result.payload.(topicsMsg)
					if !ok || child.screenID != 42 {
						t.Fatalf("потерян адрес экземпляра экрана тем: %+v", result.payload)
					}
				}
			})
		}
	}
}
