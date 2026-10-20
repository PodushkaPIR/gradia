package tui

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type bankView uint8

const (
	bankTasks bankView = iota
	bankSubjects
	bankTopics
	bankCollections
)

type taskBankLoadedMsg struct {
	page app.TaskBankPage
	seq  uint64
}
type taskBankLoadFailedMsg struct {
	err error
	seq uint64
}
type bankTaskSavedMsg struct{ task domain.TaskBankTask }
type bankTaskFailedMsg struct{ err error }
type taskCollectionSavedMsg struct{ item domain.TaskCollection }
type taskCollectionFailedMsg struct{ err error }
type taskPackExportedMsg struct{ path string }
type taskPackFailedMsg struct{ err error }

type taskBankModel struct {
	ctx                               context.Context
	service                           *app.TaskBankService
	screenID, loadSeq                 uint64
	data                              app.TaskBankPage
	filter                            domain.TaskBankFilter
	view                              bankView
	cursor, taskCursor, width, height int
	loading                           bool
	err                               error
	taskForm                          *taskBankForm
	editTaskID                        domain.TaskBankTaskID
	nameForm                          *nameForm
	editCollectionID                  domain.TaskCollectionID
	selectTaskID                      domain.TaskBankTaskID
	selectCollectionID                domain.TaskCollectionID
	detail                            *domain.TaskBankTask
	detailOffset                      int
	keys                              taskBankKeyMap
	attachments                       *taskAttachmentsModel
	attachmentSeq                     uint64
	marked                            map[domain.TaskBankTaskID]bool
	exporting                         bool
	packPath                          string
	packErr                           error
}

func (m taskBankModel) reload() (taskBankModel, tea.Cmd) {
	if m.loading || m.taskForm != nil || m.nameForm != nil {
		return m, nil
	}
	m.loading, m.err = true, nil
	m.loadSeq++
	ctx, service, filter, screen, seq := m.ctx, m.service, m.filter, m.screenID, m.loadSeq
	return m, func() tea.Msg {
		page, err := service.Browse(ctx, filter)
		if err != nil {
			return routeToTaskBank(screen, taskBankLoadFailedMsg{err: err, seq: seq})
		}
		return routeToTaskBank(screen, taskBankLoadedMsg{page: page, seq: seq})
	}
}

