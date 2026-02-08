-- SPEC-5: Unified Search & Graph CLI
-- Depends on: 004_pgvector.sql (SPEC-1), 002_metadata.sql (SPEC-2), 003_requirements.sql (SPEC-3)

BEGIN;

-- Deduplicate edges before adding unique constraint
DELETE FROM edges WHERE ctid NOT IN (
    SELECT min(ctid) FROM edges GROUP BY from_id, to_id, edge_type
);

-- Unique constraint on edges to prevent duplicates
ALTER TABLE edges ADD CONSTRAINT edges_unique_triple
    UNIQUE (from_id, to_id, edge_type);

-- General shard listing with all filters
CREATE OR REPLACE FUNCTION list_shards(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since TIMESTAMPTZ DEFAULT NULL,
    p_limit INT DEFAULT 20,
    p_offset INT DEFAULT 0
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    type TEXT,
    status TEXT,
    creator TEXT,
    labels TEXT[],
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    snippet TEXT
) AS $$
    SELECT
        s.id, s.title, s.type, s.status, s.creator,
        s.labels, s.created_at, s.updated_at,
        LEFT(s.content, 200) AS snippet
    FROM shards s
    WHERE s.project = p_project
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_labels IS NULL OR s.labels && p_labels)
      AND (p_creator IS NULL OR s.creator = p_creator)
      AND (p_search IS NULL OR s.search_vector @@ plainto_tsquery(p_search))
      AND (p_since IS NULL OR s.created_at >= p_since)
    ORDER BY s.created_at DESC
    LIMIT p_limit
    OFFSET p_offset;
$$ LANGUAGE sql STABLE;

-- Count total matching shards (for pagination display)
CREATE OR REPLACE FUNCTION list_shards_count(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since TIMESTAMPTZ DEFAULT NULL
) RETURNS INT AS $$
    SELECT count(*)::int
    FROM shards s
    WHERE s.project = p_project
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_labels IS NULL OR s.labels && p_labels)
      AND (p_creator IS NULL OR s.creator = p_creator)
      AND (p_search IS NULL OR s.search_vector @@ plainto_tsquery(p_search))
      AND (p_since IS NULL OR s.created_at >= p_since);
$$ LANGUAGE sql STABLE;

-- Get shard with all edges
CREATE OR REPLACE FUNCTION shard_detail(p_shard_id TEXT)
RETURNS TABLE (
    id TEXT,
    title TEXT,
    content TEXT,
    type TEXT,
    status TEXT,
    creator TEXT,
    labels TEXT[],
    metadata JSONB,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    outgoing_edge_count INT,
    incoming_edge_count INT
) AS $$
    SELECT
        s.id, s.title, s.content, s.type, s.status, s.creator,
        s.labels, s.metadata, s.created_at, s.updated_at,
        (SELECT count(*) FROM edges e WHERE e.from_id = s.id)::int,
        (SELECT count(*) FROM edges e WHERE e.to_id = s.id)::int
    FROM shards s
    WHERE s.id = p_shard_id;
$$ LANGUAGE sql STABLE;

-- Get edges for a shard (both directions)
CREATE OR REPLACE FUNCTION shard_edges(
    p_shard_id TEXT,
    p_direction TEXT DEFAULT NULL,
    p_edge_types TEXT[] DEFAULT NULL
) RETURNS TABLE (
    direction TEXT,
    edge_type TEXT,
    linked_shard_id TEXT,
    linked_shard_title TEXT,
    linked_shard_type TEXT,
    linked_shard_status TEXT,
    edge_metadata JSONB
) AS $$
    -- Outgoing edges
    SELECT
        'outgoing'::text,
        e.edge_type,
        e.to_id,
        s.title,
        s.type,
        s.status,
        e.metadata
    FROM edges e
    JOIN shards s ON s.id = e.to_id
    WHERE e.from_id = p_shard_id
      AND (p_direction IS NULL OR p_direction = 'outgoing')
      AND (p_edge_types IS NULL OR e.edge_type = ANY(p_edge_types))

    UNION ALL

    -- Incoming edges
    SELECT
        'incoming'::text,
        e.edge_type,
        e.from_id,
        s.title,
        s.type,
        s.status,
        e.metadata
    FROM edges e
    JOIN shards s ON s.id = e.from_id
    WHERE e.to_id = p_shard_id
      AND (p_direction IS NULL OR p_direction = 'incoming')
      AND (p_edge_types IS NULL OR e.edge_type = ANY(p_edge_types))

    ORDER BY 2, 1;
