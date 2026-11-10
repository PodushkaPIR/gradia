package tui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/domain"
)

const (
	taskFieldContent = iota
	taskFieldSubject
	taskFieldTopics
	taskFieldDifficulty
	taskFieldCollections
	taskFieldImagePath
	taskFieldCount
)

type taskBankForm struct {
	content, topics                     textarea.Model
	subject                             textinput.Model
	imagePath                           *textinput.Model
	collections                         []domain.TaskCollection
	chosen                              map[domain.TaskCollectionID]bool
	focus, difficulty, collectionCursor int
	saving                              bool
	err                                 string
	keys                                taskContentFormKeyMap
}

func newTaskBankForm(input domain.TaskBankInput, collections []domain.TaskCollection, width, height int) taskBankForm {
	content, topics := textarea.New(), textarea.New()
	content.Prompt, topics.Prompt = "", ""
	content.ShowLineNumbers, topics.ShowLineNumbers = false, false
	content.CharLimit, topics.CharLimit = 0, 0
	content.Placeholder, topics.Placeholder = "Текст, Markdown или URL", "Например: Циклы; Функции"
	content.SetValue(input.Content)
	topics.SetValue(formatTaskTopics(input.Topics))
	subject := textinput.New()
	subject.Placeholder, subject.CharLimit = "Например: Информатика", domain.TaskBankNameMaxLength
	subject.SetValue(input.Subject)
	subject.CursorEnd()
	chosen := make(map[domain.TaskCollectionID]bool)
	for _, id := range input.CollectionIDs {
		chosen[id] = true
	}
	difficulty := 0
	if input.Difficulty == domain.DifficultyMedium {
		difficulty = 1
	}
	if input.Difficulty == domain.DifficultyHard {
		difficulty = 2
	}
	m := taskBankForm{
		content:     content,
		topics:      topics,
		subject:     subject,
		collections: collections,
		chosen:      chosen,
		difficulty:  difficulty,
		keys:        newTaskContentFormKeyMap(),
	}
	return m.resize(width, height)
}

func newBankImageForm(input domain.TaskBankInput, collections []domain.TaskCollection, width, height int) taskBankForm {
	m := newTaskBankForm(input, collections, width, height)
	path := textinput.New()
	path.Placeholder = "Полный путь или ~/Downloads/задача.png"
	m.imagePath = &path
	m.content.Placeholder = "Например: Квадратные уравнения — задача 1"
	return m.resize(width, height)
}

func (m taskBankForm) resize(width, height int) taskBankForm {
	if m.imagePath != nil {
		m.imagePath.SetWidth(max(1, min(70, width-4)))
	}
	m.content.SetWidth(max(1, min(78, width-4)))
	m.content.SetHeight(max(2, min(6, height-16)))
	if m.imagePath != nil {
		m.content.SetHeight(2)
	}
	m.subject.SetWidth(max(1, min(70, width-4)))
	m.topics.SetWidth(max(1, min(70, width-4)))
	m.topics.SetHeight(2)
	return m
}

func (m taskBankForm) focusField(field int) (taskBankForm, tea.Cmd) {
	fields := taskFieldCount - 1
	if m.imagePath != nil {
		fields = taskFieldCount
		m.imagePath.Blur()
	}
	m.focus = (field + fields) % fields
	m.content.Blur()
	m.subject.Blur()
	m.topics.Blur()
	switch m.focus {
	case taskFieldContent:
		return m, m.content.Focus()
	case taskFieldSubject:
		return m, m.subject.Focus()
	case taskFieldTopics:
		return m, m.topics.Focus()
	case taskFieldImagePath:
		return m, m.imagePath.Focus()
	default:
		return m, nil
	}
}

func (m taskBankForm) input() (domain.TaskBankInput, error) {
	topics, err := parseTaskTopics(m.topics.Value())
	if err != nil {
		return domain.TaskBankInput{}, err
	}
	difficulties := [...]domain.TaskDifficulty{domain.DifficultyEasy, domain.DifficultyMedium, domain.DifficultyHard}
	var ids []domain.TaskCollectionID
	for _, collection := range m.collections {
		if m.chosen[collection.ID] {
			ids = append(ids, collection.ID)
		}
	}
	return domain.TaskBankInput{
		Content:       m.content.Value(),
		Subject:       m.subject.Value(),
		Topics:        topics,
		Difficulty:    difficulties[m.difficulty],
		CollectionIDs: ids,
	}, nil
}

