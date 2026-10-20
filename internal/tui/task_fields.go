package tui

import (
	"encoding/csv"
	"errors"
	"io"
	"strings"
)

var errTaskTopicFormat = errors.New("укажите темы через ;, а название с ; или кавычками заключите в двойные кавычки")

func formatTaskTopics(topics []string) string {
	var labels strings.Builder
	writer := csv.NewWriter(&labels)
	writer.Comma = ';'
	_ = writer.Write(topics)
	writer.Flush()
	return strings.TrimSuffix(labels.String(), "\n")
}

func parseTaskTopics(value string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(value))
	reader.Comma, reader.TrimLeadingSpace, reader.FieldsPerRecord = ';', true, -1
	topics, err := reader.Read()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, errTaskTopicFormat
	}
	if _, err := reader.Read(); !errors.Is(err, io.EOF) {
		return nil, errTaskTopicFormat
	}
	return topics, nil
}

func taskContentPreview(content string, limit int) string {
	line := strings.SplitN(strings.TrimSpace(content), "\n", 2)[0]
	runes := []rune(line)
	if len(runes) > max(1, limit) {
		return string(runes[:max(1, limit)]) + "…"
	}
	return line
}