$$ LANGUAGE sql STABLE;

-- Add labels to a shard atomically (deduplicates)
CREATE OR REPLACE FUNCTION add_shard_labels(
    p_shard_id TEXT,
    p_labels TEXT[]
) RETURNS TEXT[] AS $$
DECLARE
    result_labels TEXT[];
BEGIN
    UPDATE shards
    SET labels = (
        SELECT ARRAY(
            SELECT DISTINCT unnest(COALESCE(labels, '{}') || p_labels)
            ORDER BY 1
        )
    ),
    updated_at = now()
    WHERE id = p_shard_id
    RETURNING labels INTO result_labels;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result_labels;
END;
$$ LANGUAGE plpgsql;

-- Remove labels from a shard atomically
CREATE OR REPLACE FUNCTION remove_shard_labels(
    p_shard_id TEXT,
    p_labels TEXT[]
) RETURNS TEXT[] AS $$
DECLARE
    result_labels TEXT[];
BEGIN
    UPDATE shards
    SET labels = (
        SELECT ARRAY(
            SELECT unnest(COALESCE(labels, '{}'))
            EXCEPT
            SELECT unnest(p_labels)
            ORDER BY 1
        )
    ),
    updated_at = now()
    WHERE id = p_shard_id
    RETURNING labels INTO result_labels;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result_labels;
END;
$$ LANGUAGE plpgsql;

-- Create edge with duplicate prevention
CREATE OR REPLACE FUNCTION create_edge(
    p_from_id TEXT,
    p_to_id TEXT,
    p_edge_type TEXT,
    p_metadata JSONB DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
    rows_affected INT;
BEGIN
    -- Verify both shards exist
    IF NOT EXISTS (SELECT 1 FROM shards WHERE id = p_from_id) THEN
        RAISE EXCEPTION 'Shard % not found', p_from_id;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM shards WHERE id = p_to_id) THEN
        RAISE EXCEPTION 'Shard % not found', p_to_id;
    END IF;

    -- Prevent self-reference
    IF p_from_id = p_to_id THEN
        RAISE EXCEPTION 'Cannot create edge from a shard to itself';
    END IF;

    -- Insert with duplicate prevention
    INSERT INTO edges (from_id, to_id, edge_type, metadata)
    VALUES (p_from_id, p_to_id, p_edge_type, COALESCE(p_metadata, '{}'))
    ON CONFLICT (from_id, to_id, edge_type) DO NOTHING;

    GET DIAGNOSTICS rows_affected = ROW_COUNT;

    IF rows_affected = 0 THEN
        RAISE EXCEPTION 'Edge already exists: % --%--> %', p_from_id, p_edge_type, p_to_id;
    END IF;

    RETURN true;
END;
$$ LANGUAGE plpgsql;

-- Delete edge
CREATE OR REPLACE FUNCTION delete_edge(
    p_from_id TEXT,
    p_to_id TEXT,
    p_edge_type TEXT
) RETURNS BOOLEAN AS $$
DECLARE
    rows_affected INT;
BEGIN
    DELETE FROM edges
    WHERE from_id = p_from_id AND to_id = p_to_id AND edge_type = p_edge_type;

    GET DIAGNOSTICS rows_affected = ROW_COUNT;

    IF rows_affected = 0 THEN
        RAISE EXCEPTION 'No edge of type ''%'' from % to %', p_edge_type, p_from_id, p_to_id;
    END IF;

    RETURN true;