func (m taskBankForm) Update(msg tea.Msg) (taskBankForm, formAction, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m = m.resize(size.Width, max(1, size.Height-6))
	}
	if m.saving {
		return m, formNoAction, nil
	}
	if pressed, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(pressed, m.keys.Cancel.Binding):
			return m, formCancel, nil
		case key.Matches(pressed, m.keys.Save.Binding):
			m.saving, m.err = true, ""
			return m, formSubmit, nil
		case key.Matches(pressed, m.keys.Next.Binding):
			updated, cmd := m.focusField(m.focus + 1)
			return updated, formNoAction, cmd
		case key.Matches(pressed, m.keys.Previous.Binding):
			updated, cmd := m.focusField(m.focus - 1)
			return updated, formNoAction, cmd
		}
		if m.focus == taskFieldDifficulty {
			switch pressed.String() {
			case "1":
				m.difficulty = 0
			case "2":
				m.difficulty = 1
			case "3":
				m.difficulty = 2
			case "left", "up":
				m.difficulty = max(0, m.difficulty-1)
			case "right", "down":
				m.difficulty = min(2, m.difficulty+1)
			}
			m.err = ""
			return m, formNoAction, nil
		}
		if m.focus == taskFieldCollections {
			switch pressed.String() {
			case "up", "k":
				m.collectionCursor = max(0, m.collectionCursor-1)
			case "down", "j":
				m.collectionCursor = max(0, min(m.collectionCursor+1, len(m.collections)-1))
			case "space":
				if m.collectionCursor >= 0 && m.collectionCursor < len(m.collections) {
					id := m.collections[m.collectionCursor].ID
					chosen := make(map[domain.TaskCollectionID]bool, len(m.chosen))
					for key, value := range m.chosen {
						chosen[key] = value
					}
					chosen[id] = !chosen[id]
					m.chosen = chosen
				}
			}
			m.err = ""
			return m, formNoAction, nil
		}
	}
	var cmd tea.Cmd
	switch m.focus {
	case taskFieldContent:
		before := m.content.Value()
		m.content, cmd = m.content.Update(msg)
		if m.content.Value() != before {
			m.err = ""
		}
	case taskFieldSubject:
		before := m.subject.Value()
		m.subject, cmd = m.subject.Update(msg)
		if m.subject.Value() != before {
			m.err = ""
		}
	case taskFieldTopics:
		before := m.topics.Value()
		m.topics, cmd = m.topics.Update(msg)
		if m.topics.Value() != before {
			m.err = ""
		}
	case taskFieldImagePath:
		path := *m.imagePath
		before := path.Value()
		path, cmd = path.Update(msg)
		m.imagePath = &path
		if path.Value() != before {
			m.err = ""
		}
	}
	return m, formNoAction, cmd
}

func (m taskBankForm) fail(err error) (taskBankForm, tea.Cmd) {
	m.saving, m.err = false, taskBankError(err, "Не удалось сохранить задачу банка. Повторите Ctrl+S.")
	if m.imagePath != nil {
		m.err = attachmentError(err, m.err)
		if errors.Is(err, domain.ErrTaskContentRequired) {
			m.err = "Введите короткое название задачи."
		}
	}
	return m.focusField(m.focus)
}

func (m taskBankForm) InputMode() inputMode {
	if m.saving {
		return inputBusy
	}
	return inputText
}
func (m taskBankForm) ShortHelp() string {
	if m.saving {
		return ""
	}
	return shortHelp(m.keys.Save, m.keys.Next, m.keys.Previous, m.keys.Cancel)
}

func (m taskBankForm) View() string {
	difficulties := [...]domain.TaskDifficulty{domain.DifficultyEasy, domain.DifficultyMedium, domain.DifficultyHard}
	heading := "Задача банка\nТекст / Markdown / URL:\n"
	if m.imagePath != nil {
		heading = "Задача из картинки\nПуть к изображению:\n" + m.imagePath.View() +
			"\nКороткое название (условие остаётся в картинке):\n"
	}
	text := heading + m.content.View() +
		"\nПредмет:\n" + m.subject.View() +
		"\nТемы (через ;, имена с ; — в кавычках):\n" + m.topics.View()
	text += "\nСложность: " + difficulties[m.difficulty].Label()
	if m.focus == taskFieldDifficulty {
		text += " (←/→ или 1/2/3)"
	}
	text += "\nКоллекции (Space — выбор, ↑/↓ — список):"
	if len(m.collections) == 0 {
		text += "\nПока нет. Создайте их в разделе c."
	}
	start := max(0, m.collectionCursor-1)
	for i := start; i < min(len(m.collections), start+2); i++ {
		collection := m.collections[i]
		marker, check := " ", " "
		if m.focus == taskFieldCollections && i == m.collectionCursor {
			marker = ">"
		}
		if m.chosen[collection.ID] {
			check = "x"
		}
		text += fmt.Sprintf("\n%s [%s] %s", marker, check, collection.Name)
	}
	if m.saving {
		text += "\nСохранение…"
	}
	if m.err != "" {
		text += "\n" + m.err
	}
	return strings.TrimRight(text, "\n")
}
