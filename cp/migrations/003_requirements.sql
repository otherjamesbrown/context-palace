-- SPEC-3: Requirement Management
-- SQL functions for requirement dashboard, circular dependency detection,
-- and auto-status trigger

-- Requirement dashboard query
CREATE OR REPLACE FUNCTION requirement_dashboard(p_project TEXT)
RETURNS TABLE (
    id TEXT,
    title TEXT,
    lifecycle_status TEXT,
    priority INT,
    category TEXT,
    task_count_total INT,
    task_count_closed INT,
    test_count INT,
    blocked_by_ids TEXT[],
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
) AS $$
    SELECT
        s.id, s.title,
        COALESCE(s.metadata->>'lifecycle_status', 'draft'),
        COALESCE((s.metadata->>'priority')::int, 3),
        s.metadata->>'category',
        (SELECT count(*) FROM edges e
         WHERE e.to_id = s.id AND e.edge_type = 'implements')::int,
        (SELECT count(*) FROM edges e
         JOIN shards t ON t.id = e.from_id
         WHERE e.to_id = s.id AND e.edge_type = 'implements'
         AND t.status = 'closed')::int,
        (SELECT count(*) FROM edges e
         WHERE e.to_id = s.id AND e.edge_type = 'has-artifact'
         AND EXISTS (SELECT 1 FROM shards a WHERE a.id = e.from_id AND a.type = 'test'))::int,
        (SELECT array_agg(e.to_id) FROM edges e
         WHERE e.from_id = s.id AND e.edge_type = 'blocked-by'
         AND EXISTS (SELECT 1 FROM shards b WHERE b.id = e.to_id
                     AND COALESCE(b.metadata->>'lifecycle_status','draft') != 'verified')),
        s.created_at,
        s.updated_at
    FROM shards s
    WHERE s.project = p_project
      AND s.type = 'requirement'
      AND s.status != 'closed'
    ORDER BY COALESCE((s.metadata->>'priority')::int, 3), s.created_at;
$$ LANGUAGE sql STABLE;

-- Check for circular dependencies in blocked-by edges
CREATE OR REPLACE FUNCTION has_circular_dependency(
    p_from TEXT,
    p_to TEXT
) RETURNS BOOLEAN AS $$
    WITH RECURSIVE dep_chain AS (
        SELECT to_id FROM edges WHERE from_id = p_to AND edge_type = 'blocked-by'
        UNION
        SELECT e.to_id FROM edges e
        JOIN dep_chain d ON e.from_id = d.to_id
        WHERE e.edge_type = 'blocked-by'
    )
    SELECT EXISTS (SELECT 1 FROM dep_chain WHERE to_id = p_from)
           OR p_from = p_to;
$$ LANGUAGE sql STABLE;

-- Trigger: when a task shard is closed, check if all implementing tasks for
-- any linked requirement are now closed, and auto-set lifecycle_status to 'implemented'
CREATE OR REPLACE FUNCTION auto_check_requirement_status()
RETURNS TRIGGER AS $$
DECLARE
    req_id TEXT;
    total_tasks INT;
    closed_tasks INT;
    current_lifecycle TEXT;
BEGIN
    -- Only fire when status changes to 'closed'
    IF NEW.status != 'closed' OR OLD.status = 'closed' THEN
        RETURN NEW;
    END IF;

    -- Find all requirements this task implements
    FOR req_id IN
        SELECT e.to_id FROM edges e
        WHERE e.from_id = NEW.id AND e.edge_type = 'implements'
    LOOP
        -- Get current lifecycle status
        SELECT COALESCE(s.metadata->>'lifecycle_status', 'draft')
        INTO current_lifecycle
        FROM shards s WHERE s.id = req_id;

        -- Only auto-transition from in_progress
        IF current_lifecycle != 'in_progress' THEN
            CONTINUE;
        END IF;

        -- Count total and closed implementing tasks
        SELECT count(*),
               count(*) FILTER (WHERE t.status = 'closed')
        INTO total_tasks, closed_tasks
        FROM edges e
        JOIN shards t ON t.id = e.from_id
        WHERE e.to_id = req_id AND e.edge_type = 'implements';

        -- If all tasks are closed, set to implemented
        IF total_tasks > 0 AND total_tasks = closed_tasks THEN
            UPDATE shards
            SET metadata = jsonb_set(
                COALESCE(metadata, '{}'),
                '{lifecycle_status}',
                '"implemented"'
            ),
            updated_at = NOW()
            WHERE id = req_id;
        END IF;
    END LOOP;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create the trigger (drop first to allow re-running migration)
DROP TRIGGER IF EXISTS trg_auto_check_requirement_status ON shards;
CREATE TRIGGER trg_auto_check_requirement_status
    AFTER UPDATE OF status ON shards
    FOR EACH ROW
    EXECUTE FUNCTION auto_check_requirement_status();
