package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type homeworkLoadedMsg struct {
	items []domain.Homework
	seq   uint64
}
type homeworkTasksLoadedMsg struct {
	id    domain.HomeworkID
	tasks []domain.HomeworkTask
	seq   uint64
}
type homeworkLoadFailedMsg struct {
	id  domain.HomeworkID
	err error
	seq uint64
}
type homeworkSavedMsg struct{ item domain.Homework }
type homeworkSaveFailedMsg struct{ err error }
type homeworkTaskSavedMsg struct{ task domain.HomeworkTask }
type homeworkTaskFailedMsg struct{ err error }

type homeworkModel struct {
	ctx                               context.Context
	service                           *app.HomeworkService
	student                           domain.StudentID
	detailsID, screenID               uint64
	loadSeq                           uint64
	homeworks                         []domain.Homework
	selected                          *domain.Homework
	tasks                             []domain.HomeworkTask
	cursor, homeCursor, width, height int
	loading, saving                   bool
	err, saveErr                      error
	nameForm                          *nameForm
	editHomeworkID                    domain.HomeworkID
	taskForm                          *homeworkTaskForm
	editTaskID                        domain.HomeworkTaskID
	keys                              homeworkKeyMap
	attachments                       *taskAttachmentsModel
	attachmentSeq                     uint64
	bankService                       *app.TaskBankService
	bankPicker                        *taskBankModel
	bankPickerSeq                     uint64
	bankCopyErr                       error
}

func (m homeworkModel) reload() (homeworkModel, tea.Cmd) {
	if m.loading || m.saving {
		return m, nil
	}
	m.loading, m.err, m.saveErr = true, nil, nil
	m.loadSeq++
	seq := m.loadSeq
	ctx, service, student, details, screen := m.ctx, m.service, m.student, m.detailsID, m.screenID
	if m.selected != nil {
		id := m.selected.ID
		return m, func() tea.Msg {
			tasks, err := service.Tasks(ctx, student, id)
			if err != nil {
				return routeToHomework(details, screen, homeworkLoadFailedMsg{id: id, err: err, seq: seq})
			}
			return routeToHomework(details, screen, homeworkTasksLoadedMsg{id: id, tasks: tasks, seq: seq})
		}
	}
	return m, func() tea.Msg {
		items, err := service.List(ctx, student)
		if err != nil {
			return routeToHomework(details, screen, homeworkLoadFailedMsg{err: err, seq: seq})
		}
		return routeToHomework(details, screen, homeworkLoadedMsg{items: items, seq: seq})
	}
}

func (m homeworkModel) syncSummary() homeworkModel {
	if m.selected == nil {
		return m
	}
	item := *m.selected
	item.TotalTasks, item.CompletedTasks = len(m.tasks), 0
	for _, task := range m.tasks {
		if task.Completed {
			item.CompletedTasks++
		}
	}
	m.selected = &item
	m.homeworks = slices.Clone(m.homeworks)
	for i := range m.homeworks {
		if m.homeworks[i].ID == item.ID {
			m.homeworks[i] = item
			break
		}
	}
	return m
}

