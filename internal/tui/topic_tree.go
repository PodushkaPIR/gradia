package tui

import (
	"maps"
	"sort"

	"github.com/shesh/gradia/internal/domain"
)

type topicRow struct {
	topic domain.TemplateTopic
	depth int
}

func topicParentID(topic domain.TemplateTopic) domain.TemplateTopicID {
	if topic.ParentID == nil {
		return 0
	}
	return *topic.ParentID
}

func (m templateTopicsModel) currentParentID() domain.TemplateTopicID {
	if len(m.path) == 0 {
		return 0
	}
	return m.path[len(m.path)-1].ID
}

func (m templateTopicsModel) canCreate() bool {
	return len(m.path) < domain.TopicMaxDepth
}

func (m templateTopicsModel) rows() []topicRow {
	children := make(map[domain.TemplateTopicID][]domain.TemplateTopic)
	for _, topic := range m.topics {
		parent := topicParentID(topic)
		children[parent] = append(children[parent], topic)
	}
	for _, topics := range children {
		sort.Slice(topics, func(i, j int) bool {
			if topics[i].Position != topics[j].Position {
				return topics[i].Position < topics[j].Position
			}
			return topics[i].ID < topics[j].ID
		})
	}
	var rows []topicRow
	var walk func(domain.TemplateTopicID, int)
	walk = func(parent domain.TemplateTopicID, depth int) {
		if depth >= domain.TopicMaxDepth {
			return
		}
		for _, topic := range children[parent] {
			rows = append(rows, topicRow{topic: topic, depth: depth})
			walk(topic.ID, depth+1)
		}
	}
	walk(m.currentParentID(), 0)
	return rows
}

func (m templateTopicsModel) rememberSelection() templateTopicsModel {
	rows := m.rows()
	if m.cursor >= 0 && m.cursor < len(rows) {
		m.selections = maps.Clone(m.selections)
		if m.selections == nil {
			m.selections = make(map[domain.TemplateTopicID]domain.TemplateTopicID)
		}
		m.selections[m.currentParentID()] = rows[m.cursor].topic.ID
	}
	return m
}

func (m templateTopicsModel) selectTopic(id domain.TemplateTopicID) templateTopicsModel {
	rows := m.rows()
	m.cursor = max(0, min(m.cursor, len(rows)-1))
	for i, row := range rows {
		if row.topic.ID == id {
			m.cursor = i
			break
		}
	}
	return m
}
