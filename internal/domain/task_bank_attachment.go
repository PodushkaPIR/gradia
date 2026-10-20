package domain

import (
	"errors"
	"time"
)

type TaskBankAttachmentID int64

type TaskBankAttachment struct {
	ID                     TaskBankAttachmentID
	TaskID                 TaskBankTaskID
	Name, MediaType, Pages string
	Size                   int64
	CreatedAt              time.Time
}

var ErrTaskBankAttachmentNotFound = errors.New("файл не найден у выбранной задачи банка")