func (m homeworkModel) Update(msg tea.Msg) (homeworkModel, tea.Cmd) {
	switch msg := msg.(type) {
	case homeworkBankMsg:
		if m.bankPicker == nil || msg.screenID != m.bankPickerSeq {
			return m, nil
		}
		return m.updateBankReply(msg.payload)
	case homeworkAttachmentsMsg:
		if m.attachments == nil || m.attachments.screenID != msg.screenID {
			return m, nil
		}
		child, cmd := m.attachments.Update(msg.payload)
		m.attachments = &child
		return m, cmd
	case homeworkLoadedMsg:
		if m.selected != nil || msg.seq != m.loadSeq {
			return m, nil
		}
		m.homeworks, m.loading, m.err = slices.Clone(msg.items), false, nil
		m.cursor = max(0, min(m.cursor, len(m.homeworks)-1))
		return m, nil
	case homeworkTasksLoadedMsg:
		if m.selected == nil || m.selected.ID != msg.id || msg.seq != m.loadSeq {
			return m, nil
		}
		m.tasks, m.loading, m.err = slices.Clone(msg.tasks), false, nil
		m.cursor = max(0, min(m.cursor, len(m.tasks)-1))
		return m.syncSummary(), nil
	case homeworkLoadFailedMsg:
		if msg.seq != m.loadSeq || m.selected == nil && msg.id != 0 || m.selected != nil && m.selected.ID != msg.id {
			return m, nil
		}
		m.loading, m.err = false, msg.err
		return m, nil
	case homeworkSavedMsg:
		m.nameForm, m.editHomeworkID = nil, 0
		m.homeworks = slices.Clone(m.homeworks)
		index := slices.IndexFunc(m.homeworks, func(item domain.Homework) bool { return item.ID == msg.item.ID })
		if index < 0 {
			m.homeworks = append(m.homeworks, msg.item)
			index = len(m.homeworks) - 1
		} else {
			m.homeworks[index] = msg.item
		}
		if m.selected != nil && m.selected.ID == msg.item.ID {
			item := msg.item
			m.selected = &item
		} else {
			m.cursor = index
		}
		return m, nil
	case homeworkSaveFailedMsg:
		if m.nameForm == nil {
			return m, nil
		}
		form, cmd := m.nameForm.fail(msg.err, homeworkError(msg.err, "Не удалось сохранить домашнее задание. Повторите Enter."))
		m.nameForm = &form
		return m, cmd
	case homeworkTaskSavedMsg:
		if m.selected == nil || m.selected.ID != msg.task.HomeworkID {
			return m, nil
		}
		m.taskForm, m.editTaskID, m.saving, m.saveErr = nil, 0, false, nil
		m.tasks = slices.Clone(m.tasks)
		index := slices.IndexFunc(m.tasks, func(task domain.HomeworkTask) bool { return task.ID == msg.task.ID })
		if index < 0 {
			m.tasks = append(m.tasks, msg.task)
			index = len(m.tasks) - 1
		} else {
			m.tasks[index] = msg.task
		}
		m.cursor = index
		return m.syncSummary(), nil
	case homeworkTaskFailedMsg:
		m.saving = false
		if m.taskForm != nil {
			form, cmd := m.taskForm.fail(msg.err)
			m.taskForm = &form
			return m, cmd
		}
		m.saveErr = msg.err
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, max(1, msg.Height-14)
	}
	if m.bankPicker != nil {
		return m.updateBankPicker(msg)
	}
	if m.attachments != nil {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, m.attachments.keys.Back.Binding) && m.attachments.form == nil && m.attachments.InputMode() == inputNavigation {
			m.attachments = nil
			return m, nil
		}
		child, cmd := m.attachments.Update(msg)
		m.attachments = &child
		return m, cmd
	}
	if m.nameForm != nil {
		form, action, cmd := m.nameForm.Update(msg)
		m.nameForm = &form
		switch action {
		case formCancel:
			m.nameForm, m.editHomeworkID = nil, 0
			return m, nil
		case formSubmit:
			ctx, service, student, id, name, details, screen := m.ctx, m.service, m.student, m.editHomeworkID, form.name.Value(), m.detailsID, m.screenID
			return m, func() tea.Msg {
				var item domain.Homework
				var err error
				if id == 0 {
					item, err = service.Create(ctx, student, name)
				} else {
					item, err = service.Rename(ctx, student, id, name)
				}
				if err != nil {
					return routeToHomework(details, screen, homeworkSaveFailedMsg{err: err})
				}
				return routeToHomework(details, screen, homeworkSavedMsg{item: item})
			}
		}
		return m, cmd
	}
	if m.taskForm != nil {
		form, action, cmd := m.taskForm.Update(msg)
		m.taskForm = &form
		switch action {
		case formCancel:
			m.taskForm, m.editTaskID = nil, 0
			return m, nil
		case formSubmit:
			input, err := form.input()
			if err != nil {
				form, cmd := form.fail(err)
				m.taskForm = &form
				return m, cmd
			}
			ctx, service, student, id, taskID, details, screen := m.ctx, m.service, m.student, m.selected.ID, m.editTaskID, m.detailsID, m.screenID
			return m, func() tea.Msg {
				var task domain.HomeworkTask
				var err error
				if taskID == 0 {
					task, err = service.AddTask(ctx, student, id, input)
				} else {
					task, err = service.EditTask(ctx, student, id, taskID, input)
				}
				if err != nil {
					return routeToHomework(details, screen, homeworkTaskFailedMsg{err: err})
				}
				return routeToHomework(details, screen, homeworkTaskSavedMsg{task: task})
			}
		}
		return m, cmd
	}
	if m.saving {
		return m, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(pressed, m.keys.Back.Binding) && m.selected != nil {
			m.selected, m.tasks, m.loading, m.err, m.saveErr = nil, nil, false, nil, nil
			m.cursor = max(0, min(m.homeCursor, len(m.homeworks)-1))
			return m, nil
		}
		if key.Matches(pressed, m.keys.Reload.Binding) {
			return m.reload()
		}
		if m.loading || m.err != nil {
			return m, nil
		}
		switch {
		case key.Matches(pressed, m.keys.Bank.Binding):
			if m.selected != nil && m.bankService != nil {
				return m.openBankPicker()
			}
		case key.Matches(pressed, m.keys.Attachments.Binding):
			if m.service.AttachmentsEnabled() && m.selected != nil && m.cursor >= 0 && m.cursor < len(m.tasks) {
				m.attachmentSeq++
				task := m.tasks[m.cursor]
				details, parent, screen := m.detailsID, m.screenID, m.attachmentSeq
				wrap := func(msg tea.Msg) tea.Msg { return routeToHomeworkAttachments(details, parent, screen, msg) }
				child := taskAttachmentsModel{ctx: m.ctx, source: homeworkAttachmentSource{service: m.service, student: m.student, homework: m.selected.ID, task: task.ID}, taskContent: task.Content, wrap: wrap, screenID: screen, width: m.width, height: m.height, keys: newAttachmentsKeyMap()}
				child, cmd := child.reload()
				m.attachments = &child
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Create.Binding):
			if m.selected == nil {
				return m.openNameForm(nil)
			}
			return m.openTaskForm(nil)
		case key.Matches(pressed, m.keys.Rename.Binding):
			if m.selected != nil {
				return m.openNameForm(m.selected)
			}
			if m.cursor >= 0 && m.cursor < len(m.homeworks) {
				return m.openNameForm(&m.homeworks[m.cursor])
			}
		case key.Matches(pressed, m.keys.Open.Binding):
			if m.selected == nil && m.cursor >= 0 && m.cursor < len(m.homeworks) {
				item := m.homeworks[m.cursor]
				m.homeCursor, m.cursor, m.selected = m.cursor, 0, &item
				return m.reload()
			}
		case key.Matches(pressed, m.keys.EditTask.Binding):
			if m.selected != nil && m.cursor >= 0 && m.cursor < len(m.tasks) {
				return m.openTaskForm(&m.tasks[m.cursor])
			}
		case key.Matches(pressed, m.keys.Toggle.Binding):
			if m.selected != nil && m.cursor >= 0 && m.cursor < len(m.tasks) {
				task := m.tasks[m.cursor]
				m.saving, m.saveErr = true, nil
				ctx, service, student, id, details, screen := m.ctx, m.service, m.student, m.selected.ID, m.detailsID, m.screenID
				return m, func() tea.Msg {
					updated, err := service.SetCompleted(ctx, student, id, task.ID, !task.Completed)
					if err != nil {
						return routeToHomework(details, screen, homeworkTaskFailedMsg{err: err})
					}
					return routeToHomework(details, screen, homeworkTaskSavedMsg{task: updated})
				}
			}
		case key.Matches(pressed, m.keys.Down.Binding):
			m.cursor = max(0, min(m.cursor+1, m.listLength()-1))
		case key.Matches(pressed, m.keys.Up.Binding):
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m homeworkModel) openNameForm(item *domain.Homework) (homeworkModel, tea.Cmd) {
	form := newNameForm(nameFormConfig{Title: "Домашнее задание", Label: "Название", Limit: domain.HomeworkNameMaxLength, Width: m.width})
	m.editHomeworkID = 0
	if item != nil {
		form.name.SetValue(item.Name)
		form.name.CursorEnd()
		m.editHomeworkID = item.ID
	}
	cmd := form.Init()
	m.nameForm = &form
	return m, cmd
}

func (m homeworkModel) openTaskForm(task *domain.HomeworkTask) (homeworkModel, tea.Cmd) {
	input := domain.HomeworkTaskInput{Difficulty: domain.DifficultyEasy}
	m.editTaskID = 0
	if task != nil {
		input = domain.HomeworkTaskInput{Content: task.Content, Topics: task.Topics, Difficulty: task.Difficulty}
		m.editTaskID = task.ID
	}
	form, cmd := newHomeworkTaskForm(input, m.width, m.height).focusField(0)
	m.taskForm = &form
	return m, cmd
}

func (m homeworkModel) listLength() int {
	if m.selected != nil {
		return len(m.tasks)
	}
	return len(m.homeworks)
}
func (m homeworkModel) atRoot() bool {
	return m.selected == nil && m.nameForm == nil && m.taskForm == nil && m.attachments == nil && m.bankPicker == nil
}
func (m homeworkModel) InputMode() inputMode {
	if m.bankPicker != nil {
		if m.saving {
			return inputBusy
		}
		return m.bankPicker.InputMode()
	}
	if m.attachments != nil {
		return m.attachments.InputMode()
	}
	if m.nameForm != nil {
		return m.nameForm.InputMode()
	}
	if m.taskForm != nil {
		return m.taskForm.InputMode()
	}
	if m.saving {
		return inputBusy
	}
	return inputNavigation
}

func homeworkError(err error, fallback string) string {
	for _, known := range []error{domain.ErrHomeworkNameRequired, domain.ErrHomeworkNameTooLong, domain.ErrTaskContentRequired, domain.ErrTaskTopicsRequired, domain.ErrInvalidTaskDifficulty, domain.ErrTopicNameTooLong, domain.ErrHomeworkNotFound, domain.ErrHomeworkTaskNotFound} {
		if errors.Is(err, known) {
			return err.Error()
		}
	}
	return fallback
}

func (m homeworkModel) View() string {
	title := "Домашние задания\n\n"
	if m.selected != nil {
		title = "Домашнее задание — " + m.selected.Name + "\n\n"
	}
	if m.bankPicker != nil {
		if m.saving {
			return title + "Копирование задачи и вложений…"
		}
		text := title + "a — добавить независимую копию с вложениями.\n\n" + m.bankPicker.View()
		if m.bankCopyErr != nil {
			text += "\n\n" + homeworkError(m.bankCopyErr, "Не удалось добавить задачу. Повторите a; r — обновить банк.")
		}
		return text
	}
	if m.attachments != nil {
		return title + m.attachments.View()
	}
	if m.nameForm != nil {
		return title + m.nameForm.View()
	}
	if m.taskForm != nil {
		return title + m.taskForm.View()
	}
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить домашние задания или задачи. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки."
	}
	var text strings.Builder
	text.WriteString(title)
	if m.selected != nil {
		fmt.Fprintf(&text, "%d/%d выполнено\n\n", m.selected.CompletedTasks, m.selected.TotalTasks)
	}
	if m.listLength() == 0 {
		text.WriteString("Пока пусто. Нажмите " + m.keys.Create.Help().Key + " для создания.")
		return text.String()
	}
	height := max(1, m.height-6)
	start := max(0, m.cursor-height+1)
	for i := start; i < min(m.listLength(), start+height); i++ {
		if i > start {
			text.WriteByte('\n')
		}
		if i == m.cursor {
			text.WriteString("> ")
		} else {
			text.WriteString("  ")
		}
		if m.selected == nil {
			item := m.homeworks[i]
			fmt.Fprintf(&text, "%s — %d/%d выполнено", item.Name, item.CompletedTasks, item.TotalTasks)
		} else {
			task := m.tasks[i]
			if task.Completed {
				text.WriteString("[x] ")
			} else {
				text.WriteString("[ ] ")
			}
			text.WriteString(taskContentPreview(task.Content, m.width-20))
		}
	}
	if m.selected != nil && m.cursor >= 0 && m.cursor < len(m.tasks) {
		task := m.tasks[m.cursor]
		fmt.Fprintf(&text, "\n\nСложность: %s\nТемы: %s\n", task.Difficulty.Label(), strings.Join(task.Topics, "; "))
		lines := strings.Split(task.Content, "\n")
		if len(lines) > 3 {
			lines = append(lines[:3], "…")
		}
		text.WriteString(strings.Join(lines, "\n"))
	}
	if m.saving {
		text.WriteString("\n\nСохранение…")
	}
	if m.saveErr != nil {
		text.WriteString("\n\nНе удалось сохранить выполнение задачи. Повторите Space.")
	}
	return text.String()
}

