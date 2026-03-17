-- Add description to shard_edges() return set
-- Shows linked shard description alongside title in edge listings

DROP FUNCTION IF EXISTS shard_edges(TEXT, TEXT, TEXT[]);

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
    edge_metadata JSONB,
    linked_shard_description TEXT
) AS $$
    -- Outgoing edges
    SELECT
        'outgoing'::text,
        e.edge_type,
        e.to_id,
        s.title,
        s.type,
        s.status,
        e.metadata,
        s.description
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
        e.metadata,
        s.description
    FROM edges e
    JOIN shards s ON s.id = e.from_id
    WHERE e.to_id = p_shard_id
      AND (p_direction IS NULL OR p_direction = 'incoming')
      AND (p_edge_types IS NULL OR e.edge_type = ANY(p_edge_types))

    ORDER BY 2, 1;
$$ LANGUAGE sql STABLE;
