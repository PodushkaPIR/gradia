package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shesh/gradia/internal/domain"
	driver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const homeworkColumns = `id, student_id, name, created_at, updated_at,
 (SELECT COUNT(*) FROM homework_tasks WHERE homework_id = homework.id),
 (SELECT COUNT(*) FROM homework_tasks WHERE homework_id = homework.id AND completed = 1)`

const homeworkTaskColumns = `id, homework_id, content_md, difficulty, completed, position, created_at, updated_at`

func (s *Store) ListHomework(ctx context.Context, student domain.StudentID) ([]domain.Homework, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+homeworkColumns+` FROM homework WHERE student_id = ? ORDER BY id`, student)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]domain.Homework, 0)
	for rows.Next() {
		item, err := scanHomework(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s *Store) CreateHomework(ctx context.Context, student domain.StudentID, name string, at time.Time) (domain.Homework, error) {
	item, err := scanHomework(s.db.QueryRowContext(ctx, `INSERT INTO homework (student_id, name, created_at, updated_at) VALUES (?, ?, ?, ?) RETURNING `+homeworkColumns, student, name, at.UnixMilli(), at.UnixMilli()))
	var constraint *driver.Error
	if errors.As(err, &constraint) && constraint.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
		return domain.Homework{}, domain.ErrStudentNotFound
	}
	return item, err
}

func (s *Store) RenameHomework(ctx context.Context, student domain.StudentID, id domain.HomeworkID, name string, at time.Time) (domain.Homework, error) {
	return scanHomework(s.db.QueryRowContext(ctx, `UPDATE homework SET name = ?, updated_at = ? WHERE id = ? AND student_id = ? RETURNING `+homeworkColumns, name, at.UnixMilli(), id, student))
}

func scanHomework(row interface{ Scan(...any) error }) (domain.Homework, error) {
	var item domain.Homework
	var created, updated int64
	if err := row.Scan(&item.ID, &item.StudentID, &item.Name, &created, &updated, &item.TotalTasks, &item.CompletedTasks); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Homework{}, domain.ErrHomeworkNotFound
		}
		return domain.Homework{}, err
	}
	item.CreatedAt, item.UpdatedAt = time.UnixMilli(created).UTC(), time.UnixMilli(updated).UTC()
	return item, nil
}

type homeworkQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func checkHomework(ctx context.Context, query homeworkQuerier, student domain.StudentID, id domain.HomeworkID) error {
	var exists int
	if err := query.QueryRowContext(ctx, `SELECT 1 FROM homework WHERE id = ? AND student_id = ?`, id, student).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrHomeworkNotFound
		}
		return err
	}
	return nil
}

func (s *Store) ListHomeworkTasks(ctx context.Context, student domain.StudentID, id domain.HomeworkID) ([]domain.HomeworkTask, error) {
	if err := checkHomework(ctx, s.db, student, id); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.homework_id, t.content_md, t.difficulty, t.completed, t.position, t.created_at, t.updated_at, p.topic_name FROM homework_tasks t LEFT JOIN homework_task_topics p ON p.task_id = t.id WHERE t.homework_id = ? ORDER BY t.position, t.id, p.position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]domain.HomeworkTask, 0)
	for rows.Next() {
		var topic sql.NullString
		task, err := scanHomeworkTask(rows, &topic)
		if err != nil {
			return nil, err
		}
		if len(tasks) == 0 || tasks[len(tasks)-1].ID != task.ID {
			tasks = append(tasks, task)
		}
		if topic.Valid {
			tasks[len(tasks)-1].Topics = append(tasks[len(tasks)-1].Topics, topic.String)
		}
	}
	return tasks, rows.Err()
}

func scanHomeworkTask(row interface{ Scan(...any) error }, extra ...any) (domain.HomeworkTask, error) {
	var task domain.HomeworkTask
	var created, updated int64
	destination := []any{&task.ID, &task.HomeworkID, &task.Content, &task.Difficulty, &task.Completed, &task.Position, &created, &updated}
	if err := row.Scan(append(destination, extra...)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.HomeworkTask{}, domain.ErrHomeworkTaskNotFound
		}
		return domain.HomeworkTask{}, err
	}
	task.CreatedAt, task.UpdatedAt = time.UnixMilli(created).UTC(), time.UnixMilli(updated).UTC()
	return task, nil
}

func (s *Store) SaveHomeworkTask(ctx context.Context, student domain.StudentID, id domain.HomeworkID, taskID domain.HomeworkTaskID, input domain.HomeworkTaskInput, at time.Time) (domain.HomeworkTask, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	defer tx.Rollback()
	if err := checkHomework(ctx, tx, student, id); err != nil {
		return domain.HomeworkTask{}, err
	}
	var task domain.HomeworkTask
	if taskID == 0 {
		task, err = scanHomeworkTask(tx.QueryRowContext(ctx, `INSERT INTO homework_tasks (homework_id, content_md, difficulty, position, created_at, updated_at) SELECT ?, ?, ?, COALESCE(MAX(position), -1) + 1, ?, ? FROM homework_tasks WHERE homework_id = ? RETURNING `+homeworkTaskColumns, id, input.Content, input.Difficulty, at.UnixMilli(), at.UnixMilli(), id))
	} else {
		task, err = scanHomeworkTask(tx.QueryRowContext(ctx, `UPDATE homework_tasks SET content_md = ?, difficulty = ?, updated_at = ? WHERE id = ? AND homework_id = ? RETURNING `+homeworkTaskColumns, input.Content, input.Difficulty, at.UnixMilli(), taskID, id))
	}
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM homework_task_topics WHERE task_id = ?`, task.ID); err != nil {
		return domain.HomeworkTask{}, err
	}
	for position, name := range input.Topics {
		if _, err := tx.ExecContext(ctx, `INSERT INTO homework_task_topics (task_id, topic_name, position) VALUES (?, ?, ?)`, task.ID, name, position); err != nil {
			return domain.HomeworkTask{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.HomeworkTask{}, err
	}
	task.Topics = append([]string(nil), input.Topics...)
	return task, nil
}

func (s *Store) SetHomeworkTaskCompleted(ctx context.Context, student domain.StudentID, id domain.HomeworkID, taskID domain.HomeworkTaskID, completed bool, at time.Time) (domain.HomeworkTask, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	defer tx.Rollback()
	if err := checkHomework(ctx, tx, student, id); err != nil {
		return domain.HomeworkTask{}, err
	}
	task, err := scanHomeworkTask(tx.QueryRowContext(ctx, `UPDATE homework_tasks SET completed = ?, updated_at = ? WHERE id = ? AND homework_id = ? RETURNING `+homeworkTaskColumns, completed, at.UnixMilli(), taskID, id))
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT topic_name FROM homework_task_topics WHERE task_id = ? ORDER BY position`, task.ID)
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return domain.HomeworkTask{}, err
		}
		task.Topics = append(task.Topics, name)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return domain.HomeworkTask{}, err
	}
	if closeErr != nil {
		return domain.HomeworkTask{}, closeErr
	}
	if err := tx.Commit(); err != nil {
		return domain.HomeworkTask{}, err
	}
	return task, nil
}
