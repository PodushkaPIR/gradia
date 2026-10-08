CREATE TABLE template_topics (
    id          INTEGER PRIMARY KEY,
    template_id INTEGER NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
    name        TEXT NOT NULL CHECK (length(trim(name)) > 0),
    position    INTEGER NOT NULL CHECK (position >= 0),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE (template_id, position)
);
