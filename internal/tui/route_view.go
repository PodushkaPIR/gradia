package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type routeTopicsLoadedMsg struct{ topics []domain.RouteTopic }
type routeTopicsFailedMsg struct{ err error }
type routeProgressSavedMsg struct {
	topicID domain.RouteTopicID
	status  domain.ProgressStatus
}
type routeChangesFailedMsg struct{ err error }
type routeExclusionSavedMsg struct {
	topicID  domain.RouteTopicID
	excluded bool
}
type routeRevisionSavedMsg struct {
	topicID  domain.RouteTopicID
	revision bool
}
type routeTopicSavedMsg struct {
	topic   domain.RouteTopic
	renamed bool
}
type routeTopicFailedMsg struct{ err error }
type routeTopicsReorderedMsg struct{ positions []domain.RouteTopicPosition }
type routeNoteSavedMsg struct {
	topicID  domain.RouteTopicID
	markdown string
}
type routeNoteFailedMsg struct{ err error }
type routeTopicRow struct {
	topic               domain.RouteTopic
	depth               int
	hasChildren         bool
	effectivelyExcluded bool
}

func routeTopicRows(topics []domain.RouteTopic) []routeTopicRow {
	excluded := domain.RouteTopicExclusions(topics)
	children := make(map[domain.RouteTopicID][]domain.RouteTopic)
	for _, topic := range topics {
		var parent domain.RouteTopicID
		if topic.ParentID != nil {
			parent = *topic.ParentID
		}
		children[parent] = append(children[parent], topic)
	}
	for _, group := range children {
		sort.Slice(group, func(i, j int) bool {
			if group[i].Position != group[j].Position {
				return group[i].Position < group[j].Position
			}
			return group[i].ID < group[j].ID
		})
	}
	var rows []routeTopicRow
	var walk func(domain.RouteTopicID, int)
	walk = func(parent domain.RouteTopicID, depth int) {
		if depth >= domain.TopicMaxDepth {
			return
		}
		for _, topic := range children[parent] {
			rows = append(rows, routeTopicRow{topic: topic, depth: depth, hasChildren: len(children[topic.ID]) > 0, effectivelyExcluded: excluded[topic.ID]})
			walk(topic.ID, depth+1)
		}
	}
	walk(0, 0)
	return rows
}

type routeViewModel struct {
	ctx                 context.Context
	service             *app.RouteService
	route               domain.Route
	detailsID, screenID uint64
	rows                []routeTopicRow
	topics              []domain.RouteTopic
	showExcluded        bool
	cursor, height      int
	width               int
	topicForm           *nameForm
	topicEditID         domain.RouteTopicID
	topicParentID       domain.RouteTopicID
	note                *noteForm
	noteTopicID         domain.RouteTopicID
	loading             bool
	saving              bool
	err                 error
	saveErr             error
	summaries           map[domain.RouteTopicID]domain.ProgressSummary
	keys                routeViewKeyMap
}

// rebuildRows сохраняет выбор по ID, даже когда меняется видимость соседних веток.
func (m routeViewModel) rebuildRows() routeViewModel {
	var selected domain.RouteTopicID
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		selected = m.rows[m.cursor].topic.ID
	}
	m.rows = routeTopicRows(m.topics)
	if !m.showExcluded {
		m.rows = slices.DeleteFunc(m.rows, func(row routeTopicRow) bool { return row.effectivelyExcluded })
	}
	m.summaries = app.RouteProgressSummary(m.topics)
	m.cursor = max(0, min(m.cursor, len(m.rows)-1))
	for i, row := range m.rows {
		if row.topic.ID == selected {
			m.cursor = i
			break
		}
	}
	return m
}

func (m routeViewModel) updateTopic(id domain.RouteTopicID, apply func(*domain.RouteTopic)) routeViewModel {
	m.topics = slices.Clone(m.topics)
	for i := range m.topics {
		if m.topics[i].ID == id {
			apply(&m.topics[i])
			break
		}
	}
	return m.rebuildRows()
}

func (m routeViewModel) reload() (routeViewModel, tea.Cmd) {
	if m.loading || m.saving || m.topicForm != nil || m.note != nil {
		return m, nil
	}
	m.loading, m.err = true, nil
	ctx, svc, route, details, view := m.ctx, m.service, m.route.ID, m.detailsID, m.screenID
	return m, func() tea.Msg {
		topics, err := svc.Topics(ctx, route)
		if err != nil {
			return routeToRouteView(details, view, routeTopicsFailedMsg{err: err})
		}
		return routeToRouteView(details, view, routeTopicsLoadedMsg{topics: topics})
	}
}

