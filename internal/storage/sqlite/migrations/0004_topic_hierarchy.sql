CREATE TABLE template_topics_new (
    id          INTEGER PRIMARY KEY,
    template_id INTEGER NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
    parent_id   INTEGER,
    name        TEXT NOT NULL CHECK (length(trim(name)) > 0),
    position    INTEGER NOT NULL CHECK (position >= 0),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE (template_id, id),
    FOREIGN KEY (template_id, parent_id)
        REFERENCES template_topics_new(template_id, id) ON DELETE RESTRICT,
    CHECK (parent_id IS NULL OR parent_id <> id)
);

INSERT INTO template_topics_new (id, template_id, name, position, created_at, updated_at)
SELECT id, template_id, name, position, created_at, updated_at FROM template_topics;

DROP TABLE template_topics;
ALTER TABLE template_topics_new RENAME TO template_topics;

CREATE UNIQUE INDEX idx_topic_root_position
    ON template_topics(template_id, position) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX idx_topic_child_position
    ON template_topics(template_id, parent_id, position) WHERE parent_id IS NOT NULL;

CREATE TRIGGER limit_topic_depth BEFORE INSERT ON template_topics
WHEN NEW.parent_id IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'topic_depth_exceeded') WHERE (
        WITH RECURSIVE ancestors(id, parent_id) AS (
            SELECT id, parent_id FROM template_topics
            WHERE template_id = NEW.template_id AND id = NEW.parent_id
            UNION
            SELECT t.id, t.parent_id FROM template_topics t
            JOIN ancestors a ON t.id = a.parent_id
            WHERE t.template_id = NEW.template_id
        )
        SELECT COUNT(*) FROM ancestors
    ) >= 5;
END;
