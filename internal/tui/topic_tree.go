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

// Индекс хранит позиции и ID, а данные тем остаются в одном исходном списке.
type topicTreeIndex struct {
	byID     map[domain.TemplateTopicID]int
	children map[domain.TemplateTopicID][]domain.TemplateTopicID
}

func buildTopicTreeIndex(topics []domain.TemplateTopic) topicTreeIndex {
	index := topicTreeIndex{
		byID:     make(map[domain.TemplateTopicID]int, len(topics)),
		children: make(map[domain.TemplateTopicID][]domain.TemplateTopicID),
	}
	for i, topic := range topics {
		index.byID[topic.ID] = i
		parent := topicParentID(topic)
		index.children[parent] = append(index.children[parent], topic.ID)
	}
	for _, ids := range index.children {
		sort.Slice(ids, func(i, j int) bool {
			a, b := topics[index.byID[ids[i]]], topics[index.byID[ids[j]]]
			if a.Position != b.Position {
				return a.Position < b.Position
			}
			return a.ID < b.ID
		})
	}
	return index
}

func (m templateTopicsModel) setTopics(topics []domain.TemplateTopic) templateTopicsModel {
	m.topics = topics
	m.index = buildTopicTreeIndex(topics)
	return m.reconcilePath()
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
	return m.path[len(m.path)-1]
}

func (m templateTopicsModel) topicByID(id domain.TemplateTopicID) (domain.TemplateTopic, bool) {
	if i, ok := m.index.byID[id]; ok {
		return m.topics[i], true
	}
	return domain.TemplateTopic{}, false
}

// При обновлении данных оставляем только существующую часть пути.
func (m templateTopicsModel) reconcilePath() templateTopicsModel {
	var parent domain.TemplateTopicID
	for i, id := range m.path {
		topic, ok := m.topicByID(id)
		if !ok || topicParentID(topic) != parent {
			m.path = m.path[:i]
			break
		}
		parent = id
	}
	return m
}

func (m templateTopicsModel) canCreate() bool {
	return len(m.path) < domain.TopicMaxDepth
}

func (m templateTopicsModel) rows() []topicRow {
	var rows []topicRow
	var walk func(domain.TemplateTopicID, int)
	walk = func(parent domain.TemplateTopicID, depth int) {
		if depth >= domain.TopicMaxDepth {
			return
		}
		for _, id := range m.index.children[parent] {
			topic := m.topics[m.index.byID[id]]
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
