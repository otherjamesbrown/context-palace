-- SPEC-7: Shard Lifecycle, Epics, and Focus
-- Depends on: SPEC-0, SPEC-2, SPEC-5

-- Focus table: persistent active epic per project+agent
CREATE TABLE IF NOT EXISTS focus (
    project     TEXT NOT NULL,
    agent       TEXT NOT NULL,
    epic_id     TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
    set_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note        TEXT,
    PRIMARY KEY (project, agent)
);

CREATE INDEX IF NOT EXISTS idx_focus_epic ON focus(epic_id);

-- Add closed_by column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'shards' AND column_name = 'closed_by'
    ) THEN
        ALTER TABLE shards ADD COLUMN closed_by TEXT;
    END IF;
END $$;

-- Epic progress: completion stats for an epic's children
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
        count(*) FILTER (WHERE s.status = 'in_progress')::int AS in_progress,
        count(*) FILTER (WHERE s.status = 'open'
            AND NOT EXISTS (
                SELECT 1 FROM edges e
                JOIN shards blocker ON blocker.id = e.to_id
                WHERE e.from_id = s.id
                  AND e.edge_type = 'blocked-by'
                  AND blocker.status != 'closed'
            ))::int AS open,
        count(*) FILTER (WHERE s.status = 'open'
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


-- Epic children: detailed child list with status, kind, owner, blockers
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
            WHEN 'open' THEN 1
            WHEN 'closed' THEN 2
        END,
        s.priority,
        s.created_at;
$$ LANGUAGE sql STABLE;


-- Next workable shard: unblocked, open, ordered by priority
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
      AND s.status = 'open'
      AND s.type NOT IN ('epic', 'memory', 'message')
      AND (p_epic_id IS NULL OR s.parent_id = p_epic_id)
      AND NOT EXISTS (
          SELECT 1 FROM edges e
          JOIN shards blocker ON blocker.id = e.to_id
          WHERE e.from_id = s.id
            AND e.edge_type = 'blocked-by'
            AND blocker.status != 'closed'
      )
    ORDER BY s.priority, s.created_at
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;


-- Board view: all shards grouped by status
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
            WHEN 'open' THEN 1
            WHEN 'closed' THEN 2
        END,
        s.priority,
        s.created_at;
$$ LANGUAGE sql STABLE;


-- Focus management: set
CREATE OR REPLACE FUNCTION focus_set(
    p_project TEXT,
    p_agent TEXT,
    p_epic_id TEXT,
    p_note TEXT DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM shards
        WHERE id = p_epic_id AND project = p_project AND type = 'epic'
    ) THEN
        RAISE EXCEPTION 'Shard % is not an epic in project %', p_epic_id, p_project;
    END IF;

    INSERT INTO focus (project, agent, epic_id, set_at, note)
    VALUES (p_project, p_agent, p_epic_id, NOW(), p_note)
    ON CONFLICT (project, agent) DO UPDATE
    SET epic_id = p_epic_id, set_at = NOW(), note = p_note;
END;
$$ LANGUAGE plpgsql VOLATILE;


-- Focus management: get (auto-clears if epic is closed)
CREATE OR REPLACE FUNCTION focus_get(
    p_project TEXT,
    p_agent TEXT
) RETURNS TABLE (
    epic_id TEXT,
    epic_title TEXT,
    epic_status TEXT,
    set_at TIMESTAMPTZ,
    note TEXT
) AS $$
DECLARE
    v_epic_id TEXT;
    v_epic_title TEXT;
    v_epic_status TEXT;
    v_set_at TIMESTAMPTZ;
    v_note TEXT;
BEGIN
    SELECT f.epic_id, s.title, s.status, f.set_at, f.note
    INTO v_epic_id, v_epic_title, v_epic_status, v_set_at, v_note
    FROM focus f
    JOIN shards s ON s.id = f.epic_id
    WHERE f.project = p_project AND f.agent = p_agent;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    IF v_epic_status = 'closed' THEN
        DELETE FROM focus WHERE project = p_project AND agent = p_agent;
        RETURN;
    END IF;

    RETURN QUERY SELECT v_epic_id, v_epic_title, v_epic_status, v_set_at, v_note;
END;
$$ LANGUAGE plpgsql VOLATILE;


-- Focus management: clear
CREATE OR REPLACE FUNCTION focus_clear(
    p_project TEXT,
    p_agent TEXT
) RETURNS BOOLEAN AS $$
DECLARE
    rows_deleted INT;
BEGIN
    DELETE FROM focus WHERE project = p_project AND agent = p_agent;
    GET DIAGNOSTICS rows_deleted = ROW_COUNT;
    RETURN rows_deleted > 0;
END;
$$ LANGUAGE plpgsql VOLATILE;


-- Shard assign: atomically claim a shard
CREATE OR REPLACE FUNCTION shard_assign(
    p_project TEXT,
    p_shard_id TEXT,
    p_agent TEXT
) RETURNS TEXT AS $$
DECLARE
    v_status TEXT;
    v_owner TEXT;
    v_title TEXT;
BEGIN
    SELECT status, owner, title INTO v_status, v_owner, v_title
    FROM shards WHERE id = p_shard_id AND project = p_project FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    IF v_status = 'in_progress' THEN
        RAISE EXCEPTION 'Shard % is already in_progress (owner: %)', p_shard_id, v_owner;
    END IF;

    IF v_status = 'closed' THEN
        RAISE EXCEPTION 'Shard % is already closed', p_shard_id;
    END IF;

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
            COALESCE(metadata, '{}'::jsonb),
            '{assigned_at}',
            to_jsonb(NOW()::text)
        )
    WHERE id = p_shard_id AND project = p_project;

    RETURN v_title;
