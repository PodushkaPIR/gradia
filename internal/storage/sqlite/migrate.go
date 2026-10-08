package sqlite

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed migrations/0001_students.sql
var studentsMigration string

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY CHECK (version > 0)
	)`); err != nil {
		return err
	}

	var version int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}

	migrations := []string{studentsMigration}
	if version > len(migrations) {
		return fmt.Errorf("версия БД %d новее поддерживаемой %d", version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			return fmt.Errorf("миграция %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, i+1); err != nil {
			return err
		}
	}
	return tx.Commit()
}