func (m routeViewModel) Update(msg tea.Msg) (routeViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case routeTopicsLoadedMsg:
		m.topics, m.loading, m.err = slices.Clone(msg.topics), false, nil
		m.saveErr = nil
		return m.rebuildRows(), nil
	case routeTopicsFailedMsg:
		m.loading, m.err = false, msg.err
		return m, nil
	case routeProgressSavedMsg:
		m.saving, m.saveErr = false, nil
		return m.updateTopic(msg.topicID, func(topic *domain.RouteTopic) {
			topic.Status = msg.status
			if msg.status != domain.ProgressMastered {
				topic.Revision = false
			}
		}), nil
	case routeChangesFailedMsg:
		m.saving, m.saveErr = false, msg.err
		return m, nil
	case routeTopicsReorderedMsg:
		m.saving, m.saveErr = false, nil
		m.topics = slices.Clone(m.topics)
		for i := range m.topics {
			for _, position := range msg.positions {
				if m.topics[i].ID == position.TopicID {
					m.topics[i].Position = position.Position
				}
			}
		}
		return m.rebuildRows(), nil
	case routeRevisionSavedMsg:
		m.saving, m.saveErr = false, nil
		return m.updateTopic(msg.topicID, func(topic *domain.RouteTopic) { topic.Revision = msg.revision }), nil
	case routeExclusionSavedMsg:
		m.saving, m.saveErr = false, nil
		return m.updateTopic(msg.topicID, func(topic *domain.RouteTopic) { topic.Excluded = msg.excluded }), nil
	case routeTopicSavedMsg:
		m.saving, m.saveErr = false, nil
		m.topicForm, m.topicEditID, m.topicParentID = nil, 0, 0
		found := false
		m.topics = slices.Clone(m.topics)
		for i := range m.topics {
			if m.topics[i].ID == msg.topic.ID {
				if msg.renamed {
					m.topics[i].Name = msg.topic.Name
				} else {
					m.topics[i] = msg.topic
				}
				found = true
				break
			}
		}
		if !found {
			m.topics = append(m.topics, msg.topic)
		}
		m = m.rebuildRows()
		for i, row := range m.rows {
			if row.topic.ID == msg.topic.ID {
				m.cursor = i
				break
			}
		}
		return m, nil
	case routeTopicFailedMsg:
		if m.topicForm != nil {
			form, cmd := m.topicForm.fail(msg.err, m.topicFormError(msg.err))
			m.topicForm = &form
			return m, cmd
		}
		return m, nil
	case routeNoteSavedMsg:
		m.note, m.noteTopicID = nil, 0
		return m.updateTopic(msg.topicID, func(topic *domain.RouteTopic) { topic.Notes = msg.markdown }), nil
	case routeNoteFailedMsg:
		if m.note != nil {
			note, cmd := m.note.fail(msg.err)
			m.note = &note
			return m, cmd
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.height = max(1, msg.Height-10)
		m.width = msg.Width
	}
	if m.topicForm != nil {
		form, action, cmd := m.topicForm.Update(msg)
		m.topicForm = &form
		switch action {
		case formCancel:
			m.topicForm, m.topicEditID, m.topicParentID = nil, 0, 0
			return m, nil
		case formSubmit:
			ctx, svc, route, editID, parentID, name, details, view := m.ctx, m.service, m.route.ID, m.topicEditID, m.topicParentID, form.name.Value(), m.detailsID, m.screenID
			return m, func() tea.Msg {
				if editID != 0 {
					if err := svc.RenameTopic(ctx, route, editID, name); err != nil {
						return routeToRouteView(details, view, routeTopicFailedMsg{err: err})
					}
					return routeToRouteView(details, view, routeTopicSavedMsg{topic: domain.RouteTopic{ID: editID, Name: name}, renamed: true})
				}
				topic, err := svc.AddTopic(ctx, route, parentID, name)
				if err != nil {
					return routeToRouteView(details, view, routeTopicFailedMsg{err: err})
				}
				return routeToRouteView(details, view, routeTopicSavedMsg{topic: topic})
			}
		}
		return m, cmd
	}
	if m.note != nil {
		note, action, cmd := m.note.Update(msg)
		m.note = &note
		switch action {
		case formCancel:
			m.note, m.noteTopicID = nil, 0
			return m, nil
		case formSubmit:
			ctx, svc, route, topic, markdown, details, view := m.ctx, m.service, m.route.ID, m.noteTopicID, note.text.Value(), m.detailsID, m.screenID
			return m, func() tea.Msg {
				if err := svc.SetNote(ctx, route, topic, markdown); err != nil {
					return routeToRouteView(details, view, routeNoteFailedMsg{err: err})
				}
				return routeToRouteView(details, view, routeNoteSavedMsg{topicID: topic, markdown: markdown})
			}
		}
		return m, cmd
	}
	if m.saving {
		return m, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.MoveTopicUp.Binding):
			return m.moveTopic(false)
		case key.Matches(pressed, m.keys.MoveTopicDown.Binding):
			return m.moveTopic(true)
		case key.Matches(pressed, m.keys.CreateTopic.Binding):
			if !m.loading && m.err == nil {
				parentID := domain.RouteTopicID(0)
				if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].topic.ParentID != nil {
					parentID = *m.rows[m.cursor].topic.ParentID
				}
				return m.openTopicForm("Новая тема", "Название", "Например, Циклы", parentID, 0, "")
			}
		case key.Matches(pressed, m.keys.AddSubtopic.Binding):
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(m.rows) {
				return m.openTopicForm("Новая подтема", "Название", "Например, Цикл for", m.rows[m.cursor].topic.ID, 0, "")
			}
		case key.Matches(pressed, m.keys.EditTopic.Binding):
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(m.rows) {
				topic := m.rows[m.cursor].topic
				return m.openTopicForm("Переименование темы", "Название", "Название темы", 0, topic.ID, topic.Name)
			}
		case key.Matches(pressed, m.keys.Exclude.Binding):
			return m.changeExclusion()
		case key.Matches(pressed, m.keys.ShowExcluded.Binding):
			if !m.loading && m.err == nil {
				m.showExcluded = !m.showExcluded
				return m.rebuildRows(), nil
			}
		case key.Matches(pressed, m.keys.EditNote.Binding):
			if !m.loading && m.err == nil && m.cursor >= 0 && m.cursor < len(m.rows) {
				topic := m.rows[m.cursor].topic
				note := newNoteForm(topic.Name, topic.Notes, m.width, m.height)
				cmd := note.Init()
				m.note, m.noteTopicID = &note, topic.ID
				return m, cmd
			}
		case key.Matches(pressed, m.keys.NextStatus.Binding):
			return m.changeStatus(true)
		case key.Matches(pressed, m.keys.Revision.Binding):
			return m.changeRevision()
		case key.Matches(pressed, m.keys.PreviousStatus.Binding):
			return m.changeStatus(false)
		case key.Matches(pressed, m.keys.Reload.Binding):
			return m.reload()
		case key.Matches(pressed, m.keys.Down.Binding):
			m.cursor = max(0, min(m.cursor+1, len(m.rows)-1))
		case key.Matches(pressed, m.keys.Up.Binding):
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m routeViewModel) changeStatus(forward bool) (routeViewModel, tea.Cmd) {
	if m.loading || m.err != nil || m.cursor < 0 || m.cursor >= len(m.rows) || m.rows[m.cursor].hasChildren {
		return m, nil
	}
	topic := m.rows[m.cursor].topic
	status := topic.Status.Next()
	if !forward {
		status = topic.Status.Previous()
	}
	if status == topic.Status.Effective() {
		return m, nil
	}
	m.saving, m.saveErr = true, nil
	ctx, svc, route, id, details, view := m.ctx, m.service, m.route.ID, topic.ID, m.detailsID, m.screenID
	return m, func() tea.Msg {
		if err := svc.SetStatus(ctx, route, id, status); err != nil {
			return routeToRouteView(details, view, routeChangesFailedMsg{err: err})
		}
		return routeToRouteView(details, view, routeProgressSavedMsg{topicID: id, status: status})
	}
}

