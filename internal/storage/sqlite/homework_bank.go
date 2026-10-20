package sqlite

import (
	"context"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) SnapshotBankTaskToHomework(ctx context.Context, student domain.StudentID, homework domain.HomeworkID, source domain.TaskBankTaskID, at time.Time) (domain.HomeworkTask, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	defer tx.Rollback()
	if err := checkHomework(ctx, tx, student, homework); err != nil {
		return domain.HomeworkTask{}, err
	}
	bankTask, err := scanBankTask(tx.QueryRowContext(ctx, `SELECT `+bankTaskColumns+` FROM task_bank_tasks WHERE id = ?`, source))
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	task, err := scanHomeworkTask(tx.QueryRowContext(ctx, `INSERT INTO homework_tasks (homework_id, content_md, difficulty, position, created_at, updated_at) SELECT ?, ?, ?, COALESCE(MAX(position), -1) + 1, ?, ? FROM homework_tasks WHERE homework_id = ? RETURNING `+homeworkTaskColumns, homework, bankTask.Content, bankTask.Difficulty, at.UnixMilli(), at.UnixMilli(), homework))
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO homework_task_topics (task_id, topic_name, position) SELECT ?, topic_name, position FROM task_bank_task_topics WHERE task_id = ? ORDER BY position`, task.ID, source); err != nil {
		return domain.HomeworkTask{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO homework_attachments (task_id, name, media_type, pages, content, created_at) SELECT ?, name, media_type, pages, content, ? FROM task_bank_attachments WHERE task_id = ? ORDER BY id`, task.ID, at.UnixMilli(), source); err != nil {
		return domain.HomeworkTask{}, err
	}
	if err := readHomeworkTaskTopics(ctx, tx, &task); err != nil {
		return domain.HomeworkTask{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.HomeworkTask{}, err
	}
	return task, nil
}