func (m taskBankModel) Update(msg tea.Msg) (taskBankModel, tea.Cmd) {
	switch msg := msg.(type) {
	case taskPackExportedMsg:
		m.exporting, m.packPath, m.packErr = false, msg.path, nil
		return m, nil
	case taskPackFailedMsg:
		m.exporting, m.packErr = false, msg.err
		return m, nil
	case bankAttachmentsMsg:
		if m.attachments == nil || m.attachments.screenID != msg.screenID {
			return m, nil
		}
		child, cmd := m.attachments.Update(msg.payload)
		m.attachments = &child
		return m, cmd
	case taskBankLoadedMsg:
		if msg.seq != m.loadSeq {
			return m, nil
		}
		m.data, m.loading, m.err = msg.page, false, nil
		m.cursor = max(0, min(m.cursor, m.listLength()-1))
		if m.view == bankTasks && m.selectTaskID != 0 {
			for i, task := range m.data.Tasks {
				if task.ID == m.selectTaskID {
					m.cursor = i
					break
				}
			}
		}
		if m.view == bankCollections && m.selectCollectionID != 0 {
			for i, item := range m.data.Collections {
				if item.ID == m.selectCollectionID {
					m.cursor = i
					break
				}
			}
		}
		m.selectTaskID, m.selectCollectionID = 0, 0
		if m.detail != nil {
			index := slices.IndexFunc(m.data.Tasks, func(task domain.TaskBankTask) bool { return task.ID == m.detail.ID })
			if index < 0 {
				m.detail = nil
			} else {
				task := m.data.Tasks[index]
				m.detail = &task
				m.detailOffset = max(0, min(m.detailOffset, len(m.detailLines())-m.detailHeight()))
			}
		}
		return m, nil
	case taskBankLoadFailedMsg:
		if msg.seq == m.loadSeq {
			m.loading, m.err = false, msg.err
		}
		return m, nil
	case bankTaskSavedMsg:
		m.taskForm, m.editTaskID, m.detail = nil, 0, nil
		m.selectTaskID = msg.task.ID
		return m.reload()
	case bankTaskFailedMsg:
		if m.taskForm == nil {
			return m, nil
		}
		form, cmd := m.taskForm.fail(msg.err)
		m.taskForm = &form
		return m, cmd
	case taskCollectionSavedMsg:
		m.nameForm, m.editCollectionID, m.selectCollectionID = nil, 0, msg.item.ID
		return m.reload()
	case taskCollectionFailedMsg:
		if m.nameForm == nil {
			return m, nil
		}
		form, cmd := m.nameForm.fail(msg.err, taskBankError(msg.err, "Не удалось сохранить коллекцию. Повторите Enter."))
		m.nameForm = &form
		return m, cmd
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, max(1, msg.Height-8)
		if m.detail != nil {
			m.detailOffset = max(0, min(m.detailOffset, len(m.detailLines())-m.detailHeight()))
		}
	}
	if m.exporting {
		return m, nil
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
				failed, cmd := form.fail(err)
				m.taskForm = &failed
				return m, cmd
			}
			ctx, service, id, screen := m.ctx, m.service, m.editTaskID, m.screenID
			return m, func() tea.Msg {
				var task domain.TaskBankTask
				var err error
				if id == 0 {
					task, err = service.Create(ctx, input)
				} else {
					task, err = service.Edit(ctx, id, input)
				}
				if err != nil {
					return routeToTaskBank(screen, bankTaskFailedMsg{err: err})
				}
				return routeToTaskBank(screen, bankTaskSavedMsg{task: task})
			}
		}
		return m, cmd
	}
	if m.nameForm != nil {
		form, action, cmd := m.nameForm.Update(msg)
		m.nameForm = &form
		switch action {
		case formCancel:
			m.nameForm, m.editCollectionID = nil, 0
			return m, nil
		case formSubmit:
			ctx, service, id, name, screen := m.ctx, m.service, m.editCollectionID, form.name.Value(), m.screenID
			return m, func() tea.Msg {
				var item domain.TaskCollection
				var err error
				if id == 0 {
					item, err = service.CreateCollection(ctx, name)
				} else {
					item, err = service.RenameCollection(ctx, id, name)
				}
				if err != nil {
					return routeToTaskBank(screen, taskCollectionFailedMsg{err: err})
				}
				return routeToTaskBank(screen, taskCollectionSavedMsg{item: item})
			}
		}
		return m, cmd
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(pressed, m.keys.Back.Binding) {
			if m.detail != nil {
				m.detail = nil
				return m, nil
			}
			if m.view != bankTasks {
				m.view, m.cursor = bankTasks, max(0, min(m.taskCursor, len(m.data.Tasks)-1))
				return m, nil
			}
		}
		if key.Matches(pressed, m.keys.Reload.Binding) {
			return m.reload()
		}
		if m.loading || m.err != nil {
			return m, nil
		}
		if key.Matches(pressed, m.keys.Export.Binding) && (m.view == bankTasks || m.detail != nil) {
			return m.exportPack()
		}
		if key.Matches(pressed, m.keys.Mark.Binding) {
			if m.detail != nil {
				return m.toggleMark(m.detail.ID), nil
			}
			if m.view == bankTasks && m.cursor >= 0 && m.cursor < len(m.data.Tasks) {
				return m.toggleMark(m.data.Tasks[m.cursor].ID), nil
			}
		}
		if key.Matches(pressed, m.keys.Attachments.Binding) && m.service.AttachmentsEnabled() {
			if m.detail != nil {
				return m.openAttachments(*m.detail)
			}
			if m.view == bankTasks && m.cursor >= 0 && m.cursor < len(m.data.Tasks) {
				return m.openAttachments(m.data.Tasks[m.cursor])
			}
		}
		if m.detail != nil {
			switch {
			case key.Matches(pressed, m.keys.Edit.Binding):
				return m.openTaskForm(m.detail)
			case key.Matches(pressed, m.keys.Up.Binding):
				m.detailOffset = max(0, m.detailOffset-1)
			case key.Matches(pressed, m.keys.Down.Binding):
				m.detailOffset = min(max(0, len(m.detailLines())-m.detailHeight()), m.detailOffset+1)
			}
			return m, nil
		}
		switch {
		case key.Matches(pressed, m.keys.Create.Binding):
			if m.view == bankCollections {
				return m.openCollectionForm(nil)
			}
			if m.view == bankTasks {
				return m.openTaskForm(nil)
			}
		case m.view == bankCollections && key.Matches(pressed, m.keys.Rename.Binding):
			if m.cursor >= 0 && m.cursor < len(m.data.Collections) {
				return m.openCollectionForm(&m.data.Collections[m.cursor])
			}
		case key.Matches(pressed, m.keys.Edit.Binding):
			if m.view == bankTasks && m.cursor >= 0 && m.cursor < len(m.data.Tasks) {
				return m.openTaskForm(&m.data.Tasks[m.cursor])
			}
		case key.Matches(pressed, m.keys.Subjects.Binding):
			return m.chooseView(bankSubjects), nil
		case key.Matches(pressed, m.keys.Topics.Binding):
			return m.chooseView(bankTopics), nil
		case key.Matches(pressed, m.keys.Collections.Binding):
			return m.chooseView(bankCollections), nil
		case key.Matches(pressed, m.keys.Clear.Binding):
			m.filter, m.view, m.cursor = domain.TaskBankFilter{}, bankTasks, 0
			return m.reload()
		case key.Matches(pressed, m.keys.Open.Binding):
			if m.cursor < 0 || m.cursor >= m.listLength() {
				return m, nil
			}
			switch m.view {
			case bankTasks:
				task := m.data.Tasks[m.cursor]
				m.detail, m.detailOffset = &task, 0
				return m, nil
			case bankSubjects:
				m.filter.Subject, m.filter.Topic = m.data.Subjects[m.cursor], ""
			case bankTopics:
				m.filter.Topic = m.data.Topics[m.cursor]
			case bankCollections:
				m.filter.CollectionID = m.data.Collections[m.cursor].ID
			}
			m.view, m.cursor = bankTasks, m.taskCursor
			return m.reload()
		case key.Matches(pressed, m.keys.Up.Binding):
			m.cursor = max(0, m.cursor-1)
		case key.Matches(pressed, m.keys.Down.Binding):
			m.cursor = max(0, min(m.cursor+1, m.listLength()-1))
		}
	}
	return m, nil
}

