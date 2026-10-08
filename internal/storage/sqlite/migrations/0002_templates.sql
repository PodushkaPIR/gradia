CREATE TABLE templates (
    id             INTEGER PRIMARY KEY,
    name           TEXT NOT NULL CHECK (length(trim(name)) > 0),
    description_md TEXT NOT NULL DEFAULT '',
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);
