package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
)

const bankAttachmentColumns = `id, task_id, name, media_type, pages, length(content), created_at`

func scanBankAttachment(row interface{ Scan(...any) error }, extra ...any) (domain.TaskBankAttachment, error) {
	var item domain.TaskBankAttachment
	var created int64
	columns := []any{&item.ID, &item.TaskID, &item.Name, &item.MediaType, &item.Pages, &item.Size, &created}
	if err := row.Scan(append(columns, extra...)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TaskBankAttachment{}, domain.ErrTaskBankAttachmentNotFound
		}
		return domain.TaskBankAttachment{}, err
	}
	item.CreatedAt = time.UnixMilli(created).UTC()
	return item, nil
}

func (s *Store) ListBankAttachments(ctx context.Context, task domain.TaskBankTaskID) ([]domain.TaskBankAttachment, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM task_bank_tasks WHERE id = ?`, task).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTaskBankTaskNotFound
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+bankAttachmentColumns+` FROM task_bank_attachments WHERE task_id = ? ORDER BY id`, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.TaskBankAttachment, 0)
	for rows.Next() {
		item, err := scanBankAttachment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AddBankAttachment(ctx context.Context, metadata domain.TaskBankAttachment, data []byte) (domain.TaskBankAttachment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.TaskBankAttachment{}, err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM task_bank_tasks WHERE id = ?`, metadata.TaskID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TaskBankAttachment{}, domain.ErrTaskBankTaskNotFound
		}
		return domain.TaskBankAttachment{}, err
	}
	item, err := scanBankAttachment(tx.QueryRowContext(ctx, `INSERT INTO task_bank_attachments (task_id, name, media_type, pages, content, created_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING `+bankAttachmentColumns, metadata.TaskID, metadata.Name, metadata.MediaType, metadata.Pages, data, metadata.CreatedAt.UnixMilli()))
	if err != nil {
		return domain.TaskBankAttachment{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.TaskBankAttachment{}, err
	}
	return item, nil
}

func (s *Store) GetBankAttachment(ctx context.Context, task domain.TaskBankTaskID, id domain.TaskBankAttachmentID) (domain.TaskBankAttachment, []byte, error) {
	var data []byte
	metadata, err := scanBankAttachment(s.db.QueryRowContext(ctx, `SELECT `+bankAttachmentColumns+`, content FROM task_bank_attachments WHERE id = ? AND task_id = ?`, id, task), &data)
	return metadata, data, err
}
