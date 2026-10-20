package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/shesh/gradia/internal/domain"
)

// TaskPackTask is the portable representation shared by import and export ports.
// Local row IDs and timestamps are deliberately absent.
type TaskPackTask struct {
	ID          string                `json:"id"`
	SHA256      string                `json:"sha256"`
	Content     string                `json:"content"`
	Subject     string                `json:"subject"`
	Topics      []string              `json:"topics"`
	Difficulty  domain.TaskDifficulty `json:"difficulty"`
	Collections []string              `json:"collections"`
	Attachments []TaskPackAttachment  `json:"attachments"`
}

type TaskPackAttachment struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Pages     string `json:"pages,omitempty"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
}

// TaskPackFingerprint compares logical content, independently of identity and archive layout.
func TaskPackFingerprint(task TaskPackTask) string {
	task.ID, task.SHA256 = "", ""
	task.Topics, task.Collections, task.Attachments = append([]string{}, task.Topics...), append([]string{}, task.Collections...), append([]TaskPackAttachment{}, task.Attachments...)
	slices.Sort(task.Topics)
	slices.Sort(task.Collections)
	for i := range task.Attachments {
		task.Attachments[i].Path = ""
	}
	slices.SortFunc(task.Attachments, func(a, b TaskPackAttachment) int {
		left, _ := json.Marshal(a)
		right, _ := json.Marshal(b)
		return slices.Compare(left, right)
	})
	data, _ := json.Marshal(task) // This data-only shape cannot contain unsupported JSON values.
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