func (m homeworkModel) ShortHelp() string {
	if m.bankPicker != nil {
		if m.saving {
			return ""
		}
		return shortHelp(m.keys.AssignBank) + " · " + m.bankPicker.ShortHelp()
	}
	if m.attachments != nil {
		return m.attachments.ShortHelp()
	}
	if m.nameForm != nil {
		return m.nameForm.ShortHelp()
	}
	if m.taskForm != nil {
		return m.taskForm.ShortHelp()
	}
	if m.saving {
		return ""
	}
	if m.selected != nil {
		return shortHelp(m.keys.Create, m.keys.Bank, m.keys.EditTask, m.keys.Toggle, m.keys.Attachments, m.keys.Back, m.keys.Help)
	}
	return shortHelp(m.keys.Open, m.keys.Create, m.keys.Rename, m.keys.Back, m.keys.Help)
}

func (m homeworkModel) FullHelp() (string, []shortcut) {
	if m.bankPicker != nil {
		title, bindings := m.bankPicker.FullHelp()
		return title, append(bindings, m.keys.AssignBank)
	}
	if m.attachments != nil {
		return m.attachments.FullHelp()
	}
	keys := m.keys
	bindings := []shortcut{keys.helpBinding(), keys.Create, keys.Rename}
	if m.selected == nil {
		bindings = append(bindings, keys.Open)
	} else {
		bindings = append(bindings, keys.Bank, keys.EditTask, keys.Toggle, keys.Attachments)
	}
	return "Домашние задания", append(bindings, keys.Reload, keys.Back, keys.Help, keys.Quit)
}
