-- Migration 018: Knowledge tree search
-- Recursive CTE to collect all descendants of a root shard,
-- then hybrid search (full-text + vector) within that subtree.

CREATE OR REPLACE FUNCTION knowledge_tree_search(
    p_project TEXT,
    p_root_id TEXT,
    p_query TEXT DEFAULT NULL,
    p_query_embedding vector(768) DEFAULT NULL,
    p_include_closed BOOLEAN DEFAULT FALSE,
    p_limit INT DEFAULT 20,
    p_min_similarity FLOAT DEFAULT 0.3
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    type TEXT,
    status TEXT,
    description TEXT,
    text_rank FLOAT,
    similarity FLOAT,
    combined_score FLOAT,
    depth INT,
    parent_path TEXT[],
    snippet TEXT,
    labels TEXT[],
    created_at TIMESTAMPTZ
) AS $$
WITH RECURSIVE tree AS (
    -- Direct children of root (via parent_id or child-of edge)
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
    -- Recursive descendants
    SELECT c.id, t.path || t.id, t.depth + 1
    FROM tree t
    JOIN shards c ON c.project = p_project
      AND (p_include_closed OR c.status != 'closed')
      AND c.id IN (
          SELECT s.id FROM shards s WHERE s.parent_id = t.id
          UNION
          SELECT e.from_id FROM edges e WHERE e.to_id = t.id AND e.edge_type = 'child-of'
      )
    WHERE t.depth < 10
),
scored AS (
    SELECT
        s.id, s.title, s.type, s.status, s.description,
        CASE WHEN p_query IS NOT NULL AND s.search_vector IS NOT NULL
                  AND s.search_vector @@ plainto_tsquery('english', p_query)
             THEN ts_rank(s.search_vector, plainto_tsquery('english', p_query))
             ELSE 0 END AS text_rank,
        CASE WHEN p_query_embedding IS NOT NULL AND s.embedding IS NOT NULL
             THEN (1 - (s.embedding <=> p_query_embedding))::FLOAT
             ELSE 0 END AS similarity,
        t.depth,
        t.path AS parent_path,
        LEFT(s.content, 300) AS snippet,
        ARRAY(SELECT l.label FROM labels l WHERE l.shard_id = s.id) AS labels,
        s.created_at
    FROM tree t
    JOIN shards s ON s.id = t.id
)
SELECT
    id, title, type, status, description,
    text_rank, similarity,
    CASE
        WHEN p_query_embedding IS NOT NULL AND p_query IS NOT NULL
            THEN (similarity * 0.7 + text_rank * 0.3)
        WHEN p_query_embedding IS NOT NULL
            THEN similarity
        ELSE text_rank
    END AS combined_score,
    depth, parent_path, snippet, labels, created_at
FROM scored
WHERE CASE
    -- Text-only: require text match
    WHEN p_query IS NOT NULL AND p_query_embedding IS NULL THEN text_rank > 0
    -- Semantic-only: require similarity threshold
    WHEN p_query IS NULL AND p_query_embedding IS NOT NULL THEN similarity >= p_min_similarity
    -- Hybrid: require either text match or similarity threshold
    WHEN p_query IS NOT NULL AND p_query_embedding IS NOT NULL THEN text_rank > 0 OR similarity >= p_min_similarity
    -- No query (browse all): return everything
    ELSE TRUE
END
ORDER BY combined_score DESC
LIMIT p_limit;
$$ LANGUAGE sql STABLE;

-- knowledge_tree_list: browse the tree structure from a root without searching
CREATE OR REPLACE FUNCTION knowledge_tree_list(
    p_project TEXT,
    p_root_id TEXT,
    p_max_depth INT DEFAULT 1,
    p_include_closed BOOLEAN DEFAULT FALSE
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    type TEXT,
    status TEXT,
    description TEXT,
    child_count BIGINT,
    depth INT,
    parent_path TEXT[],
    labels TEXT[],
    created_at TIMESTAMPTZ
) AS $$
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
    s.created_at
FROM tree t
JOIN shards s ON s.id = t.id
ORDER BY t.depth, s.title;
$$ LANGUAGE sql STABLE;
