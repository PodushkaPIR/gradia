package app

import "github.com/shesh/gradia/internal/domain"

// Сводки считаются по конечным темам; запись 0 — сводка всего маршрута.
func RouteProgressSummary(topics []domain.RouteTopic) map[domain.RouteTopicID]domain.ProgressSummary {
	children := make(map[domain.RouteTopicID][]domain.RouteTopic)
	for _, topic := range topics {
		var parent domain.RouteTopicID
		if topic.ParentID != nil {
			parent = *topic.ParentID
		}
		children[parent] = append(children[parent], topic)
	}
	summaries := make(map[domain.RouteTopicID]domain.ProgressSummary, len(topics)+1)
	var visit func(domain.RouteTopic, int) domain.ProgressSummary
	visit = func(topic domain.RouteTopic, depth int) domain.ProgressSummary {
		var summary domain.ProgressSummary
		if depth > domain.TopicMaxDepth {
			return summary
		}
		if len(children[topic.ID]) == 0 {
			switch topic.Status.Effective() {
			case domain.ProgressLearning:
				summary.Learning = 1
			case domain.ProgressNeedsPractice:
				summary.NeedsPractice = 1
			case domain.ProgressMastered:
				summary.Mastered = 1
			default:
				summary.NotStarted = 1
			}
		} else {
			for _, child := range children[topic.ID] {
				summary = summary.Add(visit(child, depth+1))
			}
		}
		summaries[topic.ID] = summary
		return summary
	}
	var total domain.ProgressSummary
	for _, topic := range children[0] {
		total = total.Add(visit(topic, 1))
	}
	summaries[0] = total
	return summaries
}
