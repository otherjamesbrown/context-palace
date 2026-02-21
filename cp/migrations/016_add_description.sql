-- Add description field to shards
-- A short tagline (10-15 words) summarizing what the shard is about.
-- Separate from title (identity) and content (body).

ALTER TABLE shards ADD COLUMN IF NOT EXISTS description TEXT;

-- Must drop functions whose return type changes
DROP FUNCTION IF EXISTS shard_detail(TEXT);
DROP FUNCTION IF EXISTS epic_children(TEXT, TEXT);

-- Update shard_detail to return description
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
    incoming_edge_count INT,
    description TEXT
) AS $$
    SELECT
        s.id, s.title, s.content, s.type, s.status, s.creator,
        s.labels, s.metadata, s.created_at, s.updated_at,
        (SELECT count(*) FROM edges e WHERE e.from_id = s.id)::int,
        (SELECT count(*) FROM edges e WHERE e.to_id = s.id)::int,
        s.description
    FROM shards s
    WHERE s.id = p_shard_id;
$$ LANGUAGE sql STABLE;

-- Drop old update_shard (4-param version) and replace with 5-param version
DROP FUNCTION IF EXISTS update_shard(TEXT, TEXT, TEXT, TEXT);

-- Update update_shard to accept description
CREATE OR REPLACE FUNCTION update_shard(
    p_shard_id TEXT,
    p_project TEXT,
    p_title TEXT DEFAULT NULL,
    p_content TEXT DEFAULT NULL,
    p_description TEXT DEFAULT NULL
) RETURNS TABLE (
    id TEXT,
    updated_at TIMESTAMPTZ,
    title_changed BOOLEAN,
    content_changed BOOLEAN,
    shard_type TEXT,
    description_changed BOOLEAN
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

    -- Update title, content, and/or description
    UPDATE shards s
    SET title = COALESCE(p_title, s.title),
        content = COALESCE(p_content, s.content),
        description = COALESCE(p_description, s.description),
        updated_at = now()
    WHERE s.id = p_shard_id AND s.project = p_project;

    RETURN QUERY SELECT
        p_shard_id,
        now(),
        (p_title IS NOT NULL),
        (p_content IS NOT NULL),
        current_type,
        (p_description IS NOT NULL);
END;
$$ LANGUAGE plpgsql;

-- Update epic_children to return description
CREATE OR REPLACE FUNCTION epic_children(
    p_project TEXT,
    p_epic_id TEXT
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    status TEXT,
    kind TEXT,
    owner TEXT,
    priority INT,
    assigned_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    closed_by TEXT,
    closed_reason TEXT,
    blocked_by TEXT[],
    description TEXT
) AS $$
    SELECT
        s.id, s.title, s.status,
        COALESCE(
            (SELECT replace(l.label, 'kind:', '')
             FROM labels l
             WHERE l.shard_id = s.id AND l.label LIKE 'kind:%'
             LIMIT 1),
            'task'
        ) AS kind,
        s.owner,
        s.priority,
        (s.metadata->>'assigned_at')::timestamptz,
        s.closed_at,
        s.closed_by,
        s.closed_reason,
        COALESCE(
            (SELECT array_agg(e.to_id)
             FROM edges e
             JOIN shards blocker ON blocker.id = e.to_id
             WHERE e.from_id = s.id
               AND e.edge_type = 'blocked-by'
               AND blocker.status != 'closed'),
            '{}'::text[]
        ),
        s.description
    FROM shards s
    WHERE s.project = p_project
      AND s.parent_id = p_epic_id
      AND s.type != 'epic'
    ORDER BY
        CASE s.status
            WHEN 'in_progress' THEN 0
            WHEN 'needs-review' THEN 1
            WHEN 'ready' THEN 2
            WHEN 'open' THEN 3
            WHEN 'closed' THEN 4
        END,
        s.priority,
        s.created_at;
$$ LANGUAGE sql STABLE;
