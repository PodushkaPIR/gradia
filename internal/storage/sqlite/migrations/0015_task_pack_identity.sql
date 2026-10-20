CREATE TABLE task_bank_task_identity (
    task_id INTEGER PRIMARY KEY REFERENCES task_bank_tasks(id) ON DELETE CASCADE,
    uid TEXT NOT NULL UNIQUE CHECK (length(uid) = 32 AND uid NOT GLOB '*[^0-9a-f]*')
);
INSERT INTO task_bank_task_identity (task_id, uid)
SELECT id, lower(hex(randomblob(16))) FROM task_bank_tasks;

CREATE TRIGGER create_task_bank_identity AFTER INSERT ON task_bank_tasks
BEGIN
    INSERT INTO task_bank_task_identity (task_id, uid) VALUES (NEW.id, lower(hex(randomblob(16))));
END;
