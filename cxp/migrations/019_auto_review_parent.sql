-- Migration 019: Auto-review parent when last child closes
-- Drop legacy requirement trigger and add new parent auto-review trigger

-- Drop the legacy requirement trigger (requirement type is retired)
DROP TRIGGER IF EXISTS trg_auto_check_requirement_status ON shards;
DROP FUNCTION IF EXISTS auto_check_requirement_status();

-- Auto-transition parent work items to needs-review when last open child closes
CREATE OR REPLACE FUNCTION auto_review_parent_on_last_child_closed()
RETURNS TRIGGER AS $$
DECLARE
    parent_rec RECORD;
    open_siblings INT;
BEGIN
    -- Only fire when status changes to 'closed'
    IF NEW.status = 'closed' AND OLD.status != 'closed' THEN
        -- Find parent(s) via child-of edges
        FOR parent_rec IN
            SELECT s.id, s.type, s.status
            FROM edges e
            JOIN shards s ON e.to_id = s.id
            WHERE e.from_id = NEW.id
            AND e.edge_type = 'child-of'
            AND s.type IN ('design', 'bug', 'task')
            AND s.status IN ('open', 'ready', 'in_progress')
        LOOP
            -- Count remaining open children (excluding the one just closed)
            SELECT count(*) INTO open_siblings
            FROM edges e2
            JOIN shards s2 ON e2.from_id = s2.id
            WHERE e2.to_id = parent_rec.id
            AND e2.edge_type = 'child-of'
            AND s2.status NOT IN ('closed', 'deferred')
            AND s2.id != NEW.id;

            IF open_siblings = 0 THEN
                UPDATE shards SET status = 'needs-review'
                WHERE id = parent_rec.id;
            END IF;
        END LOOP;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_auto_review_parent
    AFTER UPDATE OF status ON shards
    FOR EACH ROW
    EXECUTE FUNCTION auto_review_parent_on_last_child_closed();