END;
$$ LANGUAGE plpgsql VOLATILE;


-- Shard close: close with reason, report unblocked shards
CREATE OR REPLACE FUNCTION shard_close(
    p_project TEXT,
    p_shard_id TEXT,
    p_agent TEXT,
    p_reason TEXT DEFAULT NULL
) RETURNS TABLE (
    closed_title TEXT,
    unblocked_id TEXT,
    unblocked_title TEXT
) AS $$
DECLARE
    v_status TEXT;
    v_title TEXT;
    v_parent_id TEXT;
BEGIN
    SELECT status, title, parent_id INTO v_status, v_title, v_parent_id
    FROM shards WHERE id = p_shard_id AND project = p_project FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    -- Idempotent: already closed = no-op
    IF v_status = 'closed' THEN
        RETURN QUERY SELECT v_title, NULL::TEXT, NULL::TEXT;
        RETURN;
    END IF;

    UPDATE shards
    SET status = 'closed',
        closed_at = NOW(),
        closed_by = p_agent,
        closed_reason = p_reason,
        updated_at = NOW()
    WHERE id = p_shard_id;

    -- Update parent epic's updated_at
    IF v_parent_id IS NOT NULL THEN
        UPDATE shards SET updated_at = NOW() WHERE id = v_parent_id;
    END IF;

    -- Return the closed shard + any newly unblocked shards
    RETURN QUERY
    SELECT v_title, NULL::TEXT, NULL::TEXT
    UNION ALL
    SELECT NULL::TEXT, s.id, s.title
    FROM shards s
    WHERE s.status = 'open'
      AND EXISTS (
          SELECT 1 FROM edges e
          WHERE e.from_id = s.id
            AND e.to_id = p_shard_id
            AND e.edge_type = 'blocked-by'
      )
      AND NOT EXISTS (
          SELECT 1 FROM edges e
          JOIN shards blocker ON blocker.id = e.to_id
          WHERE e.from_id = s.id
            AND e.edge_type = 'blocked-by'
            AND blocker.status != 'closed'
      );
END;
$$ LANGUAGE plpgsql VOLATILE;
