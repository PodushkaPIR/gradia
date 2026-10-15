package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	driver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/shesh/gradia/internal/domain"
)

func (s *Store) ListStudents(ctx context.Context) ([]domain.Student, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, grade, notes_md, created_at, updated_at
		FROM students ORDER BY name, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := make([]domain.Student, 0)
	for rows.Next() {
		student, err := scanStudent(rows)
		if err != nil {
			return nil, err
		}
		students = append(students, student)
	}
	return students, rows.Err()
}

func (s *Store) CreateStudent(ctx context.Context, student domain.Student) (domain.Student, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO students (name, grade, notes_md, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, student.Name, student.Grade, student.Notes, student.CreatedAt.UnixMilli(), student.UpdatedAt.UnixMilli())
	if err != nil {
		return domain.Student{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Student{}, err
	}
	student.ID = domain.StudentID(id)
	return student, nil
}

func (s *Store) RenameStudent(ctx context.Context, id domain.StudentID, name string, updatedAt time.Time) (domain.Student, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE students SET name = ?, updated_at = ? WHERE id = ?
		RETURNING id, name, grade, notes_md, created_at, updated_at
	`, name, updatedAt.UnixMilli(), id)
	student, err := scanStudent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Student{}, domain.ErrStudentNotFound
	}
	return student, err
}

func (s *Store) DeleteStudent(ctx context.Context, id domain.StudentID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM students WHERE id = ?`, id)
	if err != nil {
		var sqliteErr *driver.Error
		if errors.As(err, &sqliteErr) {
			switch sqliteErr.Code() {
			case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY, sqlite3.SQLITE_CONSTRAINT_TRIGGER:
				// SQLite также возвращает CONSTRAINT_TRIGGER для ON DELETE RESTRICT.
				return domain.ErrStudentInUse
			}
		}
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrStudentNotFound
	}
	return nil
}

func scanStudent(row interface{ Scan(...any) error }) (domain.Student, error) {
	var student domain.Student
	var grade sql.NullString
	var created, updated int64
	if err := row.Scan(&student.ID, &student.Name, &grade, &student.Notes, &created, &updated); err != nil {
		return domain.Student{}, err
	}
	if grade.Valid {
		student.Grade = &grade.String
	}
	student.CreatedAt = time.UnixMilli(created).UTC()
	student.UpdatedAt = time.UnixMilli(updated).UTC()
	return student, nil
}
