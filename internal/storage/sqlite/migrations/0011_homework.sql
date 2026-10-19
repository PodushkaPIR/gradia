CREATE TABLE homework (
    id INTEGER PRIMARY KEY,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_homework_student ON homework(student_id, id);

CREATE TABLE homework_tasks (
    id INTEGER PRIMARY KEY,
    homework_id INTEGER NOT NULL REFERENCES homework(id) ON DELETE RESTRICT,
    content_md TEXT NOT NULL CHECK (length(trim(content_md)) > 0),
    difficulty TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    completed INTEGER NOT NULL DEFAULT 0 CHECK (completed IN (0, 1)),
    position INTEGER NOT NULL CHECK (position >= 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(homework_id, position)
);

CREATE TABLE homework_task_topics (
    task_id INTEGER NOT NULL REFERENCES homework_tasks(id) ON DELETE CASCADE,
    topic_name TEXT NOT NULL CHECK (length(trim(topic_name)) > 0),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY(task_id, topic_name),
    UNIQUE(task_id, position)
);
