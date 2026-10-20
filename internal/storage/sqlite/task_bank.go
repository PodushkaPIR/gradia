package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	driver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const bankTaskColumns = `id, content_md, subject, difficulty, created_at, updated_at`
const bankCollectionColumns = `id, name, created_at, updated_at, (SELECT COUNT(*) FROM task_bank_collection_tasks WHERE collection_id = task_bank_collections.id)`
const bankTaskWhere = ` WHERE (? = '' OR t.subject = ?) AND (? = '' OR EXISTS (SELECT 1 FROM task_bank_task_topics p WHERE p.task_id = t.id AND p.topic_name = ?)) AND (? = 0 OR EXISTS (SELECT 1 FROM task_bank_collection_tasks m WHERE m.task_id = t.id AND m.collection_id = ?))`

func scanBankTask(row interface{ Scan(...any) error }) (domain.TaskBankTask, error) {
	var task domain.TaskBankTask
	var created, updated int64
	if err := row.Scan(&task.ID, &task.Content, &task.Subject, &task.Difficulty, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TaskBankTask{}, domain.ErrTaskBankTaskNotFound
		}
		return domain.TaskBankTask{}, err
	}
	task.CreatedAt, task.UpdatedAt = time.UnixMilli(created).UTC(), time.UnixMilli(updated).UTC()
	return task, nil
}

func scanTaskCollection(row interface{ Scan(...any) error }) (domain.TaskCollection, error) {
	var item domain.TaskCollection
	var created, updated int64
	if err := row.Scan(&item.ID, &item.Name, &created, &updated, &item.TaskCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TaskCollection{}, domain.ErrTaskCollectionNotFound
		}
		var constraint *driver.Error
		if errors.As(err, &constraint) && constraint.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return domain.TaskCollection{}, domain.ErrTaskCollectionNameExists
		}
		return domain.TaskCollection{}, err
	}
	item.CreatedAt, item.UpdatedAt = time.UnixMilli(created).UTC(), time.UnixMilli(updated).UTC()
	return item, nil
}

func (s *Store) CreateTaskCollection(ctx context.Context, name string, at time.Time) (domain.TaskCollection, error) {
	return scanTaskCollection(s.db.QueryRowContext(ctx, `INSERT INTO task_bank_collections (name, created_at, updated_at) VALUES (?, ?, ?) RETURNING `+bankCollectionColumns, name, at.UnixMilli(), at.UnixMilli()))
}

func (s *Store) RenameTaskCollection(ctx context.Context, id domain.TaskCollectionID, name string, at time.Time) (domain.TaskCollection, error) {
	return scanTaskCollection(s.db.QueryRowContext(ctx, `UPDATE task_bank_collections SET name = ?, updated_at = ? WHERE id = ? RETURNING `+bankCollectionColumns, name, at.UnixMilli(), id))
}

