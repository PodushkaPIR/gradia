CREATE TABLE route_topic_progress (
    topic_id INTEGER PRIMARY KEY REFERENCES route_topics(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN ('not_started', 'learning', 'needs_practice', 'mastered')),
    updated_at INTEGER NOT NULL
);
