package domain

import "testing"

func TestProgressStatusTransitions(t *testing.T) {
	for _, tc := range []struct{ status, next, previous ProgressStatus }{
		{ProgressNotStarted, ProgressLearning, ProgressNotStarted},
		{ProgressLearning, ProgressNeedsPractice, ProgressNotStarted},
		{ProgressNeedsPractice, ProgressMastered, ProgressLearning},
		{ProgressMastered, ProgressMastered, ProgressNeedsPractice},
	} {
		if !tc.status.Valid() || tc.status.Next() != tc.next || tc.status.Previous() != tc.previous {
			t.Fatalf("неверный переход для %q", tc.status)
		}
	}
	if ProgressStatus("invalid").Valid() || ProgressStatus("").Valid() {
		t.Fatal("произвольные строки не должны приниматься для записи")
	}
	if ProgressStatus("").Next() != ProgressLearning || ProgressStatus("").Effective() != ProgressNotStarted {
		t.Fatal("не заданный прогресс должен означать not_started")
	}
}