END;
$$ LANGUAGE plpgsql;

-- All labels in use with counts (excludes closed shards)
CREATE OR REPLACE FUNCTION label_summary(p_project TEXT)
RETURNS TABLE (
    label TEXT,
    shard_count INT
) AS $$
    SELECT
        unnest(s.labels) AS label,
        count(*)::int AS shard_count
    FROM shards s
    WHERE s.project = p_project
      AND s.status != 'closed'
      AND s.labels IS NOT NULL
      AND array_length(s.labels, 1) > 0
    GROUP BY 1
    ORDER BY 2 DESC, 1;
$$ LANGUAGE sql STABLE;

-- Update shard content and/or title
CREATE OR REPLACE FUNCTION update_shard(
    p_shard_id TEXT,
    p_project TEXT,
    p_title TEXT DEFAULT NULL,
    p_content TEXT DEFAULT NULL
) RETURNS TABLE (
    id TEXT,
    updated_at TIMESTAMPTZ,
    title_changed BOOLEAN,
    content_changed BOOLEAN,
    shard_type TEXT
) AS $$
DECLARE
    current_type TEXT;
BEGIN
    -- Verify shard exists and get type
    SELECT s.type INTO current_type
    FROM shards s WHERE s.id = p_shard_id AND s.project = p_project;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    -- Update title and/or content
    UPDATE shards s
    SET title = COALESCE(p_title, s.title),
        content = COALESCE(p_content, s.content),
        updated_at = now()
    WHERE s.id = p_shard_id AND s.project = p_project;

    RETURN QUERY SELECT
        p_shard_id,
        now(),
        (p_title IS NOT NULL),
        (p_content IS NOT NULL),
        current_type;
END;
$$ LANGUAGE plpgsql;

-- AMENDED: semantic_search() with p_since TIMESTAMPTZ parameter
CREATE OR REPLACE FUNCTION semantic_search(
    p_project TEXT,
    p_query_embedding vector(768),
    p_types TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_limit INT DEFAULT 20,
    p_min_similarity FLOAT DEFAULT 0.3,
    p_since TIMESTAMPTZ DEFAULT NULL
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    type TEXT,
    status TEXT,
    similarity FLOAT,
    snippet TEXT,
    labels TEXT[],
    created_at TIMESTAMPTZ
) AS $$
    SELECT
        s.id, s.title, s.type, s.status,
        1 - (s.embedding <=> p_query_embedding) AS similarity,
        LEFT(s.content, 200) AS snippet,
        s.labels,
        s.created_at
    FROM shards s
    WHERE s.project = p_project
      AND s.embedding IS NOT NULL
      AND 1 - (s.embedding <=> p_query_embedding) >= p_min_similarity
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_labels IS NULL OR s.labels && p_labels)
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_since IS NULL OR s.created_at >= p_since)
    ORDER BY s.embedding <=> p_query_embedding
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;

-- Semantic search limited to memory shards
CREATE OR REPLACE FUNCTION memory_recall(
    p_project TEXT,
    p_query_embedding vector(768),
    p_labels TEXT[] DEFAULT NULL,
    p_limit INT DEFAULT 10,
    p_min_similarity FLOAT DEFAULT 0.3
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    content TEXT,
    similarity FLOAT,
    labels TEXT[],
    created_at TIMESTAMPTZ
) AS $$
    SELECT
        s.id, s.title, s.content,
        1 - (s.embedding <=> p_query_embedding) AS similarity,
        s.labels, s.created_at
    FROM shards s
    WHERE s.project = p_project
      AND s.type = 'memory'
      AND s.status != 'closed'
      AND s.embedding IS NOT NULL
      AND 1 - (s.embedding <=> p_query_embedding) >= p_min_similarity
      AND (p_labels IS NULL OR s.labels && p_labels)
    ORDER BY s.embedding <=> p_query_embedding
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;

COMMIT;
