package domain

import "errors"

var (
	ErrInvalidProgressStatus = errors.New("некорректный статус прогресса")
	ErrRouteTopicNotFound    = errors.New("тема не найдена в этом маршруте")
	ErrTopicHasSubtopics     = errors.New("прогресс раздела рассчитывается по его подтемам")
)

type ProgressStatus string

const (
	ProgressNotStarted    ProgressStatus = "not_started"
	ProgressLearning      ProgressStatus = "learning"
	ProgressNeedsPractice ProgressStatus = "needs_practice"
	ProgressMastered      ProgressStatus = "mastered"
)

func (s ProgressStatus) Effective() ProgressStatus {
	if s == "" {
		return ProgressNotStarted
	}
	return s
}

func (s ProgressStatus) Valid() bool {
	switch s {
	case ProgressNotStarted, ProgressLearning, ProgressNeedsPractice, ProgressMastered:
		return true
	}
	return false
}

func (s ProgressStatus) Next() ProgressStatus {
	switch s.Effective() {
	case ProgressNotStarted:
		return ProgressLearning
	case ProgressLearning:
		return ProgressNeedsPractice
	case ProgressNeedsPractice, ProgressMastered:
		return ProgressMastered
	}
	return ProgressNotStarted
}

func (s ProgressStatus) Previous() ProgressStatus {
	switch s.Effective() {
	case ProgressMastered:
		return ProgressNeedsPractice
	case ProgressNeedsPractice:
		return ProgressLearning
	case ProgressLearning, ProgressNotStarted:
		return ProgressNotStarted
	}
	return ProgressNotStarted
}

type ProgressSummary struct{ NotStarted, Learning, NeedsPractice, Mastered int }

func (s ProgressSummary) Total() int { return s.NotStarted + s.Learning + s.NeedsPractice + s.Mastered }
func (s ProgressSummary) Add(other ProgressSummary) ProgressSummary {
	s.NotStarted += other.NotStarted
	s.Learning += other.Learning
	s.NeedsPractice += other.NeedsPractice
	s.Mastered += other.Mastered
	return s
}
