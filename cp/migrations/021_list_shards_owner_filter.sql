-- Add owner (assigned-to) filter to list_shards and list_shards_count

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
    p_parent_id_null BOOLEAN DEFAULT FALSE,
    p_owner TEXT DEFAULT NULL
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
      AND (p_owner IS NULL OR s.owner = p_owner)
    ORDER BY s.created_at DESC
    LIMIT p_limit OFFSET p_offset;
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION list_shards_count(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since TIMESTAMPTZ DEFAULT NULL,
    p_parent_id_null BOOLEAN DEFAULT FALSE,
    p_owner TEXT DEFAULT NULL
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
      AND (NOT p_parent_id_null OR s.parent_id IS NULL)
      AND (p_owner IS NULL OR s.owner = p_owner);
$$ LANGUAGE sql STABLE;