func (m routeViewModel) moveTopic(forward bool) (routeViewModel, tea.Cmd) {
	if m.loading || m.err != nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return m, nil
	}
	topic := m.rows[m.cursor].topic
	step := -1
	if forward {
		step = 1
	}
	for i := m.cursor + step; i >= 0 && i < len(m.rows); i += step {
		other := m.rows[i].topic
		sameParent := topic.ParentID == nil && other.ParentID == nil
		if topic.ParentID != nil && other.ParentID != nil {
			sameParent = *topic.ParentID == *other.ParentID
		}
		if !sameParent {
			continue
		}
		m.saving, m.saveErr = true, nil
		ctx, svc, route, details, view := m.ctx, m.service, m.route.ID, m.detailsID, m.screenID
		return m, func() tea.Msg {
			positions, err := svc.SwapTopics(ctx, route, topic.ID, other.ID)
			if err != nil {
				return routeToRouteView(details, view, routeChangesFailedMsg{err: err})
			}
			return routeToRouteView(details, view, routeTopicsReorderedMsg{positions: positions})
		}
	}
	return m, nil
}

func (m routeViewModel) InputMode() inputMode {
	if m.topicForm != nil {
		return m.topicForm.InputMode()
	}
	if m.note != nil {
		return m.note.InputMode()
	}
	if m.saving {
		return inputBusy
	}
	return inputNavigation
}

