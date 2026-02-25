-- Migration 015: Rename agent-cxp to agent-steve
-- Context Palace developer agent identity change.
-- Run manually on dev02 after review.

BEGIN;

-- 1. shards: creator, owner, closed_by
UPDATE shards SET creator = 'agent-steve' WHERE creator = 'agent-cxp';
UPDATE shards SET owner = 'agent-steve' WHERE owner = 'agent-cxp';
UPDATE shards SET closed_by = 'agent-steve' WHERE closed_by = 'agent-cxp';

-- 2. read_receipts
UPDATE read_receipts SET agent_id = 'agent-steve' WHERE agent_id = 'agent-cxp';

-- 3. focus
UPDATE focus SET agent = 'agent-steve' WHERE agent = 'agent-cxp';

-- 4. cli_commands
UPDATE cli_commands SET agent = 'agent-steve' WHERE agent = 'agent-cxp';

-- 5. labels: to: and cc: routing labels
UPDATE labels SET label = 'to:agent-steve' WHERE label = 'to:agent-cxp';
UPDATE labels SET label = 'cc:agent-steve' WHERE label = 'cc:agent-cxp';

-- 6. shards.metadata->>'last_changed_by'
UPDATE shards
SET metadata = jsonb_set(metadata, '{last_changed_by}', '"agent-steve"')
WHERE metadata->>'last_changed_by' = 'agent-cxp';

-- 7. shards.metadata->'access_log' array entries
-- Replace agent-cxp in JSONB array elements
UPDATE shards
SET metadata = (
    SELECT jsonb_set(metadata, '{access_log}', new_log)
    FROM (
        SELECT jsonb_agg(
            CASE
                WHEN elem::text LIKE '%agent-cxp%'
                THEN replace(elem::text, 'agent-cxp', 'agent-steve')::jsonb
                ELSE elem
            END
        ) AS new_log
        FROM jsonb_array_elements(metadata->'access_log') AS elem
    ) sub
)
WHERE metadata ? 'access_log'
  AND metadata->>'access_log' LIKE '%agent-cxp%';

-- 8. edges.metadata->>'changed_by'
UPDATE edges
SET metadata = jsonb_set(metadata, '{changed_by}', '"agent-steve"')
WHERE metadata->>'changed_by' = 'agent-cxp';

COMMIT;
