CREATE TRIGGER limit_route_topic_depth BEFORE INSERT ON route_topics
WHEN NEW.parent_id IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'route_topic_depth_exceeded') WHERE (
        WITH RECURSIVE ancestors(id, parent_id) AS (
            SELECT id, parent_id FROM route_topics
            WHERE route_id = NEW.route_id AND id = NEW.parent_id
            UNION
            SELECT t.id, t.parent_id FROM route_topics t
            JOIN ancestors a ON t.id = a.parent_id
            WHERE t.route_id = NEW.route_id
        )
        SELECT COUNT(*) FROM ancestors
    ) >= 5;
END;
