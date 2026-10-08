package tui

import (
	"context"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type studentsLoadedMsg struct {
	students []domain.Student
}

type studentsLoadFailedMsg struct {
	err error
}

type studentsModel struct {
	ctx      context.Context
	service  *app.StudentService
	students []domain.Student
	cursor   int
	loading  bool
	err      error
	height   int
	selectID domain.StudentID
}

func newStudentsModel(ctx context.Context, service *app.StudentService) studentsModel {
	return studentsModel{ctx: ctx, service: service, loading: true, height: 10}
}

func (m studentsModel) Init() tea.Cmd {
	return loadStudentsCmd(m.ctx, m.service)
}

func (m studentsModel) selectedStudent() (domain.Student, bool) {
	if m.loading || m.err != nil || m.cursor < 0 || m.cursor >= len(m.students) {
		return domain.Student{}, false
	}
	return m.students[m.cursor], true
}

func loadStudentsCmd(ctx context.Context, service *app.StudentService) tea.Cmd {
	return func() tea.Msg {
		students, err := service.List(ctx)
		if err != nil {
			return studentsLoadFailedMsg{err: err}
		}
		return studentsLoadedMsg{students: students}
	}
}

func (m studentsModel) Update(msg tea.Msg) (studentsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case studentsLoadedMsg:
		m.students = msg.students
		m.loading = false
		m.err = nil
		m.cursor = max(0, min(m.cursor, len(m.students)-1))
		if m.selectID != 0 {
			for i, student := range m.students {
				if student.ID == m.selectID {
					m.cursor = i
					break
				}
			}
			m.selectID = 0
		}
	case studentsLoadFailedMsg:
		m.loading = false
		m.err = msg.err
	case studentDeletedMsg:
		for i, student := range m.students {
			if student.ID == msg.id {
				m.students = slices.Delete(slices.Clone(m.students), i, i+1)
				m.cursor = max(0, min(m.cursor, len(m.students)-1))
				m.selectID = 0
				break
			}
		}
	case tea.WindowSizeMsg:
		m.height = max(1, msg.Height-8)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "r":
			if !m.loading {
				m.loading = true
				m.err = nil
				return m, m.Init()
			}
		case "j", "down":
			m.cursor = max(0, min(m.cursor+1, len(m.students)-1))
		case "k", "up":
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m studentsModel) View() string {
	const title = "Ученики\n\n"
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить учеников. Нажмите r для повторной попытки."
	}
	if len(m.students) == 0 {
		return title + "Список пока пуст."
	}

	var text strings.Builder
	text.WriteString(title)
	start := max(0, m.cursor-m.height+1)
	end := min(len(m.students), start+m.height)
	for i := start; i < end; i++ {
		if i > start {
			text.WriteByte('\n')
		}
		if i == m.cursor {
			text.WriteString("> ")
		} else {
			text.WriteString("  ")
		}
		text.WriteString("• ")
		text.WriteString(m.students[i].Name)
	}
	return text.String()
}