func (s *Store) SaveBankTask(ctx context.Context, id domain.TaskBankTaskID, input domain.TaskBankInput, at time.Time) (domain.TaskBankTask, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.TaskBankTask{}, err
	}
	defer tx.Rollback()
	for _, collection := range input.CollectionIDs {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM task_bank_collections WHERE id = ?`, collection).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.TaskBankTask{}, domain.ErrTaskCollectionNotFound
			}
			return domain.TaskBankTask{}, err
		}
	}
	var task domain.TaskBankTask
	if id == 0 {
		task, err = scanBankTask(tx.QueryRowContext(ctx, `INSERT INTO task_bank_tasks (content_md, subject, difficulty, created_at, updated_at) VALUES (?, ?, ?, ?, ?) RETURNING `+bankTaskColumns, input.Content, input.Subject, input.Difficulty, at.UnixMilli(), at.UnixMilli()))
	} else {
		task, err = scanBankTask(tx.QueryRowContext(ctx, `UPDATE task_bank_tasks SET content_md = ?, subject = ?, difficulty = ?, updated_at = ? WHERE id = ? RETURNING `+bankTaskColumns, input.Content, input.Subject, input.Difficulty, at.UnixMilli(), id))
	}
	if err != nil {
		return domain.TaskBankTask{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_bank_task_topics WHERE task_id = ?`, task.ID); err != nil {
		return domain.TaskBankTask{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_bank_collection_tasks WHERE task_id = ?`, task.ID); err != nil {
		return domain.TaskBankTask{}, err
	}
	for position, name := range input.Topics {
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_bank_task_topics (task_id, topic_name, position) VALUES (?, ?, ?)`, task.ID, name, position); err != nil {
			return domain.TaskBankTask{}, err
		}
	}
	for _, collection := range input.CollectionIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_bank_collection_tasks (collection_id, task_id) VALUES (?, ?)`, collection, task.ID); err != nil {
			return domain.TaskBankTask{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.TaskBankTask{}, err
	}
	task.Topics, task.CollectionIDs = append([]string(nil), input.Topics...), append([]domain.TaskCollectionID(nil), input.CollectionIDs...)
	return task, nil
}

func (s *Store) ListBankTasks(ctx context.Context, filter domain.TaskBankFilter) ([]domain.TaskBankTask, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if filter.CollectionID > 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM task_bank_collections WHERE id = ?`, filter.CollectionID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, domain.ErrTaskCollectionNotFound
			}
			return nil, err
		}
	}
	args := []any{filter.Subject, filter.Subject, filter.Topic, filter.Topic, filter.CollectionID, filter.CollectionID}
	rows, err := tx.QueryContext(ctx, `SELECT t.id, t.content_md, t.subject, t.difficulty, t.created_at, t.updated_at FROM task_bank_tasks t`+bankTaskWhere+` ORDER BY t.id`, args...)
	if err != nil {
		return nil, err
	}
	tasks := make([]domain.TaskBankTask, 0)
	index := make(map[domain.TaskBankTaskID]int)
	for rows.Next() {
		task, err := scanBankTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		index[task.ID] = len(tasks)
		tasks = append(tasks, task)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	rows, err = tx.QueryContext(ctx, `SELECT p.task_id, p.topic_name FROM task_bank_task_topics p JOIN task_bank_tasks t ON t.id = p.task_id`+bankTaskWhere+` ORDER BY t.id, p.position`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id domain.TaskBankTaskID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := index[id]; ok {
			tasks[i].Topics = append(tasks[i].Topics, name)
		}
	}
	err, closeErr = rows.Err(), rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	rows, err = tx.QueryContext(ctx, `SELECT m.task_id, m.collection_id FROM task_bank_collection_tasks m JOIN task_bank_tasks t ON t.id = m.task_id`+bankTaskWhere+` ORDER BY t.id, m.collection_id`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id domain.TaskBankTaskID
		var collection domain.TaskCollectionID
		if err := rows.Scan(&id, &collection); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := index[id]; ok {
			tasks[i].CollectionIDs = append(tasks[i].CollectionIDs, collection)
		}
	}
	err, closeErr = rows.Err(), rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Store) BankTaskCatalog(ctx context.Context, subject string) (app.TaskBankCatalog, error) {
	var catalog app.TaskBankCatalog
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT subject FROM task_bank_tasks ORDER BY subject`)
	if err != nil {
		return catalog, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return catalog, err
		}
		catalog.Subjects = append(catalog.Subjects, name)
	}
	err, closeErr := rows.Err(), rows.Close()
	if err != nil {
		return catalog, err
	}
	if closeErr != nil {
		return catalog, closeErr
	}
	rows, err = s.db.QueryContext(ctx, `SELECT DISTINCT p.topic_name FROM task_bank_task_topics p JOIN task_bank_tasks t ON t.id = p.task_id WHERE ? = '' OR t.subject = ? ORDER BY p.topic_name`, subject, subject)
	if err != nil {
		return catalog, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return catalog, err
		}
		catalog.Topics = append(catalog.Topics, name)
	}
	err, closeErr = rows.Err(), rows.Close()
	if err != nil {
		return catalog, err
	}
	if closeErr != nil {
		return catalog, closeErr
	}
	rows, err = s.db.QueryContext(ctx, `SELECT `+bankCollectionColumns+` FROM task_bank_collections ORDER BY name, id`)
	if err != nil {
		return catalog, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanTaskCollection(rows)
		if err != nil {
			return catalog, err
		}
		catalog.Collections = append(catalog.Collections, item)
	}
	return catalog, rows.Err()
}
