CREATE TABLE task_bank_attachments (
    id INTEGER PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES task_bank_tasks(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (length(name) > 0),
    media_type TEXT NOT NULL CHECK (media_type = 'application/pdf' OR media_type LIKE 'image/%'),
    pages TEXT NOT NULL DEFAULT '' CHECK (pages = '' OR media_type = 'application/pdf'),
    content BLOB NOT NULL CHECK (length(content) > 0),
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_task_bank_attachments_task ON task_bank_attachments(task_id, id);