func (m taskBankModel) chooseView(view bankView) taskBankModel {
	if m.view == bankTasks {
		m.taskCursor = m.cursor
	}
	m.view, m.cursor = view, 0
	return m
}

func (m taskBankModel) toggleMark(id domain.TaskBankTaskID) taskBankModel {
	m.marked = maps.Clone(m.marked)
	if m.marked == nil {
		m.marked = make(map[domain.TaskBankTaskID]bool)
	}
	if m.marked[id] {
		delete(m.marked, id)
	} else {
		m.marked[id] = true
	}
	if m.detail != nil {
		m.detailOffset = max(0, min(m.detailOffset, len(m.detailLines())-m.detailHeight()))
	}
	return m
}

func (m taskBankModel) exportPack() (taskBankModel, tea.Cmd) {
	if len(m.marked) == 0 {
		m.packErr, m.packPath = app.ErrTaskPackSelectionRequired, ""
		return m, nil
	}
	var ids []domain.TaskBankTaskID
	for id := range m.marked {
		ids = append(ids, id)
	}
	m.exporting, m.packErr, m.packPath = true, nil, ""
	ctx, service, screen := m.ctx, m.service, m.screenID
	return m, func() tea.Msg {
		path, err := service.ExportPack(ctx, ids)
		if err != nil {
			return routeToTaskBank(screen, taskPackFailedMsg{err: err})
		}
		return routeToTaskBank(screen, taskPackExportedMsg{path: path})
	}
}

func (m taskBankModel) openAttachments(task domain.TaskBankTask) (taskBankModel, tea.Cmd) {
	m.attachmentSeq++
	bankID, screen := m.screenID, m.attachmentSeq
	wrap := func(msg tea.Msg) tea.Msg { return routeToBankAttachments(bankID, screen, msg) }
	child := taskAttachmentsModel{ctx: m.ctx, source: bankAttachmentSource{service: m.service, task: task.ID}, wrap: wrap, screenID: screen, taskContent: task.Content, width: m.width, height: m.height, keys: newAttachmentsKeyMap()}
	child, cmd := child.reload()
	m.attachments = &child
	return m, cmd
}

func (m taskBankModel) openTaskForm(task *domain.TaskBankTask) (taskBankModel, tea.Cmd) {
	input := domain.TaskBankInput{Subject: m.filter.Subject, Difficulty: domain.DifficultyEasy}
	if m.filter.Topic != "" {
		input.Topics = []string{m.filter.Topic}
	}
	if m.filter.CollectionID != 0 {
		input.CollectionIDs = []domain.TaskCollectionID{m.filter.CollectionID}
	}
	m.editTaskID = 0
	if task != nil {
		input = domain.TaskBankInput{Content: task.Content, Subject: task.Subject, Topics: task.Topics, Difficulty: task.Difficulty, CollectionIDs: task.CollectionIDs}
		m.editTaskID = task.ID
	}
	form, cmd := newTaskBankForm(input, m.data.Collections, m.width, m.height).focusField(0)
	m.taskForm = &form
	return m, cmd
}

