ALTER TABLE route_topics ADD COLUMN excluded INTEGER NOT NULL DEFAULT 0 CHECK (excluded IN (0, 1));

CREATE TABLE route_topic_exclusion_history (
    id INTEGER PRIMARY KEY,
    topic_id INTEGER NOT NULL REFERENCES route_topics(id) ON DELETE RESTRICT,
    before_excluded INTEGER NOT NULL CHECK (before_excluded IN (0, 1)),
    after_excluded INTEGER NOT NULL CHECK (after_excluded IN (0, 1)),
    created_at INTEGER NOT NULL,
    CHECK (before_excluded <> after_excluded)
);
CREATE INDEX route_topic_exclusion_history_topic ON route_topic_exclusion_history(topic_id, id);
