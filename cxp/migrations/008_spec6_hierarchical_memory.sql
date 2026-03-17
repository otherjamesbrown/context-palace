-- SPEC-6: Hierarchical Memory
-- Adds: memory tree navigation, access telemetry, amended list_shards for --roots

-- Amend list_shards() — add 10th param p_parent_id_null for --roots filter
CREATE OR REPLACE FUNCTION list_shards(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since TIMESTAMPTZ DEFAULT NULL,
    p_limit INT DEFAULT 50,
    p_offset INT DEFAULT 0,
    p_parent_id_null BOOLEAN DEFAULT FALSE
) RETURNS TABLE (
    id TEXT, title TEXT, type TEXT, status TEXT, creator TEXT,
    labels TEXT[], created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, snippet TEXT
) AS $$
    SELECT
        s.id, s.title, s.type, s.status, s.creator,
        s.labels, s.created_at, s.updated_at,
        left(s.content, 200) AS snippet
    FROM shards s
    WHERE s.project = p_project
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_labels IS NULL OR s.labels @> p_labels)
      AND (p_creator IS NULL OR s.creator = p_creator)
      AND (p_search IS NULL OR s.search_vector @@ plainto_tsquery('english', p_search))
      AND (p_since IS NULL OR s.created_at >= p_since)
      AND (NOT p_parent_id_null OR s.parent_id IS NULL)
    ORDER BY s.created_at DESC
    LIMIT p_limit OFFSET p_offset;
$$ LANGUAGE sql STABLE;

-- Amend list_shards_count() — same p_parent_id_null param for consistency
CREATE OR REPLACE FUNCTION list_shards_count(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since TIMESTAMPTZ DEFAULT NULL,
    p_parent_id_null BOOLEAN DEFAULT FALSE
) RETURNS INT AS $$
    SELECT count(*)::int
    FROM shards s
    WHERE s.project = p_project
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_labels IS NULL OR s.labels @> p_labels)
      AND (p_creator IS NULL OR s.creator = p_creator)
      AND (p_search IS NULL OR s.search_vector @@ plainto_tsquery('english', p_search))
      AND (p_since IS NULL OR s.created_at >= p_since)
      AND (NOT p_parent_id_null OR s.parent_id IS NULL);
$$ LANGUAGE sql STABLE;

-- Get memory tree (recursive), with summary from child-of edge metadata
CREATE OR REPLACE FUNCTION memory_tree(
    p_project TEXT,
    p_root_id TEXT DEFAULT NULL
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    parent_id TEXT,
    depth INT,
    status TEXT,
    labels TEXT[],
    access_count INT,
    last_accessed TIMESTAMPTZ,
    child_count INT,
    summary TEXT
) AS $$
    WITH RECURSIVE tree AS (
        SELECT
            s.id, s.title, s.parent_id, 0 AS depth,
            s.status, s.labels, s.metadata, s.created_at
        FROM shards s
        WHERE s.project = p_project
          AND s.type = 'memory'
          AND s.status != 'closed'
          AND (
              (p_root_id IS NOT NULL AND s.id = p_root_id)
              OR
              (p_root_id IS NULL AND s.parent_id IS NULL)
          )

        UNION ALL

        SELECT
            s.id, s.title, s.parent_id, t.depth + 1,
            s.status, s.labels, s.metadata, s.created_at
        FROM shards s
        JOIN tree t ON s.parent_id = t.id
        WHERE s.project = p_project
          AND s.type = 'memory'
          AND s.status != 'closed'
          AND t.depth < 20
    )
    SELECT
        t.id, t.title, t.parent_id, t.depth,
        t.status, t.labels,
        COALESCE((t.metadata->>'access_count')::int, 0),
        (t.metadata->>'last_accessed')::timestamptz,
        (SELECT count(*) FROM shards c
         WHERE c.parent_id = t.id AND c.type = 'memory' AND c.status != 'closed')::int,
        e.metadata->>'summary'
    FROM tree t
    LEFT JOIN edges e ON e.from_id = t.id
                     AND e.to_id = t.parent_id
                     AND e.edge_type = 'child-of'
    ORDER BY t.depth, t.created_at;
$$ LANGUAGE sql STABLE;