func (m taskBankModel) openCollectionForm(item *domain.TaskCollection) (taskBankModel, tea.Cmd) {
	form := newNameForm(nameFormConfig{Title: "Коллекция задач", Label: "Название", Limit: domain.TaskBankNameMaxLength, Width: m.width})
	m.editCollectionID = 0
	if item != nil {
		form.name.SetValue(item.Name)
		form.name.CursorEnd()
		m.editCollectionID = item.ID
	}
	cmd := form.Init()
	m.nameForm = &form
	return m, cmd
}

func taskBankError(err error, fallback string) string {
	for _, known := range []error{domain.ErrTaskContentRequired, domain.ErrTaskTopicsRequired, domain.ErrInvalidTaskDifficulty, domain.ErrTopicNameTooLong, domain.ErrTaskSubjectRequired, domain.ErrTaskBankNameTooLong, domain.ErrTaskBankTaskNotFound, domain.ErrTaskCollectionNameRequired, domain.ErrTaskCollectionNameExists, domain.ErrTaskCollectionNotFound, errTaskTopicFormat} {
		if errors.Is(err, known) {
			return err.Error()
		}
	}
	return fallback
}

func (m taskBankModel) listLength() int {
	switch m.view {
	case bankSubjects:
		return len(m.data.Subjects)
	case bankTopics:
		return len(m.data.Topics)
	case bankCollections:
		return len(m.data.Collections)
	default:
		return len(m.data.Tasks)
	}
}

func (m taskBankModel) atRoot() bool {
	return m.view == bankTasks && m.detail == nil && m.taskForm == nil && m.nameForm == nil && m.attachments == nil
}
func (m taskBankModel) InputMode() inputMode {
	if m.exporting {
		return inputBusy
	}
	if m.attachments != nil {
		return m.attachments.InputMode()
	}
	if m.taskForm != nil {
		return m.taskForm.InputMode()
	}
	if m.nameForm != nil {
		return m.nameForm.InputMode()
	}
	return inputNavigation
}

func (m taskBankModel) collectionNames(ids []domain.TaskCollectionID) string {
	var names []string
	for _, collection := range m.data.Collections {
		if slices.Contains(ids, collection.ID) {
			names = append(names, collection.Name)
		}
	}
	if len(names) == 0 {
		return "—"
	}
	return strings.Join(names, "; ")
}

func (m taskBankModel) taskMetadata(task domain.TaskBankTask) string {
	return "Предмет: " + task.Subject + " · Сложность: " + task.Difficulty.Label() + "\nТемы: " + strings.ReplaceAll(formatTaskTopics(task.Topics), "\n", " ↵ ") + "\nКоллекции: " + m.collectionNames(task.CollectionIDs)
}

func (m taskBankModel) detailLines() []string {
	if m.detail == nil {
		return nil
	}
	text := m.taskMetadata(*m.detail) + "\n\n" + m.detail.Content
	return strings.Split(ansi.Hardwrap(text, max(1, m.width), true), "\n")
}

func (m taskBankModel) packNotice() string {
	if errors.Is(m.packErr, app.ErrTaskPackSelectionRequired) {
		return "\n\nОтметьте задачи клавишей Space перед экспортом."
	}
	if m.packErr != nil {
		return "\n\nНе удалось экспортировать Task Pack. Повторите " + m.keys.Export.Help().Key + "."
	}
	if m.packPath != "" {
		return "\n\n" + ansi.Hardwrap("Task Pack сохранён: "+m.packPath, max(1, m.width), true)
	}
	return ""
}

func (m taskBankModel) detailHeight() int {
	height := m.height - strings.Count(m.packNotice(), "\n")
	if len(m.marked) > 0 {
		height--
	}
	return max(1, height)
}

