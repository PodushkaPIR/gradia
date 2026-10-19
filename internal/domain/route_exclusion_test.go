package domain

import "testing"

func TestRouteTopicExclusionsPreserveIndividualFlags(t *testing.T) {
	root, branch := RouteTopicID(1), RouteTopicID(2)
	topics := []RouteTopic{
		{ID: 4, ParentID: &branch},
		{ID: 3, ParentID: &root, Excluded: true},
		{ID: 2, ParentID: &root},
		{ID: 1, Excluded: true},
		{ID: 5},
	}
	excluded := RouteTopicExclusions(topics)
	for _, id := range []RouteTopicID{1, 2, 3, 4} {
		if !excluded[id] {
			t.Fatalf("topic %d should inherit exclusion", id)
		}
	}
	if excluded[5] || topics[0].Excluded || topics[2].Excluded {
		t.Fatal("exclusion leaked or mutated descendants")
	}
	topics[3].Excluded = false
	excluded = RouteTopicExclusions(topics)
	if excluded[1] || excluded[2] || excluded[4] || !excluded[3] {
		t.Fatalf("restoration lost individual flags: %v", excluded)
	}
}
