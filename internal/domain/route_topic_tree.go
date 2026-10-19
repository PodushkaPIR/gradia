package domain

import "sort"

// RouteTopicTreeRow keeps display order and effective exclusion consistent across views.
type RouteTopicTreeRow struct {
	Topic               RouteTopic
	Depth               int
	HasChildren         bool
	EffectivelyExcluded bool
}

func RouteTopicTreeRows(topics []RouteTopic) []RouteTopicTreeRow {
	excluded := RouteTopicExclusions(topics)
	children := make(map[RouteTopicID][]RouteTopic)
	for _, topic := range topics {
		var parent RouteTopicID
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
	var rows []RouteTopicTreeRow
	var walk func(RouteTopicID, int)
	walk = func(parent RouteTopicID, depth int) {
		if depth >= TopicMaxDepth {
			return
		}
		for _, topic := range children[parent] {
			rows = append(rows, RouteTopicTreeRow{Topic: topic, Depth: depth, HasChildren: len(children[topic.ID]) > 0, EffectivelyExcluded: excluded[topic.ID]})
			walk(topic.ID, depth+1)
		}
	}
	walk(0, 0)
	return rows
}
