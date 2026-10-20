package sqlite

import (
	"context"
	"database/sql"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) TaskPackSnapshot(ctx context.Context, ids []domain.TaskBankTaskID) (app.TaskPackSource, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return app.TaskPackSource{}, err
	}
	defer tx.Rollback()
	source := app.TaskPackSource{}
	for _, id := range ids {
		item := app.TaskPackSourceTask{}
		item.Task, err = scanBankTask(tx.QueryRowContext(ctx, `SELECT t.id, t.content_md, t.subject, t.difficulty, t.created_at, t.updated_at, i.uid FROM task_bank_tasks t JOIN task_bank_task_identity i ON i.task_id = t.id WHERE t.id = ?`, id), &item.Key)
		if err != nil {
			return app.TaskPackSource{}, err
		}
		if err := readPackTaskMetadata(ctx, tx, &item); err != nil {
			return app.TaskPackSource{}, err
		}
		source.Tasks = append(source.Tasks, item)
	}
	if err := tx.Commit(); err != nil {
		return app.TaskPackSource{}, err
	}
	return source, nil
}

func finishPackRows(rows *sql.Rows) error {
	err := rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func readPackTaskMetadata(ctx context.Context, tx *sql.Tx, item *app.TaskPackSourceTask) error {
	rows, err := tx.QueryContext(ctx, `SELECT topic_name FROM task_bank_task_topics WHERE task_id = ? ORDER BY position`, item.Task.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		item.Task.Topics = append(item.Task.Topics, name)
	}
	if err := finishPackRows(rows); err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT c.id, c.name FROM task_bank_collections c JOIN task_bank_collection_tasks m ON m.collection_id = c.id WHERE m.task_id = ? ORDER BY c.id`, item.Task.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id domain.TaskCollectionID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return err
		}
		item.Task.CollectionIDs = append(item.Task.CollectionIDs, id)
		item.Collections = append(item.Collections, name)
	}
	if err := finishPackRows(rows); err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT `+bankAttachmentColumns+` FROM task_bank_attachments WHERE task_id = ? ORDER BY id`, item.Task.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		metadata, err := scanBankAttachment(rows)
		if err != nil {
			rows.Close()
			return err
		}
		item.Attachments = append(item.Attachments, metadata)
	}
	return finishPackRows(rows)
}
