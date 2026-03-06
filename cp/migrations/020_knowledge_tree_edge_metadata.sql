-- 020: Add edge metadata (trigger, description, ordering) to knowledge_tree_list
-- The function now joins the edges table to return edge trigger text,
-- edge description, and ordering for each node in the tree.

DROP FUNCTION IF EXISTS knowledge_tree_list(text, text, integer, boolean);

CREATE FUNCTION knowledge_tree_list(
    p_project TEXT,
    p_root_id TEXT,
    p_max_depth INT DEFAULT 1,
    p_include_closed BOOLEAN DEFAULT FALSE
)
RETURNS TABLE(
    id TEXT,
    title TEXT,
    type TEXT,
    status TEXT,
    description TEXT,
    child_count BIGINT,
    depth INT,
    parent_path TEXT[],
    labels TEXT[],
    created_at TIMESTAMPTZ,
    edge_trigger TEXT,
    edge_description TEXT,
    edge_ordering INT
)
LANGUAGE sql STABLE AS $$
WITH RECURSIVE tree AS (
    SELECT c.id, ARRAY[p_root_id]::TEXT[] AS path, 0 AS depth
    FROM shards c
    WHERE c.project = p_project
      AND (p_include_closed OR c.status != 'closed')
      AND c.id IN (
          SELECT s.id FROM shards s WHERE s.parent_id = p_root_id
          UNION
          SELECT e.from_id FROM edges e WHERE e.to_id = p_root_id AND e.edge_type = 'child-of'
      )
    UNION ALL
    SELECT c.id, t.path || t.id, t.depth + 1
    FROM tree t
    JOIN shards c ON c.project = p_project
      AND (p_include_closed OR c.status != 'closed')
      AND c.id IN (
          SELECT s.id FROM shards s WHERE s.parent_id = t.id
          UNION
          SELECT e.from_id FROM edges e WHERE e.to_id = t.id AND e.edge_type = 'child-of'
      )
    WHERE t.depth < p_max_depth
)
SELECT
    s.id, s.title, s.type, s.status, s.description,
    _shard_child_count(s.id, p_include_closed),
    t.depth, t.path,
    ARRAY(SELECT l.label FROM labels l WHERE l.shard_id = s.id),
    s.created_at,
    (e.metadata->>'trigger')::TEXT,
    (e.metadata->>'description')::TEXT,
    (e.metadata->>'ordering')::INT
FROM tree t
JOIN shards s ON s.id = t.id
LEFT JOIN edges e ON e.from_id = t.id
    AND e.to_id = t.path[array_length(t.path, 1)]
    AND e.edge_type = 'child-of'
ORDER BY t.depth, COALESCE((e.metadata->>'ordering')::INT, 999), s.title;
$$;
