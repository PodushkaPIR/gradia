package tui

import (
	"context"
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
type routeProgressFailedMsg struct{ err error }
type routeNoteSavedMsg struct {
	topicID  domain.RouteTopicID
	markdown string
}
type routeNoteFailedMsg struct{ err error }
type routeTopicRow struct {
	topic       domain.RouteTopic
	depth       int
	hasChildren bool
}

func routeTopicRows(topics []domain.RouteTopic) []routeTopicRow {
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
			rows = append(rows, routeTopicRow{topic: topic, depth: depth, hasChildren: len(children[topic.ID]) > 0})
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
	cursor, height      int
	width               int
	note                *noteForm
	noteTopicID         domain.RouteTopicID
	loading             bool
	saving              bool
	err                 error
	saveErr             error
	summaries           map[domain.RouteTopicID]domain.ProgressSummary
	keys                routeViewKeyMap
}

func (m routeViewModel) reload() (routeViewModel, tea.Cmd) {
	if m.loading || m.saving || m.note != nil {
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
		m.rows, m.loading, m.err = routeTopicRows(msg.topics), false, nil
		m.saveErr = nil
		m.summaries = app.RouteProgressSummary(msg.topics)
		m.cursor = max(0, min(m.cursor, len(m.rows)-1))
		return m, nil
	case routeTopicsFailedMsg:
		m.loading, m.err = false, msg.err
		return m, nil
	case routeProgressSavedMsg:
		m.saving, m.saveErr = false, nil
		m.rows = slices.Clone(m.rows)
		for i := range m.rows {
			if m.rows[i].topic.ID == msg.topicID {
				m.rows[i].topic.Status = msg.status
				break
			}
		}
		var topics []domain.RouteTopic
		for _, row := range m.rows {
			topics = append(topics, row.topic)
		}
		m.summaries = app.RouteProgressSummary(topics)
		return m, nil
	case routeProgressFailedMsg:
		m.saving, m.saveErr = false, msg.err
		return m, nil
	case routeNoteSavedMsg:
		m.note, m.noteTopicID = nil, 0
		m.rows = slices.Clone(m.rows)
		for i := range m.rows {
			if m.rows[i].topic.ID == msg.topicID {
				m.rows[i].topic.Notes = msg.markdown
				break
			}
		}
		return m, nil
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
			return routeToRouteView(details, view, routeProgressFailedMsg{err: err})
		}
		return routeToRouteView(details, view, routeProgressSavedMsg{topicID: id, status: status})
	}
}

func (m routeViewModel) InputMode() inputMode {
	if m.note != nil {
		return m.note.InputMode()
	}
	if m.saving {
		return inputBusy
	}
	return inputNavigation
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
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить темы. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки."
	}
	if len(m.rows) == 0 {
		return title + "В этом маршруте пока нет тем."
	}
	var text strings.Builder
	text.WriteString(title)
	total := m.summaries[0]
	fmt.Fprintf(&text, "Освоено %d/%d тем\n\n", total.Mastered, total.Total())
	var preview []string
	if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].topic.Notes != "" {
		preview = strings.Split(m.rows[m.cursor].topic.Notes, "\n")
		if len(preview) > 3 {
			preview = append(preview[:3], "…")
		}
	}
	height := m.height
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
		if m.rows[i].hasChildren {
			summary := m.summaries[m.rows[i].topic.ID]
			fmt.Fprintf(&text, "  %s %d/%d освоено · %d изучается · %d нужна практика", progressBar(summary), summary.Mastered, summary.Total(), summary.Learning, summary.NeedsPractice)
		} else {
			text.WriteString("  — " + progressStatusLabel(m.rows[i].topic.Status))
		}
	}
	if len(preview) > 0 {
		text.WriteString("\n\nЗаметка (Markdown)\n" + strings.Join(preview, "\n"))
	}
	if m.saving {
		text.WriteString("\n\nСохранение…")
	}
	if m.saveErr != nil {
		text.WriteString("\n\nНе удалось сохранить прогресс. Изменение не применено; повторите действие.")
	}
	return text.String()
}
