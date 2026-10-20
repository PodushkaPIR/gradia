package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

// ImportTaskPack makes identity checks and all writes in one transaction. Existing
// identities are never updated, even when imported content differs.
func (s *Store) ImportTaskPack(ctx context.Context, pack app.TaskPackData, at time.Time) (app.TaskPackImportResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return app.TaskPackImportResult{}, err
	}
	defer tx.Rollback()
	var result app.TaskPackImportResult
	for _, task := range pack.Tasks {
		var id domain.TaskBankTaskID
		err := tx.QueryRowContext(ctx, `SELECT task_id FROM task_bank_task_identity WHERE uid = ?`, task.ID).Scan(&id)
		if err == nil {
			fingerprint, err := localPackFingerprint(ctx, tx, id)
			if err != nil {
				return app.TaskPackImportResult{}, err
			}
			if fingerprint == task.SHA256 {
				result.Duplicates++
			} else {
				result.Conflicts++
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return app.TaskPackImportResult{}, err
		}
		if err := insertPackTask(ctx, tx, task, pack.Assets, at); err != nil {
			return app.TaskPackImportResult{}, err
		}
		result.Imported++
	}
	if err := tx.Commit(); err != nil {
		return app.TaskPackImportResult{}, err
	}
	return result, nil
}

func localPackFingerprint(ctx context.Context, tx *sql.Tx, id domain.TaskBankTaskID) (string, error) {
	item := app.TaskPackSourceTask{}
	var err error
	item.Task, err = scanBankTask(tx.QueryRowContext(ctx, `SELECT `+bankTaskColumns+` FROM task_bank_tasks WHERE id = ?`, id))
	if err != nil {
		return "", err
	}
	if err := readPackTaskMetadata(ctx, tx, &item); err != nil {
		return "", err
	}
	task := app.TaskPackTask{Content: item.Task.Content, Subject: item.Task.Subject, Topics: item.Task.Topics, Difficulty: item.Task.Difficulty, Collections: item.Collections}
	for _, metadata := range item.Attachments {
		var data []byte
		if err := tx.QueryRowContext(ctx, `SELECT content FROM task_bank_attachments WHERE id = ? AND task_id = ?`, metadata.ID, id).Scan(&data); err != nil {
			return "", err
		}
		hash := sha256.Sum256(data)
		task.Attachments = append(task.Attachments, app.TaskPackAttachment{Name: metadata.Name, MediaType: metadata.MediaType, Pages: metadata.Pages, Size: metadata.Size, SHA256: hex.EncodeToString(hash[:])})
	}
	return app.TaskPackFingerprint(task), ctx.Err()
}

func insertPackTask(ctx context.Context, tx *sql.Tx, task app.TaskPackTask, assets map[string][]byte, at time.Time) error {
	var id domain.TaskBankTaskID
	if err := tx.QueryRowContext(ctx, `INSERT INTO task_bank_tasks (content_md, subject, difficulty, created_at, updated_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, task.Content, task.Subject, task.Difficulty, at.UnixMilli(), at.UnixMilli()).Scan(&id); err != nil {
		return err
	}
	// The normal insert trigger supplies a random identity; imports retain the source identity.
	if _, err := tx.ExecContext(ctx, `UPDATE task_bank_task_identity SET uid = ? WHERE task_id = ?`, task.ID, id); err != nil {
		return err
	}
	for position, name := range task.Topics {
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_bank_task_topics (task_id, topic_name, position) VALUES (?, ?, ?)`, id, name, position); err != nil {
			return err
		}
	}
	for _, name := range task.Collections {
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_bank_collections (name, created_at, updated_at) VALUES (?, ?, ?) ON CONFLICT(name) DO NOTHING`, name, at.UnixMilli(), at.UnixMilli()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_bank_collection_tasks (collection_id, task_id) SELECT id, ? FROM task_bank_collections WHERE name = ?`, id, name); err != nil {
			return err
		}
	}
	for _, attachment := range task.Attachments {
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_bank_attachments (task_id, name, media_type, pages, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`, id, attachment.Name, attachment.MediaType, attachment.Pages, assets[attachment.Path], at.UnixMilli()); err != nil {
			return err
		}
	}
	return nil
}
