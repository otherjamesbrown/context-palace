-- SPEC-4: Knowledge Documents SQL Functions
-- Depends on: SPEC-0 (shards table), SPEC-2 (metadata column)

-- Update knowledge document with versioning
-- Locks the row to prevent concurrent version collisions
CREATE OR REPLACE FUNCTION update_knowledge_doc(
    p_shard_id TEXT,
    p_new_content TEXT,
    p_change_summary TEXT,
    p_changed_by TEXT,
    p_project TEXT
) RETURNS TABLE (shard_id TEXT, version INT) AS $$
DECLARE
    current_version INT;
    current_content TEXT;
    current_status TEXT;
    version_shard_id TEXT;
BEGIN
    -- Lock the shard row for the duration of this transaction
    SELECT
        COALESCE((s.metadata->>'version')::int, 1),
        s.content,
        s.status
    INTO current_version, current_content, current_status
    FROM shards s
    WHERE s.id = p_shard_id AND s.type = 'knowledge' AND s.project = p_project
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Knowledge document % not found in project %', p_shard_id, p_project;
    END IF;

    -- Reject updates to closed documents
    IF current_status = 'closed' THEN
        RAISE EXCEPTION 'Knowledge document % is closed. Reopen with cp shard reopen before updating.', p_shard_id;
    END IF;

    -- Check content actually changed
    IF current_content = p_new_content THEN
        RAISE EXCEPTION 'Content is identical to current version';
    END IF;

    -- Create version snapshot (copy current content to new shard)
    version_shard_id := p_shard_id || '-v' || current_version;

    -- Guard against snapshot ID collision (shouldn't happen with FOR UPDATE, but safe)
    IF EXISTS (SELECT 1 FROM shards WHERE id = version_shard_id) THEN
        RAISE EXCEPTION 'Version snapshot % already exists', version_shard_id;
    END IF;

    INSERT INTO shards (id, project, title, content, type, status, creator, metadata, labels)
    SELECT
        version_shard_id, s.project,
        s.title || ' (v' || current_version || ')',
        s.content, 'knowledge', 'closed', s.creator,
        jsonb_set(s.metadata, '{version}', to_jsonb(current_version)),
        s.labels
    FROM shards s WHERE s.id = p_shard_id;

    -- Create previous-version edge
    INSERT INTO edges (from_id, to_id, edge_type, metadata)
    VALUES (p_shard_id, version_shard_id, 'previous-version',
            jsonb_build_object(
                'change_summary', p_change_summary,
                'changed_by', p_changed_by,
                'changed_at', now()::text
            ));

    -- Update current shard (set new version, summary, changed_by)
    UPDATE shards
    SET content = p_new_content,
        metadata = jsonb_set(
            jsonb_set(
                jsonb_set(
                    jsonb_set(metadata, '{version}', to_jsonb(current_version + 1)),
                    '{previous_version_id}', to_jsonb(version_shard_id)
                ),
                '{last_changed_by}', to_jsonb(p_changed_by)
            ),
            '{last_change_summary}', to_jsonb(p_change_summary)
        ),
        updated_at = now()
    WHERE id = p_shard_id;

    RETURN QUERY SELECT p_shard_id, current_version + 1;
END;
$$ LANGUAGE plpgsql;


-- Append to knowledge document (concatenate content, then version)
CREATE OR REPLACE FUNCTION append_knowledge_doc(
    p_shard_id TEXT,
    p_append_content TEXT,
    p_change_summary TEXT,
    p_changed_by TEXT,
    p_project TEXT
) RETURNS TABLE (shard_id TEXT, version INT) AS $$
DECLARE
    current_content TEXT;
    current_status TEXT;
    new_content TEXT;
BEGIN
    -- Read current content (with lock)
    SELECT s.content, s.status INTO current_content, current_status
    FROM shards s
    WHERE s.id = p_shard_id AND s.type = 'knowledge' AND s.project = p_project
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Knowledge document % not found in project %', p_shard_id, p_project;
    END IF;

    -- Reject appends to closed documents
    IF current_status = 'closed' THEN
        RAISE EXCEPTION 'Knowledge document % is closed. Reopen with cp shard reopen before appending.', p_shard_id;
    END IF;

    -- Concatenate
    new_content := COALESCE(current_content, '') || E'\n\n' || p_append_content;

    -- Delegate to update (same transaction, FOR UPDATE lock already held)
    RETURN QUERY SELECT * FROM update_knowledge_doc(
        p_shard_id, new_content, p_change_summary, p_changed_by, p_project
    );
END;
$$ LANGUAGE plpgsql;


-- Get version history for a knowledge document
-- Returns all versions reverse-chronologically with depth limit for safety
CREATE OR REPLACE FUNCTION knowledge_history(
    p_shard_id TEXT,
    p_project TEXT
) RETURNS TABLE (
    version INT,
    changed_at TIMESTAMPTZ,
    changed_by TEXT,
    change_summary TEXT,
    shard_id TEXT
) AS $$
    WITH RECURSIVE versions AS (
        -- Current version (reads change summary from shard metadata, not hardcoded)
        SELECT
            s.id,
            COALESCE((s.metadata->>'version')::int, 1) as version,
            s.updated_at as changed_at,
            COALESCE(s.metadata->>'last_changed_by', s.creator) as changed_by,
            COALESCE(s.metadata->>'last_change_summary', 'Initial document') as change_summary,
            1 as depth
        FROM shards s
        WHERE s.id = p_shard_id AND s.project = p_project

        UNION ALL

        -- Previous versions via edges (reads change summary from snapshot shard metadata)
        SELECT
            e.to_id,
            COALESCE((t.metadata->>'version')::int, 1),
            COALESCE((e.metadata->>'changed_at')::timestamptz, t.created_at),
            COALESCE(t.metadata->>'last_changed_by', t.creator),
            COALESCE(t.metadata->>'last_change_summary', 'Initial document'),
            v.depth + 1
        FROM versions v
        JOIN edges e ON e.from_id = v.id AND e.edge_type = 'previous-version'
        JOIN shards t ON t.id = e.to_id
        WHERE v.depth < 1000  -- Safety cap to prevent infinite recursion
    )
    SELECT v.version, v.changed_at, v.changed_by, v.change_summary, v.id
    FROM versions v
    ORDER BY v.version DESC;
$$ LANGUAGE sql STABLE;


-- Get content at a specific version number
CREATE OR REPLACE FUNCTION knowledge_version(
    p_shard_id TEXT,
    p_version INT,
    p_project TEXT
) RETURNS TABLE (
    shard_id TEXT,
    version INT,
    title TEXT,
    content TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ
) AS $$
DECLARE
    current_ver INT;
BEGIN
    -- Check current version first
    SELECT COALESCE((s.metadata->>'version')::int, 1) INTO current_ver
    FROM shards s
    WHERE s.id = p_shard_id AND s.project = p_project;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Knowledge document % not found', p_shard_id;
    END IF;

    IF p_version > current_ver OR p_version < 1 THEN
        RAISE EXCEPTION 'Version % not found. Document has % versions.', p_version, current_ver;
    END IF;

    -- If requesting current version, return the main shard
    IF p_version = current_ver THEN
        RETURN QUERY
        SELECT s.id, current_ver, s.title, s.content, s.metadata, s.created_at
        FROM shards s WHERE s.id = p_shard_id;
        RETURN;
    END IF;

    -- Otherwise, look up the snapshot shard
    RETURN QUERY
    SELECT s.id, p_version, s.title, s.content, s.metadata, s.created_at
    FROM shards s
    WHERE s.id = p_shard_id || '-v' || p_version
      AND s.project = p_project;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Version % snapshot not found (shard % may have been deleted)',
            p_version, p_shard_id || '-v' || p_version;
    END IF;
END;
$$ LANGUAGE plpgsql STABLE;