func (m taskBankModel) View() string {
	title := "Банк задач\n\n"
	if len(m.marked) > 0 {
		title = fmt.Sprintf("Банк задач\nОтмечено: %d (во всех фильтрах)\n\n", len(m.marked))
	}
	if m.exporting {
		return title + "Экспорт Task Pack…"
	}
	if m.attachments != nil {
		return title + m.attachments.View()
	}
	if m.taskForm != nil {
		return title + m.taskForm.View()
	}
	if m.nameForm != nil {
		return title + m.nameForm.View()
	}
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить банк задач. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки."
	}
	if m.detail != nil {
		lines := m.detailLines()
		start := max(0, min(m.detailOffset, len(lines)-m.detailHeight()))
		return title + strings.Join(lines[start:min(len(lines), start+m.detailHeight())], "\n") + m.packNotice()
	}
	var text strings.Builder
	text.WriteString(title)
	if m.filter.Subject != "" {
		text.WriteString("Предмет: " + m.filter.Subject + "\n")
	}
	if m.filter.Topic != "" {
		text.WriteString("Тема: " + m.filter.Topic + "\n")
	}
	if m.filter.CollectionID != 0 {
		text.WriteString("Коллекция: " + m.collectionNames([]domain.TaskCollectionID{m.filter.CollectionID}) + "\n")
	}
	switch m.view {
	case bankSubjects:
		text.WriteString("Выберите предмет\n\n")
	case bankTopics:
		text.WriteString("Выберите тему\n\n")
	case bankCollections:
		text.WriteString("Коллекции\n\n")
	}
	if m.listLength() == 0 {
		if m.view == bankSubjects || m.view == bankTopics {
			text.WriteString("Предметы и темы появляются после создания задач. Esc — вернуться к задачам.")
			return text.String()
		}
		text.WriteString("Пока пусто. " + m.keys.Create.Help().Key + " — создать, " + m.keys.Clear.Help().Key + " — сбросить фильтры." + m.packNotice())
		return text.String()
	}
	height := max(1, m.detailHeight()-8)
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
		switch m.view {
		case bankSubjects:
			text.WriteString(m.data.Subjects[i])
		case bankTopics:
			text.WriteString(taskContentPreview(strings.ReplaceAll(m.data.Topics[i], "\n", " ↵ "), m.width-4))
		case bankCollections:
			item := m.data.Collections[i]
			fmt.Fprintf(&text, "%s — %d задач", item.Name, item.TaskCount)
		default:
			if m.keys.Mark.Enabled() {
				if m.marked[m.data.Tasks[i].ID] {
					text.WriteString("[x] ")
				} else {
					text.WriteString("[ ] ")
				}
			}
			text.WriteString(taskContentPreview(m.data.Tasks[i].Content, m.width-6))
		}
	}
	if m.view == bankTasks && m.cursor >= 0 && m.cursor < len(m.data.Tasks) {
		text.WriteString("\n\n" + m.taskMetadata(m.data.Tasks[m.cursor]))
	}
	text.WriteString(m.packNotice())
	return text.String()
}

func (m taskBankModel) ShortHelp() string {
	if m.exporting {
		return ""
	}
	if m.attachments != nil {
		return m.attachments.ShortHelp()
	}
	if m.taskForm != nil {
		return m.taskForm.ShortHelp()
	}
	if m.nameForm != nil {
		return m.nameForm.ShortHelp()
	}
	if m.detail != nil {
		return shortHelp(m.keys.Edit, m.keys.Attachments, m.keys.Mark, m.keys.Export, m.keys.Back, m.keys.Help)
	}
	if m.view == bankCollections {
		return shortHelp(m.keys.Create, m.keys.Rename, m.keys.Open, m.keys.Back, m.keys.Help)
	}
	if m.view != bankTasks {
		return shortHelp(m.keys.Open, m.keys.Back, m.keys.Help)
	}
	if m.keys.Export.Enabled() {
		return shortHelp(m.keys.Open, m.keys.Create, m.keys.Mark, m.keys.Export, m.keys.Back, m.keys.Help)
	}
	return shortHelp(m.keys.Open, m.keys.Create, m.keys.Edit, m.keys.Attachments, m.keys.Collections, m.keys.Back, m.keys.Help)
}

func (m taskBankModel) FullHelp() (string, []shortcut) {
	if m.attachments != nil {
		return m.attachments.FullHelp()
	}
	keys := m.keys
	bindings := []shortcut{keys.helpBinding()}
	if m.detail != nil {
		bindings = append(bindings, keys.Edit, keys.Attachments, keys.Mark, keys.Export)
	} else if m.view == bankCollections {
		bindings = append(bindings, keys.Create, keys.Rename, keys.Open, keys.Clear)
	} else if m.view == bankTasks {
		bindings = append(bindings, keys.Open, keys.Create, keys.Edit, keys.Attachments, keys.Mark, keys.Export, keys.Subjects, keys.Topics, keys.Collections, keys.Clear)
	} else {
		bindings = append(bindings, keys.Open, keys.Clear)
	}
	return "Банк задач", append(bindings, keys.Reload, keys.Back, keys.Help, keys.Quit)
}
