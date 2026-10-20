package tui

import (
	"errors"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/shesh/gradia/internal/domain"
)

type homeworkTaskForm struct {
	content           textarea.Model
	topics            textarea.Model
	focus, difficulty int
	saving            bool
	err               string
	keys              taskContentFormKeyMap
}

func newHomeworkTaskForm(input domain.HomeworkTaskInput, width, height int) homeworkTaskForm {
	content := textarea.New()
	content.Prompt, content.Placeholder = "", "Текст задачи, Markdown или URL"
	content.ShowLineNumbers, content.CharLimit = false, 0
	content.SetValue(input.Content)
	topics := textarea.New()
	topics.Prompt, topics.ShowLineNumbers = "", false
	topics.Placeholder, topics.CharLimit = "Например: Циклы; Функции", 0
	topics.SetValue(formatTaskTopics(input.Topics))
	difficulty := 0
	if input.Difficulty == domain.DifficultyMedium {
		difficulty = 1
	}
	if input.Difficulty == domain.DifficultyHard {
		difficulty = 2
	}
	m := homeworkTaskForm{content: content, topics: topics, difficulty: difficulty, keys: newTaskContentFormKeyMap()}
	return m.resize(width, height)
}

func (m homeworkTaskForm) resize(width, height int) homeworkTaskForm {
	m.content.SetWidth(max(1, min(78, width-4)))
	m.content.SetHeight(max(3, min(6, height-11)))
	m.topics.SetWidth(max(1, min(70, width-4)))
	m.topics.SetHeight(2)
	return m
}

func (m homeworkTaskForm) focusField(field int) (homeworkTaskForm, tea.Cmd) {
	m.focus = (field + 3) % 3
	m.content.Blur()
	m.topics.Blur()
	if m.focus == 0 {
		return m, m.content.Focus()
	}
	if m.focus == 1 {
		return m, m.topics.Focus()
	}
	return m, nil
}

func (m homeworkTaskForm) difficultyValue() domain.TaskDifficulty {
	difficulties := [...]domain.TaskDifficulty{domain.DifficultyEasy, domain.DifficultyMedium, domain.DifficultyHard}
	return difficulties[m.difficulty]
}

func (m homeworkTaskForm) input() (domain.HomeworkTaskInput, error) {
	topics, err := parseTaskTopics(m.topics.Value())
	if err != nil {
		return domain.HomeworkTaskInput{}, err
	}
	return domain.HomeworkTaskInput{Content: m.content.Value(), Topics: topics, Difficulty: m.difficultyValue()}, nil
}

func (m homeworkTaskForm) Update(msg tea.Msg) (homeworkTaskForm, formAction, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m = m.resize(size.Width, max(1, size.Height-10))
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
		if m.focus == 2 {
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
	}
	var cmd tea.Cmd
	if m.focus == 0 {
		before := m.content.Value()
		m.content, cmd = m.content.Update(msg)
		if m.content.Value() != before {
			m.err = ""
		}
	} else if m.focus == 1 {
		before := m.topics.Value()
		m.topics, cmd = m.topics.Update(msg)
		if m.topics.Value() != before {
			m.err = ""
		}
	}
	return m, formNoAction, cmd
}

func (m homeworkTaskForm) fail(err error) (homeworkTaskForm, tea.Cmd) {
	m.saving, m.err = false, homeworkError(err, "Не удалось сохранить задачу. Повторите Ctrl+S.")
	if errors.Is(err, errTaskTopicFormat) {
		m.err = err.Error()
	}
	return m.focusField(m.focus)
}

func (m homeworkTaskForm) InputMode() inputMode {
	if m.saving {
		return inputBusy
	}
	return inputText
}
func (m homeworkTaskForm) ShortHelp() string {
	if m.saving {
		return ""
	}
	return shortHelp(m.keys.Save, m.keys.Next, m.keys.Previous, m.keys.Cancel)
}

func (m homeworkTaskForm) View() string {
	text := "Задача\n\nТекст / Markdown / URL:\n" + m.content.View() + "\nТемы (через ;, имена с ; — в кавычках):\n" + m.topics.View()
	text += "\nСложность: " + m.difficultyValue().Label()
	if m.focus == 2 {
		text += "  ←/→ или 1 — лёгкая, 2 — средняя, 3 — сложная"
	}
	if m.saving {
		text += "\n\nСохранение…"
	}
	if m.err != "" {
		text += "\n\n" + m.err
	}
	return text
}
