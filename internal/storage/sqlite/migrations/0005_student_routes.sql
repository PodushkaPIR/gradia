CREATE TABLE student_routes (
    id INTEGER PRIMARY KEY,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
    source_template_id INTEGER REFERENCES templates(id) ON DELETE SET NULL,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    description_md TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_routes_student ON student_routes(student_id, id);

CREATE TABLE route_topics (
    id INTEGER PRIMARY KEY,
    route_id INTEGER NOT NULL REFERENCES student_routes(id) ON DELETE RESTRICT,
    parent_id INTEGER,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    position INTEGER NOT NULL CHECK (position >= 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(route_id, id),
    FOREIGN KEY(route_id, parent_id) REFERENCES route_topics(route_id, id) ON DELETE RESTRICT,
    CHECK(parent_id IS NULL OR parent_id <> id)
);
CREATE UNIQUE INDEX idx_route_root_position ON route_topics(route_id, position) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX idx_route_child_position ON route_topics(route_id, parent_id, position) WHERE parent_id IS NOT NULL;
