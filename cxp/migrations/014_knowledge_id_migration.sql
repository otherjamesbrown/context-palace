-- Migration: Reassign knowledge doc IDs from slugs to pf-xxxxx format
-- This is a one-time data migration, not a schema change.
--
-- Strategy:
-- 1. Build mapping of old_id → new_id for all knowledge docs (main + versions)
-- 2. Drop FK constraints
-- 3. Update all references in: shards, edges, labels, read_receipts, file_claims
-- 4. Re-add FK constraints
-- 5. Output the mapping

BEGIN;

-- 1. Create temporary mapping table
CREATE TEMP TABLE id_migration (
    old_id TEXT PRIMARY KEY,
    new_id TEXT NOT NULL UNIQUE
);

-- Generate new IDs for main knowledge docs (non-version shards)
INSERT INTO id_migration (old_id, new_id)
SELECT id, 'pf-' || substr(md5(random()::text || id || clock_timestamp()::text), 1, 6)
FROM shards
WHERE type = 'knowledge'
  AND id NOT LIKE '%-v%'
  AND id NOT LIKE 'pf-%'
  AND project = 'penfold';

-- Generate new IDs for version shards: {old-slug}-vN → {new-id}-vN
-- We need to match the base slug to the mapping above
INSERT INTO id_migration (old_id, new_id)
SELECT v.id,
       m.new_id || '-v' || (regexp_match(v.id, '-v(\d+)$'))[1]
FROM shards v
JOIN id_migration m ON v.id LIKE m.old_id || '-v%'
WHERE v.type = 'knowledge'
  AND v.id LIKE '%-v%'
  AND v.id NOT LIKE 'pf-%';

-- Show the mapping before applying
SELECT old_id, new_id FROM id_migration ORDER BY old_id;

-- 2. Drop FK constraints
ALTER TABLE edges DROP CONSTRAINT IF EXISTS edges_from_id_fkey;
ALTER TABLE edges DROP CONSTRAINT IF EXISTS edges_to_id_fkey;
ALTER TABLE labels DROP CONSTRAINT IF EXISTS labels_shard_id_fkey;
ALTER TABLE shards DROP CONSTRAINT IF EXISTS shards_parent_id_fkey;
ALTER TABLE read_receipts DROP CONSTRAINT IF EXISTS read_receipts_shard_id_fkey;
ALTER TABLE file_claims DROP CONSTRAINT IF EXISTS file_claims_shard_id_fkey;
ALTER TABLE focus DROP CONSTRAINT IF EXISTS focus_epic_id_fkey;

-- 3. Update all references

-- Update shards.parent_id (children pointing to knowledge parents)
UPDATE shards SET parent_id = m.new_id
FROM id_migration m
WHERE shards.parent_id = m.old_id;

-- Update edges.from_id
UPDATE edges SET from_id = m.new_id
FROM id_migration m
WHERE edges.from_id = m.old_id;

-- Update edges.to_id
UPDATE edges SET to_id = m.new_id
FROM id_migration m
WHERE edges.to_id = m.old_id;

-- Update edges.metadata for child-of edges that might reference IDs
-- (edge metadata is JSON with description/trigger, no IDs to update)

-- Update labels.shard_id
UPDATE labels SET shard_id = m.new_id
FROM id_migration m
WHERE labels.shard_id = m.old_id;

-- Update read_receipts.shard_id
UPDATE read_receipts SET shard_id = m.new_id
FROM id_migration m
WHERE read_receipts.shard_id = m.old_id;

-- Update file_claims.shard_id
UPDATE file_claims SET shard_id = m.new_id
FROM id_migration m
WHERE file_claims.shard_id = m.old_id;

-- Update focus.epic_id (unlikely for knowledge docs, but be safe)
UPDATE focus SET epic_id = m.new_id
FROM id_migration m
WHERE focus.epic_id = m.old_id;

-- Finally, update the shards table primary key itself
UPDATE shards SET id = m.new_id
FROM id_migration m
WHERE shards.id = m.old_id;

-- 4. Re-add FK constraints
ALTER TABLE edges ADD CONSTRAINT edges_from_id_fkey
    FOREIGN KEY (from_id) REFERENCES shards(id);
ALTER TABLE edges ADD CONSTRAINT edges_to_id_fkey
    FOREIGN KEY (to_id) REFERENCES shards(id);
ALTER TABLE labels ADD CONSTRAINT labels_shard_id_fkey
    FOREIGN KEY (shard_id) REFERENCES shards(id);
ALTER TABLE shards ADD CONSTRAINT shards_parent_id_fkey
    FOREIGN KEY (parent_id) REFERENCES shards(id);
ALTER TABLE read_receipts ADD CONSTRAINT read_receipts_shard_id_fkey
    FOREIGN KEY (shard_id) REFERENCES shards(id);
ALTER TABLE file_claims ADD CONSTRAINT file_claims_shard_id_fkey
    FOREIGN KEY (shard_id) REFERENCES shards(id);
ALTER TABLE focus ADD CONSTRAINT focus_epic_id_fkey
    FOREIGN KEY (epic_id) REFERENCES shards(id) ON DELETE CASCADE;

-- 5. Verify: no old slugs remain
SELECT 'REMAINING OLD IDS:' AS check, count(*)
FROM shards
WHERE type = 'knowledge'
  AND id NOT LIKE 'pf-%'
  AND project = 'penfold';

COMMIT;
