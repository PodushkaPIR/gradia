package domain

// RouteTopicExclusions учитывает собственное исключение темы и исключения её предков.
// Исходные отметки не меняются: восстановление раздела сохраняет исключения потомков.
func RouteTopicExclusions(topics []RouteTopic) map[RouteTopicID]bool {
	children := make(map[RouteTopicID][]RouteTopic)
	for _, topic := range topics {
		var parent RouteTopicID
		if topic.ParentID != nil {
			parent = *topic.ParentID
		}
		children[parent] = append(children[parent], topic)
	}
	excluded := make(map[RouteTopicID]bool, len(topics))
	var visit func(RouteTopicID, bool, int)
	visit = func(parent RouteTopicID, inherited bool, depth int) {
		if depth >= TopicMaxDepth {
			return
		}
		for _, topic := range children[parent] {
			excluded[topic.ID] = inherited || topic.Excluded
			visit(topic.ID, excluded[topic.ID], depth+1)
		}
	}
	visit(0, false, 0)
	return excluded
}
