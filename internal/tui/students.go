package tui

import (
	"context"
	"errors"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
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
	ctx             context.Context
	service         *app.StudentService
	students        []domain.Student
	cursor          int
	loading         bool
	err             error
	height          int
	width           int
	selectID        domain.StudentID
	form            *nameForm
	editStudentID   domain.StudentID
	deleteDialog    *studentDeleteModel
	keys            studentsKeyMap
	routeService    *app.RouteService
	templateService *app.TemplateService
	reportService   *app.ReportService
	homeworkService *app.HomeworkService
	taskBankService *app.TaskBankService
	details         *studentDetailsModel
	detailsSeq      uint64
}

func newStudentsModel(ctx context.Context, service *app.StudentService) studentsModel {
	return studentsModel{ctx: ctx, service: service, loading: true, height: 10, width: 80, keys: newStudentsKeyMap()}
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
			return routeToStudents(studentsLoadFailedMsg{err: err})
		}
		return routeToStudents(studentsLoadedMsg{students: students})
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
		return m, nil
	case studentsLoadFailedMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	case studentSavedMsg:
		m.form = nil
		m.editStudentID = 0
		m.loading = true
		m.err = nil
		m.selectID = msg.student.ID
		return m, m.Init()
	case studentSaveFailedMsg:
		if m.form != nil {
			form, cmd := m.form.fail(msg.err, m.saveError(msg.err))
			m.form = &form
			return m, cmd
		}
		return m, nil
	case studentDetailsMsg:
		if m.details == nil || msg.screenID != m.details.screenID {
			return m, nil
		}
		details, cmd := m.details.Update(msg.payload)
		m.details = &details
		return m, cmd
	case studentDeletedMsg:
		m.deleteDialog = nil
		for i, student := range m.students {
			if student.ID == msg.id {
				m.students = slices.Delete(slices.Clone(m.students), i, i+1)
				m.cursor = max(0, min(m.cursor, len(m.students)-1))
				m.selectID = 0
				break
			}
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.height = max(1, msg.Height-8)
		m.width = msg.Width
	}
	if m.details != nil {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && key.Matches(pressed, m.details.keys.Back.Binding) && m.details.atRoot() {
			m.details = nil
			return m, nil
		}
		details, cmd := m.details.Update(msg)
		m.details = &details
		return m, cmd
	}
	if m.deleteDialog != nil {
		if pressed, ok := msg.(tea.KeyPressMsg); ok && m.InputMode().allowsCancel() && key.Matches(pressed, m.deleteDialog.keys.Cancel.Binding) {
			m.deleteDialog = nil
			return m, nil
		}
		dialog, cmd := m.deleteDialog.Update(msg)
		m.deleteDialog = &dialog
		return m, cmd
	}
	if m.form != nil {
		form, action, cmd := m.form.Update(msg)
		m.form = &form
		switch action {
		case formCancel:
			m.form = nil
			m.editStudentID = 0
			return m, nil
		case formSubmit:
			return m, saveStudentCmd(m.ctx, m.service, m.editStudentID, form.name.Value())
		}
		return m, cmd
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Open.Binding):
			if student, ok := m.selectedStudent(); ok && m.routeService != nil {
				m.detailsSeq++
				details := studentDetailsModel{ctx: m.ctx, routesService: m.routeService, templatesService: m.templateService, student: student, screenID: m.detailsSeq, height: max(1, m.height-4), width: m.width, keys: newStudentDetailsKeyMap()}
				details.reportService = m.reportService
				details.homeworkService = m.homeworkService
				details.taskBankService = m.taskBankService
				details.keys.Homework.SetEnabled(m.homeworkService != nil)
				details.keys.Report.SetEnabled(m.reportService != nil)
				details.keys.PDFReport.SetEnabled(m.reportService != nil && m.reportService.PDFExportEnabled())
				details.keys.ReportWithHomework.SetEnabled(m.reportService != nil && m.reportService.HomeworkReportEnabled())
				details.keys.PDFWithHomework.SetEnabled(m.reportService != nil && m.reportService.HomeworkReportEnabled() && m.reportService.PDFExportEnabled())
				details, cmd := details.reload()
				m.details = &details
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Create.Binding):
			if !m.loading {
				form := m.newStudentNameForm("Новый ученик")
				cmd := form.Init()
				m.editStudentID = 0
				m.form = &form
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Edit.Binding):
			if student, ok := m.selectedStudent(); ok {
				form := m.newStudentNameForm("Редактирование ученика")
				form.name.SetValue(student.Name)
				form.name.CursorEnd()
				cmd := form.Init()
				m.editStudentID = student.ID
				m.form = &form
				return m, cmd
			}
		case key.Matches(pressed, m.keys.Delete.Binding):
			if student, ok := m.selectedStudent(); ok {
				dialog := newStudentDelete(m.ctx, m.service, student)
				m.deleteDialog = &dialog
			}
		case key.Matches(pressed, m.keys.Reload.Binding):
			if !m.loading {
				m.loading = true
				m.err = nil
				return m, m.Init()
			}
		case key.Matches(pressed, m.keys.Down.Binding):
			m.cursor = max(0, min(m.cursor+1, len(m.students)-1))
		case key.Matches(pressed, m.keys.Up.Binding):
			m.cursor = max(0, m.cursor-1)
		}
	}
	return m, nil
}

