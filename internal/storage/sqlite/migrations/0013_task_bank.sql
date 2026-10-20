CREATE TABLE task_bank_tasks (
    id INTEGER PRIMARY KEY,
    content_md TEXT NOT NULL CHECK (length(trim(content_md)) > 0),
    subject TEXT NOT NULL CHECK (length(trim(subject)) > 0),
    difficulty TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_task_bank_subject ON task_bank_tasks(subject, id);

CREATE TABLE task_bank_task_topics (
    task_id INTEGER NOT NULL REFERENCES task_bank_tasks(id) ON DELETE CASCADE,
    topic_name TEXT NOT NULL CHECK (length(trim(topic_name)) > 0),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY(task_id, topic_name),
    UNIQUE(task_id, position)
);
CREATE INDEX idx_task_bank_topic_name ON task_bank_task_topics(topic_name, task_id);

CREATE TABLE task_bank_collections (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE CHECK (length(trim(name)) > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE task_bank_collection_tasks (
    collection_id INTEGER NOT NULL REFERENCES task_bank_collections(id) ON DELETE RESTRICT,
    task_id INTEGER NOT NULL REFERENCES task_bank_tasks(id) ON DELETE CASCADE,
    PRIMARY KEY(collection_id, task_id)
);
CREATE INDEX idx_task_bank_memberships_task ON task_bank_collection_tasks(task_id, collection_id);