func (m routeViewModel) openTopicForm(title, label, placeholder string, parentID, editID domain.RouteTopicID, value string) (routeViewModel, tea.Cmd) {
	form := newNameForm(nameFormConfig{Title: title, Label: label, Placeholder: placeholder, Limit: domain.TopicNameMaxLength, Width: m.width})
	if value != "" {
		form.name.SetValue(value)
		form.name.CursorEnd()
	}
	cmd := form.Init()
	m.topicForm, m.topicEditID, m.topicParentID = &form, editID, parentID
	return m, cmd
}

func (m routeViewModel) topicFormError(err error) string {
	if errors.Is(err, domain.ErrTopicNameRequired) || errors.Is(err, domain.ErrTopicNameTooLong) || errors.Is(err, domain.ErrTopicDepthExceeded) || errors.Is(err, domain.ErrRouteTopicNotFound) || errors.Is(err, domain.ErrRouteNotFound) {
		return err.Error()
	}
	return "Не удалось сохранить тему. Нажмите " + m.topicForm.keys.Save.Help().Key + " для повторной попытки."
}

func (m routeViewModel) changeRevision() (routeViewModel, tea.Cmd) {
	if m.loading || m.err != nil || m.cursor < 0 || m.cursor >= len(m.rows) || m.rows[m.cursor].hasChildren {
		return m, nil
	}
	topic := m.rows[m.cursor].topic
	if topic.Status != domain.ProgressMastered {
		return m, nil
	}
	m.saving, m.saveErr = true, nil
	ctx, svc, route, details, view := m.ctx, m.service, m.route.ID, m.detailsID, m.screenID
	revision := !topic.Revision
	return m, func() tea.Msg {
		if err := svc.SetRevision(ctx, route, topic.ID, revision); err != nil {
			return routeToRouteView(details, view, routeChangesFailedMsg{err: err})
		}
		return routeToRouteView(details, view, routeRevisionSavedMsg{topicID: topic.ID, revision: revision})
	}
}

func (m routeViewModel) changeExclusion() (routeViewModel, tea.Cmd) {
	if m.loading || m.err != nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return m, nil
	}
	topic := m.rows[m.cursor].topic
	excluded := !topic.Excluded
	m.saving, m.saveErr = true, nil
	ctx, svc, route, details, view := m.ctx, m.service, m.route.ID, m.detailsID, m.screenID
	return m, func() tea.Msg {
		if err := svc.SetExcluded(ctx, route, topic.ID, excluded); err != nil {
			return routeToRouteView(details, view, routeChangesFailedMsg{err: err})
		}
		return routeToRouteView(details, view, routeExclusionSavedMsg{topicID: topic.ID, excluded: excluded})
	}
}

func progressStatusLabel(status domain.ProgressStatus) string {
	switch status.Effective() {
	case domain.ProgressLearning:
		return "изучается"
	case domain.ProgressNeedsPractice:
		return "нужна практика"
	case domain.ProgressMastered:
		return "освоено"
	default:
		return "не начато"
	}
}

func studyStatusLabel(status domain.ProgressStatus) string {
	switch status.Effective() {
	case domain.ProgressLearning, domain.ProgressNeedsPractice:
		return "текущая"
	case domain.ProgressMastered:
		return "завершённая"
	default:
		return "предстоящая"
	}
}

func progressBar(summary domain.ProgressSummary) string {
	filled := 0
	if summary.Total() > 0 {
		filled = summary.Mastered * 10 / summary.Total()
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", 10-filled) + "]"
}