func (m studentsModel) newStudentNameForm(title string) nameForm {
	return newNameForm(nameFormConfig{Title: title, Label: "Имя", Placeholder: "Имя ученика", Limit: domain.StudentNameMaxLength, Width: m.width})
}

func (m studentsModel) saveError(err error) string {
	if errors.Is(err, domain.ErrStudentNameRequired) || errors.Is(err, domain.ErrStudentNameTooLong) {
		return err.Error()
	}
	if errors.Is(err, domain.ErrStudentNotFound) {
		return "Ученик больше не существует. Нажмите " + m.form.keys.Cancel.Help().Key + " и обновите список по " + m.keys.Reload.Help().Key + "."
	}
	return "Не удалось сохранить ученика. Нажмите " + m.form.keys.Save.Help().Key + " для повторной попытки."
}

func (m studentsModel) View() string {
	if m.details != nil {
		return m.details.View()
	}
	if m.deleteDialog != nil {
		return m.deleteDialog.View()
	}
	if m.form != nil {
		return m.form.View()
	}
	const title = "Ученики\n\n"
	if m.loading {
		return title + "Загрузка…"
	}
	if m.err != nil {
		return title + "Не удалось загрузить учеников. Нажмите " + m.keys.Reload.Help().Key + " для повторной попытки."
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
		text.WriteString(m.students[i].Name)
	}
	return text.String()
}

func (m studentsModel) InputMode() inputMode {
	if m.details != nil {
		return m.details.InputMode()
	}
	if m.deleteDialog != nil {
		return m.deleteDialog.InputMode()
	}
	if m.form != nil {
		return m.form.InputMode()
	}
	// Загрузка списка идёт в фоне и не захватывает клавиатуру.
	return inputNavigation
}

func (m studentsModel) ShortHelp() string {
	if m.details != nil {
		return m.details.ShortHelp()
	}
	switch m.InputMode() {
	case inputBusy:
		return ""
	case inputConfirmation:
		return shortHelp(m.deleteDialog.keys.Confirm, m.deleteDialog.keys.Cancel)
	case inputText:
		return m.form.ShortHelp()
	}
	return shortHelp(m.keys.Open, m.keys.Create, m.keys.Templates, m.keys.TaskBank, m.keys.Help)
}

func (m studentsModel) FullHelp() (string, []shortcut) {
	if m.details != nil {
		return m.details.FullHelp()
	}
	if m.deleteDialog != nil {
		keys := m.deleteDialog.keys
		return "Удаление ученика", []shortcut{keys.Confirm, keys.Cancel, keys.Help}
	}
	keys := m.keys
	return "Ученики", []shortcut{keys.helpBinding(), keys.Open, keys.Create, keys.Edit, keys.Delete, keys.Templates, keys.TaskBank, keys.Reload, keys.Help, keys.Quit}
}
