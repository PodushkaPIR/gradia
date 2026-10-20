package taskpacks

import "github.com/shesh/gradia/internal/app"

const Format = "gradia-task-pack"
const Version = 1

type Manifest struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Tasks   []Task `json:"tasks"`
}

type Task = app.TaskPackTask
type Attachment = app.TaskPackAttachment

func TaskFingerprint(task Task) string { return app.TaskPackFingerprint(task) }
