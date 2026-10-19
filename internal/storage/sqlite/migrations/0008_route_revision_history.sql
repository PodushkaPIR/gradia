ALTER TABLE route_topic_progress ADD COLUMN revision INTEGER NOT NULL DEFAULT 0
    CHECK (revision IN (0, 1) AND (revision = 0 OR status = 'mastered'));

CREATE TABLE route_progress_history (
    id INTEGER PRIMARY KEY,
    topic_id INTEGER NOT NULL REFERENCES route_topics(id) ON DELETE RESTRICT,
    before_status TEXT NOT NULL CHECK (before_status IN ('not_started', 'learning', 'needs_practice', 'mastered')),
    after_status TEXT NOT NULL CHECK (after_status IN ('not_started', 'learning', 'needs_practice', 'mastered')),
    before_revision INTEGER NOT NULL CHECK (before_revision IN (0, 1) AND (before_revision = 0 OR before_status = 'mastered')),
    after_revision INTEGER NOT NULL CHECK (after_revision IN (0, 1) AND (after_revision = 0 OR after_status = 'mastered')),
    created_at INTEGER NOT NULL,
    CHECK (before_status <> after_status OR before_revision <> after_revision)
);
CREATE INDEX route_progress_history_topic ON route_progress_history(topic_id, id);