-- Get direct children of a memory
CREATE OR REPLACE FUNCTION memory_children(
    p_project TEXT,
    p_parent_id TEXT
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    status TEXT,
    labels TEXT[],
    access_count INT,
    last_accessed TIMESTAMPTZ,
    child_count INT,
    content TEXT
) AS $$
    SELECT
        s.id, s.title, s.status, s.labels,
        COALESCE((s.metadata->>'access_count')::int, 0),
        (s.metadata->>'last_accessed')::timestamptz,
        (SELECT count(*) FROM shards c
         WHERE c.parent_id = s.id AND c.type = 'memory' AND c.status != 'closed')::int,
        s.content
    FROM shards s
    WHERE s.project = p_project
      AND s.parent_id = p_parent_id
      AND s.type = 'memory'
      AND s.status != 'closed'
    ORDER BY s.created_at;
$$ LANGUAGE sql STABLE;

-- Get path from root to a specific memory
CREATE OR REPLACE FUNCTION memory_path(p_memory_id TEXT)
RETURNS TABLE (
    id TEXT,
    title TEXT,
    depth INT
) AS $$
    WITH RECURSIVE path AS (
        SELECT s.id, s.title, s.parent_id, 0 AS depth
        FROM shards s WHERE s.id = p_memory_id

        UNION ALL

        SELECT s.id, s.title, s.parent_id, p.depth + 1
        FROM shards s
        JOIN path p ON s.id = p.parent_id
        WHERE p.depth < 20
    )
    SELECT id, title, (SELECT max(depth) FROM path) - depth AS real_depth
    FROM path
    ORDER BY real_depth;
$$ LANGUAGE sql STABLE;

-- Promotion candidates: children accessed more than parent
CREATE OR REPLACE FUNCTION memory_hot(
    p_project TEXT,
    p_min_depth INT DEFAULT 1,
    p_limit INT DEFAULT 20
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    depth INT,
    access_count INT,
    parent_id TEXT,
    parent_title TEXT,
    parent_access_count INT
) AS $$
    WITH RECURSIVE tree AS (
        SELECT s.id, s.title, s.parent_id, 0 AS depth, s.metadata
        FROM shards s
        WHERE s.project = p_project
          AND s.type = 'memory'
          AND s.status != 'closed'
          AND s.parent_id IS NULL

        UNION ALL

        SELECT s.id, s.title, s.parent_id, t.depth + 1, s.metadata
        FROM shards s
        JOIN tree t ON s.parent_id = t.id
        WHERE s.project = p_project
          AND s.type = 'memory' AND s.status != 'closed'
          AND t.depth < 20
    )
    SELECT
        c.id, c.title, c.depth,
        COALESCE((c.metadata->>'access_count')::int, 0) AS access_count,
        p.id, p.title,
        COALESCE((p.metadata->>'access_count')::int, 0) AS parent_access_count
    FROM tree c
    JOIN shards p ON p.id = c.parent_id
    WHERE c.depth >= p_min_depth
      AND COALESCE((c.metadata->>'access_count')::int, 0) >
          COALESCE((p.metadata->>'access_count')::int, 0)
    ORDER BY COALESCE((c.metadata->>'access_count')::int, 0) DESC
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;

-- Increment access telemetry
CREATE OR REPLACE FUNCTION memory_touch(
    p_memory_id TEXT,
    p_agent TEXT,
    p_depth INT DEFAULT 0
) RETURNS VOID AS $$
DECLARE
    new_entry JSONB;
    old_log JSONB;
    new_log JSONB;
BEGIN
    new_entry := jsonb_build_object(
        'at', now()::text,
        'by', p_agent,
        'depth', p_depth
    );

    SELECT COALESCE(metadata->'access_log', '[]'::jsonb)
    INTO old_log
    FROM shards WHERE id = p_memory_id;

    SELECT COALESCE(to_jsonb(array_agg(elem ORDER BY ord)), '[]'::jsonb)
    INTO new_log
    FROM (
        SELECT new_entry AS elem, 0 AS ord
        UNION ALL
        SELECT value, ordinality::int
        FROM jsonb_array_elements(old_log) WITH ORDINALITY
        WHERE ordinality <= 49
    ) sub;

    UPDATE shards
    SET metadata = jsonb_set(
        jsonb_set(
            jsonb_set(
                COALESCE(metadata, '{}'::jsonb),
                '{access_count}',
                to_jsonb(COALESCE((metadata->>'access_count')::int, 0) + 1)
            ),
            '{last_accessed}',
            to_jsonb(now()::text)
        ),
        '{access_log}',
        new_log
    )
    WHERE id = p_memory_id;
END;
$$ LANGUAGE plpgsql VOLATILE;