func (m routeViewModel) View() string {
	title := "Маршрут — " + m.route.Name + "\n\n"
	if m.note != nil {
		return title + m.note.View()
	}
	if m.topicForm != nil {
		return title + m.topicForm.View()
	}
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить темы. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки."
	}
	if len(m.topics) == 0 {
		return title + "В этом маршруте пока нет тем."
	}
	var text strings.Builder
	text.WriteString(title)
	total := m.summaries[0]
	fmt.Fprintf(&text, "Освоено %d/%d тем\n\n", total.Mastered, total.Total())
	current := total.Learning + total.NeedsPractice
	fmt.Fprintf(&text, "Текущих: %d · Предстоящих: %d\n\n", current, total.NotStarted)
	extraHeaderLines := 2
	var revisionCount, excludedMastered int
	var suggestedTopic string
	for _, row := range routeTopicRows(m.topics) {
		if row.hasChildren {
			continue
		}
		if row.effectivelyExcluded && row.topic.Status == domain.ProgressMastered {
			excludedMastered++
		}
		if row.topic.Revision && !row.effectivelyExcluded {
			revisionCount++
		}
		if current == 0 && suggestedTopic == "" && !row.effectivelyExcluded && row.topic.Status.Effective() != domain.ProgressMastered {
			suggestedTopic = row.topic.Name
		}
	}
	if excludedMastered > 0 {
		fmt.Fprintf(&text, "Ранее освоено среди исключённых: %d тем\n\n", excludedMastered)
		extraHeaderLines += 2
	}
	if m.showExcluded {
		text.WriteString("Показаны исключённые темы\n\n")
		extraHeaderLines += 2
	}
	if len(m.rows) == 0 {
		text.WriteString("Все темы исключены. Нажмите " + m.keys.ShowExcluded.Help().Key + ", чтобы показать их и восстановить.")
	}
	if revisionCount > 0 {
		fmt.Fprintf(&text, "К повторению: %d тем\n\n", revisionCount)
		extraHeaderLines += 2
	}
	if suggestedTopic != "" {
		text.WriteString("Рекомендуется начать: " + suggestedTopic + "\n\n")
		extraHeaderLines += 2
	}
	var preview []string
	if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].topic.Notes != "" {
		preview = strings.Split(m.rows[m.cursor].topic.Notes, "\n")
		if len(preview) > 3 {
			preview = append(preview[:3], "…")
		}
	}
	height := max(1, m.height-extraHeaderLines)
	if len(preview) > 0 {
		height = max(1, height-len(preview)-3)
	}
	start := max(0, m.cursor-height+1)
	for i := start; i < min(len(m.rows), start+height); i++ {
		if i > start {
			text.WriteByte('\n')
		}
		if i == m.cursor {
			text.WriteString("> ")
		} else {
			text.WriteString("  ")
		}
		text.WriteString(strings.Repeat("  ", m.rows[i].depth))
		if m.rows[i].hasChildren {
			text.WriteString("▾ ")
		} else {
			text.WriteString("• ")
		}
		text.WriteString(m.rows[i].topic.Name)
		if m.rows[i].topic.Excluded {
			text.WriteString(" [исключено]")
		} else if m.rows[i].effectivelyExcluded {
			text.WriteString(" [исключено с разделом]")
		}
		if m.rows[i].hasChildren {
			summary := m.summaries[m.rows[i].topic.ID]
			fmt.Fprintf(&text, "  %s %d/%d освоено · %d изучается · %d нужна практика", progressBar(summary), summary.Mastered, summary.Total(), summary.Learning, summary.NeedsPractice)
		} else {
			text.WriteString("  — " + progressStatusLabel(m.rows[i].topic.Status))
			if m.rows[i].topic.Revision {
				text.WriteString(" · повторить")
			}
			if !m.rows[i].effectivelyExcluded {
				text.WriteString(" · " + studyStatusLabel(m.rows[i].topic.Status))
			}
		}
	}
	if len(preview) > 0 {
		text.WriteString("\n\nЗаметка (Markdown)\n" + strings.Join(preview, "\n"))
	}
	if m.saving {
		text.WriteString("\n\nСохранение…")
	}
	if m.saveErr != nil {
		text.WriteString("\n\nНе удалось сохранить изменения. Изменение не применено; повторите действие.")
	}
	return text.String()
}
