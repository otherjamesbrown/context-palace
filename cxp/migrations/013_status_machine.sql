-- Status machine: custom statuses + instance identity on claims
-- Statuses: open → ready → in_progress → needs-review → closed
-- Also supports 'blocked' label (separate from status).
-- Instance identity: each Claude Code session has a unique instance ID
-- recorded on shard_assign to prevent double-claiming.

-- Update shard_assign: add instance parameter, allow 'ready' status,
-- reject if claimed by different instance
CREATE OR REPLACE FUNCTION shard_assign(
    p_project TEXT,
    p_shard_id TEXT,
    p_agent TEXT,
    p_instance TEXT DEFAULT NULL
) RETURNS TEXT AS $$
DECLARE
    v_status TEXT;
    v_owner TEXT;
    v_title TEXT;
    v_instance TEXT;
BEGIN
    SELECT status, owner, title, metadata->>'instance'
    INTO v_status, v_owner, v_title, v_instance
    FROM shards WHERE id = p_shard_id AND project = p_project FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    IF v_status = 'closed' THEN
        RAISE EXCEPTION 'Shard % is already closed', p_shard_id;
    END IF;

    IF v_status = 'needs-review' THEN
        RAISE EXCEPTION 'Shard % is in needs-review status', p_shard_id;
    END IF;

    -- If already in_progress, only allow re-claim by same instance
    IF v_status = 'in_progress' THEN
        IF p_instance IS NOT NULL AND v_instance IS NOT NULL AND p_instance != v_instance THEN
            RAISE EXCEPTION 'Shard % is already claimed by % (instance: %)', p_shard_id, v_owner, v_instance;
        END IF;
        IF p_instance IS NULL OR v_instance IS NULL THEN
            RAISE EXCEPTION 'Shard % is already in_progress (owner: %)', p_shard_id, v_owner;
        END IF;
        -- Same instance re-claiming: allow (idempotent)
        RETURN v_title;
    END IF;

    -- Check blockers (for open/ready statuses)
    IF EXISTS (
        SELECT 1 FROM edges e
        JOIN shards blocker ON blocker.id = e.to_id
        WHERE e.from_id = p_shard_id
          AND e.edge_type = 'blocked-by'
          AND blocker.status != 'closed'
    ) THEN
        RAISE EXCEPTION 'Shard % has unresolved blockers', p_shard_id;
    END IF;

    UPDATE shards
    SET status = 'in_progress',
        owner = p_agent,
        updated_at = NOW(),
        metadata = jsonb_set(
            jsonb_set(
                COALESCE(metadata, '{}'::jsonb),
                '{assigned_at}',
                to_jsonb(NOW()::text)
            ),
            '{instance}',
            COALESCE(to_jsonb(p_instance), 'null'::jsonb)
        )
    WHERE id = p_shard_id AND project = p_project;

    RETURN v_title;
END;
$$ LANGUAGE plpgsql VOLATILE;


-- Update shard_next: include 'ready' shards (spec written, available for work)
CREATE OR REPLACE FUNCTION shard_next(
    p_project TEXT,
    p_epic_id TEXT DEFAULT NULL,
    p_limit INT DEFAULT 5
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    kind TEXT,
    priority INT,
    epic_id TEXT,
    epic_title TEXT
) AS $$
    SELECT
        s.id, s.title,
        COALESCE(
            (SELECT replace(l.label, 'kind:', '')
             FROM labels l
             WHERE l.shard_id = s.id AND l.label LIKE 'kind:%'
             LIMIT 1),
            'task'
        ),
        s.priority,
        s.parent_id,
        p.title
    FROM shards s
    LEFT JOIN shards p ON p.id = s.parent_id AND p.type = 'epic'
    WHERE s.project = p_project
      AND s.status IN ('open', 'ready')
      AND s.type NOT IN ('epic', 'memory', 'message')
      AND (p_epic_id IS NULL OR s.parent_id = p_epic_id)
      AND NOT EXISTS (
          SELECT 1 FROM edges e
          JOIN shards blocker ON blocker.id = e.to_id
          WHERE e.from_id = s.id
            AND e.edge_type = 'blocked-by'
            AND blocker.status != 'closed'
      )
    ORDER BY
        CASE s.status WHEN 'ready' THEN 0 ELSE 1 END,
        s.priority,
        s.created_at
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;


-- Update shard_board: include new statuses in ordering
CREATE OR REPLACE FUNCTION shard_board(
    p_project TEXT,
    p_epic_id TEXT DEFAULT NULL,
    p_agent TEXT DEFAULT NULL
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    status TEXT,
    kind TEXT,
    owner TEXT,
    priority INT,
    epic_id TEXT,
    epic_title TEXT,
    assigned_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    blocked_by TEXT[]
) AS $$
    SELECT
        s.id, s.title, s.status,
        COALESCE(
            (SELECT replace(l.label, 'kind:', '')
             FROM labels l
             WHERE l.shard_id = s.id AND l.label LIKE 'kind:%'
             LIMIT 1),
            'task'
        ),
        s.owner, s.priority, s.parent_id, p.title,
        (s.metadata->>'assigned_at')::timestamptz,
        s.closed_at,
        COALESCE(
            (SELECT array_agg(e.to_id)
             FROM edges e
             JOIN shards blocker ON blocker.id = e.to_id
             WHERE e.from_id = s.id
               AND e.edge_type = 'blocked-by'
               AND blocker.status != 'closed'),
            '{}'::text[]
        )
    FROM shards s
    LEFT JOIN shards p ON p.id = s.parent_id AND p.type = 'epic'
    WHERE s.project = p_project
      AND s.type NOT IN ('epic', 'memory', 'message')
      AND (p_epic_id IS NULL OR s.parent_id = p_epic_id)
      AND (p_agent IS NULL OR s.owner = p_agent)
      AND (p_agent IS NOT NULL
           OR p_epic_id IS NOT NULL
           OR s.status != 'closed'
           OR s.closed_at > NOW() - INTERVAL '24 hours')
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


-- Update epic_progress: count ready and needs-review statuses
CREATE OR REPLACE FUNCTION epic_progress(
    p_project TEXT,
    p_epic_id TEXT
) RETURNS TABLE (
    total INT,
    completed INT,
    in_progress INT,
    open INT,
    blocked INT
) AS $$
    SELECT
        count(*)::int AS total,
        count(*) FILTER (WHERE s.status = 'closed')::int AS completed,
        count(*) FILTER (WHERE s.status IN ('in_progress', 'needs-review'))::int AS in_progress,
        count(*) FILTER (WHERE s.status IN ('open', 'ready')
            AND NOT EXISTS (
                SELECT 1 FROM edges e
                JOIN shards blocker ON blocker.id = e.to_id
                WHERE e.from_id = s.id
                  AND e.edge_type = 'blocked-by'
                  AND blocker.status != 'closed'
            ))::int AS open,
        count(*) FILTER (WHERE s.status IN ('open', 'ready')
            AND EXISTS (
                SELECT 1 FROM edges e
                JOIN shards blocker ON blocker.id = e.to_id
                WHERE e.from_id = s.id
                  AND e.edge_type = 'blocked-by'
                  AND blocker.status != 'closed'
            ))::int AS blocked
    FROM shards s
    WHERE s.project = p_project
      AND s.parent_id = p_epic_id
      AND s.type != 'epic';
$$ LANGUAGE sql STABLE;


-- Update epic_children: include new statuses in ordering
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
    blocked_by TEXT[]
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
        )
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
