package sqlite

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed migrations/0001_students.sql
var studentsMigration string

//go:embed migrations/0002_templates.sql
var templatesMigration string

//go:embed migrations/0003_template_topics.sql
var templateTopicsMigration string

//go:embed migrations/0004_topic_hierarchy.sql
var topicHierarchyMigration string

//go:embed migrations/0005_student_routes.sql
var studentRoutesMigration string

//go:embed migrations/0006_route_progress.sql
var routeProgressMigration string

//go:embed migrations/0007_route_topic_notes.sql
var routeTopicNotesMigration string

//go:embed migrations/0008_route_revision_history.sql
var routeRevisionHistoryMigration string

//go:embed migrations/0009_route_topic_exclusions.sql
var routeTopicExclusionsMigration string

//go:embed migrations/0010_route_topic_depth.sql
var routeTopicDepthMigration string

//go:embed migrations/0011_homework.sql
var homeworkMigration string

//go:embed migrations/0012_homework_attachments.sql
var homeworkAttachmentsMigration string

//go:embed migrations/0013_task_bank.sql
var taskBankMigration string

//go:embed migrations/0014_task_bank_attachments.sql
var taskBankAttachmentsMigration string

var schemaMigrations = []string{studentsMigration, templatesMigration, templateTopicsMigration, topicHierarchyMigration, studentRoutesMigration, routeProgressMigration, routeTopicNotesMigration, routeRevisionHistoryMigration, routeTopicExclusionsMigration, routeTopicDepthMigration, homeworkMigration, homeworkAttachmentsMigration, taskBankMigration, taskBankAttachmentsMigration}

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

	migrations := schemaMigrations
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
