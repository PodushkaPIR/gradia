package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

const homeworkAttachmentColumns = `id, task_id, name, media_type, pages, length(content), created_at`

func checkHomeworkAttachmentTask(ctx context.Context, query homeworkQuerier, student domain.StudentID, homework domain.HomeworkID, task domain.HomeworkTaskID) error {
	if err := checkHomework(ctx, query, student, homework); err != nil {
		return err
	}
	var exists int
	if err := query.QueryRowContext(ctx, `SELECT 1 FROM homework_tasks WHERE id = ? AND homework_id = ?`, task, homework).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrHomeworkTaskNotFound
		}
		return err
	}
	return nil
}

func scanHomeworkAttachment(row interface{ Scan(...any) error }, extra ...any) (domain.HomeworkAttachment, error) {
	var item domain.HomeworkAttachment
	var created int64
	columns := []any{&item.ID, &item.TaskID, &item.Name, &item.MediaType, &item.Pages, &item.Size, &created}
	if err := row.Scan(append(columns, extra...)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.HomeworkAttachment{}, domain.ErrHomeworkAttachmentNotFound
		}
		return domain.HomeworkAttachment{}, err
	}
	item.CreatedAt = time.UnixMilli(created).UTC()
	return item, nil
}

func (s *Store) ListHomeworkAttachments(ctx context.Context, student domain.StudentID, homework domain.HomeworkID, task domain.HomeworkTaskID) ([]domain.HomeworkAttachment, error) {
	if err := checkHomeworkAttachmentTask(ctx, s.db, student, homework, task); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+homeworkAttachmentColumns+` FROM homework_attachments WHERE task_id = ? ORDER BY id`, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.HomeworkAttachment, 0)
	for rows.Next() {
		item, err := scanHomeworkAttachment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AddHomeworkAttachment(ctx context.Context, student domain.StudentID, homework domain.HomeworkID, metadata domain.HomeworkAttachment, data []byte) (domain.HomeworkAttachment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.HomeworkAttachment{}, err
	}
	defer tx.Rollback()
	if err := checkHomeworkAttachmentTask(ctx, tx, student, homework, metadata.TaskID); err != nil {
		return domain.HomeworkAttachment{}, err
	}
	item, err := scanHomeworkAttachment(tx.QueryRowContext(ctx, `INSERT INTO homework_attachments (task_id, name, media_type, pages, content, created_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING `+homeworkAttachmentColumns, metadata.TaskID, metadata.Name, metadata.MediaType, metadata.Pages, data, metadata.CreatedAt.UnixMilli()))
	if err != nil {
		return domain.HomeworkAttachment{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.HomeworkAttachment{}, err
	}
	return item, nil
}

func (s *Store) GetHomeworkAttachment(ctx context.Context, student domain.StudentID, homework domain.HomeworkID, task domain.HomeworkTaskID, id domain.HomeworkAttachmentID) (domain.HomeworkAttachment, []byte, error) {
	var data []byte
	metadata, err := scanHomeworkAttachment(s.db.QueryRowContext(ctx, `SELECT `+homeworkAttachmentColumns+`, content FROM homework_attachments WHERE id = ? AND task_id = ? AND task_id IN (SELECT t.id FROM homework_tasks t JOIN homework h ON h.id = t.homework_id WHERE h.student_id = ? AND h.id = ?)`, id, task, student, homework), &data)
	return metadata, data, err
}
