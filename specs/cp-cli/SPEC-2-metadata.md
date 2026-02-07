# SPEC-2: Structured Shard Metadata

**Status:** Draft
**Depends on:** SPEC-0
**Blocks:** SPEC-3, SPEC-4

---

## Goal

Add a JSONB metadata column to shards so structured data (lifecycle status, priority,
test coverage, version info) can be stored alongside text content. Define conventions
for structured shard types.

## What Exists

- `content TEXT` — plain text, no structure
- `labels TEXT[]` — flat tags
- `type TEXT` — shard type identifier

## What to Build

1. **JSONB metadata column** on shards table
2. **CLI commands** for metadata get/set/query
3. **Content conventions** for structured shard types
4. **Metadata schemas** per type (advisory, not enforced)

## Database Changes

### Migration: `002_metadata.sql`

```sql
-- Add metadata column with empty object default
ALTER TABLE shards ADD COLUMN metadata JSONB DEFAULT '{}';

-- GIN index for JSONB containment queries (@>, ?, ?&, ?|)
CREATE INDEX idx_shards_metadata ON shards USING gin (metadata);

-- Functional index for common lifecycle_status queries
CREATE INDEX idx_shards_metadata_lifecycle ON shards ((metadata->>'lifecycle_status'))
    WHERE metadata ? 'lifecycle_status';

-- Functional index for priority queries
CREATE INDEX idx_shards_metadata_priority ON shards ((metadata->>'priority'))
    WHERE metadata ? 'priority';

-- Helper function: merge metadata (don't replace, merge keys)
CREATE OR REPLACE FUNCTION update_metadata(
    p_shard_id TEXT,
    p_metadata JSONB
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    UPDATE shards
    SET metadata = metadata || p_metadata
    WHERE id = p_shard_id
    RETURNING metadata INTO result;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Helper function: set nested metadata value
CREATE OR REPLACE FUNCTION set_metadata_path(
    p_shard_id TEXT,
    p_path TEXT[],
    p_value JSONB
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    UPDATE shards
    SET metadata = jsonb_set(metadata, p_path, p_value, true)
    WHERE id = p_shard_id
    RETURNING metadata INTO result;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Helper function: delete metadata key
CREATE OR REPLACE FUNCTION delete_metadata_key(
    p_shard_id TEXT,
    p_key TEXT
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    UPDATE shards
    SET metadata = metadata - p_key
    WHERE id = p_shard_id
    RETURNING metadata INTO result;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Update create_shard to accept metadata
-- (Add metadata parameter to existing function signature)
CREATE OR REPLACE FUNCTION create_shard(
    p_project TEXT,
    p_creator TEXT,
    p_title TEXT,
    p_content TEXT DEFAULT NULL,
    p_type TEXT DEFAULT NULL,
    p_labels TEXT[] DEFAULT '{}',
    p_parent_id TEXT DEFAULT NULL,
    p_priority INT DEFAULT NULL,
    p_metadata JSONB DEFAULT '{}'
) RETURNS TEXT AS $$
DECLARE
    new_id TEXT;
BEGIN
    new_id := gen_shard_id(p_project);
    INSERT INTO shards (id, project, title, content, type, creator, labels, parent_id, priority, metadata)
    VALUES (new_id, p_project, p_title, p_content, p_type, p_creator, p_labels, p_parent_id, p_priority, p_metadata);
    RETURN new_id;
END;
$$ LANGUAGE plpgsql;
```

## Metadata Schemas (Advisory)

### requirement

```json
{
    "lifecycle_status": "draft|approved|in_progress|implemented|verified",
    "priority": 1,
    "category": "entity-management",
    "success_criteria_count": 6,
    "edge_case_count": 8,
    "test_coverage": { "unit": 0, "integration": 0 },
    "owner": "agent-penfold"
}
```

### knowledge

```json
{
    "doc_type": "architecture|vision|roadmap|decision|reference",
    "version": 3,
    "previous_version_id": "pf-arch-001-v2",
    "scope": "system|component|process",
    "components": ["gateway", "worker"],
    "last_reviewed": "2026-02-07",
    "reviewed_by": "agent-penfold"
}
```

### task (extend existing)

```json
{
    "implements_requirement": "pf-req-01",
    "test_shard_ids": ["pf-test-001"],
    "acceptance_verified": false,
    "acceptance_verified_by": null
}
```

### bug (extend existing)

```json
{
    "affects_requirement": "pf-req-01",
    "root_cause": "AI client timeout hardcoded at 120s",
    "fix_verified": false,
    "fix_verified_by": null
}
```

## CLI Surface

```bash
# Get all metadata for a shard
cp shard metadata get <shard-id>
# Output: JSON object

# Get specific metadata field
cp shard metadata get <shard-id> lifecycle_status
# Output: "draft"

# Get nested field
cp shard metadata get <shard-id> test_coverage.unit
# Output: 0

# Set metadata field
cp shard metadata set <shard-id> lifecycle_status approved

# Set nested field
cp shard metadata set <shard-id> test_coverage.unit 5

# Set JSON value
cp shard metadata set <shard-id> test_coverage '{"unit": 5, "integration": 2}'

# Delete metadata key
cp shard metadata delete <shard-id> deprecated_field

# Query shards by metadata
cp shard query --type requirement --meta "lifecycle_status=approved"
cp shard query --type task --meta "implements_requirement=pf-req-01"
cp shard query --meta "priority=1"

# Create shard with metadata
cp shard create --type requirement --title "Test Req" \
    --meta '{"lifecycle_status":"draft","priority":2,"category":"testing"}'
```

## Success Criteria

1. **Column exists:** `metadata JSONB DEFAULT '{}'` on shards table.
2. **Backward compatible:** All existing shards work with `metadata = '{}'`.
3. **Merge semantics:** `cp shard metadata set` merges keys, doesn't replace entire object.
4. **Nested paths:** `set test_coverage.unit 5` creates intermediate objects if needed.
5. **GIN queries:** `cp shard query --meta "key=value"` uses index, not seq scan.
6. **create_shard updated:** Accepts metadata parameter.
7. **JSON output:** `cp shard metadata get <id> -o json` returns valid JSON.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Set metadata on shard with null metadata | Initialize to `{}` then merge. |
| Query field that doesn't exist on most shards | Returns only shards that have the field. |
| Nested update `test_coverage.unit = 5` when `test_coverage` doesn't exist | Creates `{"test_coverage": {"unit": 5}}`. |
| Very large metadata (>1MB) | Reject: "Metadata too large (1.2MB). Maximum 1MB." |
| Invalid JSON in --meta flag | Reject at CLI: "Invalid JSON: <parse error>". |
| Set metadata on non-existent shard | Error: "Shard pf-xxx not found." |
| Delete non-existent key | No-op, success. |
| Concurrent metadata updates | PostgreSQL MVCC handles this. Last write wins at key level. |

---

## Test Cases

### SQL Tests: Metadata Column

```
TEST: metadata column has correct default
  Given: New shard created via INSERT without metadata
  When:  SELECT metadata FROM shards WHERE id = <new>
  Then:  Returns '{}'

TEST: metadata column accepts JSON
  Given: INSERT with metadata = '{"key": "value"}'
  When:  SELECT metadata->>'key' FROM shards WHERE id = <new>
  Then:  Returns 'value'

TEST: existing shards have null-safe metadata
  Given: Existing shards created before migration
  When:  SELECT COALESCE(metadata, '{}') FROM shards
  Then:  Returns '{}' for all (default applied by ALTER TABLE)
```

### SQL Tests: update_metadata Function

```
TEST: update_metadata merges keys
  Given: Shard with metadata = '{"a": 1}'
  When:  SELECT update_metadata(<id>, '{"b": 2}')
  Then:  metadata = '{"a": 1, "b": 2}'

TEST: update_metadata overwrites existing key
  Given: Shard with metadata = '{"a": 1, "b": 2}'
  When:  SELECT update_metadata(<id>, '{"a": 99}')
  Then:  metadata = '{"a": 99, "b": 2}'

TEST: update_metadata with non-existent shard
  Given: No shard with id 'nonexistent'
  When:  SELECT update_metadata('nonexistent', '{"a": 1}')
  Then:  RAISES EXCEPTION 'Shard nonexistent not found'

TEST: update_metadata preserves nested objects
  Given: Shard with metadata = '{"a": {"x": 1, "y": 2}}'
  When:  SELECT update_metadata(<id>, '{"a": {"x": 99}}')
  Then:  metadata = '{"a": {"x": 99}}' (top-level merge, not deep merge)
         Note: This is intentional — use set_metadata_path for nested updates
```

### SQL Tests: set_metadata_path Function

```
TEST: set_metadata_path creates nested value
  Given: Shard with metadata = '{}'
  When:  SELECT set_metadata_path(<id>, '{test_coverage,unit}', '5')
  Then:  metadata = '{"test_coverage": {"unit": 5}}'

TEST: set_metadata_path updates existing nested value
  Given: Shard with metadata = '{"test_coverage": {"unit": 3, "integration": 1}}'
  When:  SELECT set_metadata_path(<id>, '{test_coverage,unit}', '5')
  Then:  metadata = '{"test_coverage": {"unit": 5, "integration": 1}}'

TEST: set_metadata_path with string value
  Given: Shard with metadata = '{}'
  When:  SELECT set_metadata_path(<id>, '{lifecycle_status}', '"approved"')
  Then:  metadata = '{"lifecycle_status": "approved"}'
```

### SQL Tests: delete_metadata_key Function

```
TEST: delete_metadata_key removes key
  Given: Shard with metadata = '{"a": 1, "b": 2}'
  When:  SELECT delete_metadata_key(<id>, 'a')
  Then:  metadata = '{"b": 2}'

TEST: delete_metadata_key non-existent key is no-op
  Given: Shard with metadata = '{"a": 1}'
  When:  SELECT delete_metadata_key(<id>, 'nonexistent')
  Then:  metadata = '{"a": 1}' (unchanged)
```

### SQL Tests: GIN Index Queries

```
TEST: containment query uses index
  Given: 100 shards, 5 with metadata @> '{"lifecycle_status": "approved"}'
  When:  SELECT * FROM shards WHERE metadata @> '{"lifecycle_status": "approved"}'
  Then:  Returns exactly 5 shards
  And:   EXPLAIN shows idx_shards_metadata (GIN index scan)

TEST: key existence query
  Given: 100 shards, 10 with metadata ? 'category'
  When:  SELECT * FROM shards WHERE metadata ? 'category'
  Then:  Returns exactly 10 shards

TEST: nested value query
  Given: Shards with metadata.test_coverage.unit = various values
  When:  SELECT * FROM shards WHERE (metadata->'test_coverage'->>'unit')::int > 3
  Then:  Returns only shards where unit > 3
```

### SQL Tests: create_shard with Metadata

```
TEST: create_shard accepts metadata parameter
  Given: Valid project and creator
  When:  SELECT create_shard('test', 'agent', 'Title', NULL, 'requirement',
         '{}', NULL, NULL, '{"lifecycle_status": "draft", "priority": 2}')
  Then:  New shard has metadata with lifecycle_status and priority

TEST: create_shard with empty metadata
  Given: Valid project and creator
  When:  SELECT create_shard('test', 'agent', 'Title')
  Then:  New shard has metadata = '{}'
```

### Go Unit Tests: Metadata CLI

```
TEST: parseMetaFlag with simple key=value
  Given: --meta "lifecycle_status=approved"
  When:  parseMetaFlag is called
  Then:  Returns map: {"lifecycle_status": "approved"}

TEST: parseMetaFlag with JSON value
  Given: --meta '{"lifecycle_status": "approved", "priority": 2}'
  When:  parseMetaFlag is called
  Then:  Returns parsed JSON object

TEST: parseMetaFlag with invalid input
  Given: --meta "not valid"
  When:  parseMetaFlag is called
  Then:  Returns error: "Invalid metadata format. Use key=value or JSON."

TEST: parseDotPath splits correctly
  Given: "test_coverage.unit"
  When:  parseDotPath is called
  Then:  Returns ["test_coverage", "unit"]

TEST: parseDotPath single key
  Given: "lifecycle_status"
  When:  parseDotPath is called
  Then:  Returns ["lifecycle_status"]
```

### Integration Tests: Metadata Operations

```
TEST: metadata set and get round-trip
  Given: Shard created with cp shard create
  When:  `cp shard metadata set <id> lifecycle_status approved`
         `cp shard metadata get <id> lifecycle_status`
  Then:  Returns "approved"

TEST: metadata nested set and get
  Given: Shard exists
  When:  `cp shard metadata set <id> test_coverage.unit 5`
         `cp shard metadata get <id> test_coverage.unit`
  Then:  Returns "5"

TEST: metadata query returns matching shards
  Given: 3 requirement shards, 1 with lifecycle_status=approved
  When:  `cp shard query --type requirement --meta "lifecycle_status=approved"`
  Then:  Returns exactly 1 shard

TEST: metadata create with --meta flag
  Given: Valid config
  When:  `cp shard create --type requirement --title "Test" --meta '{"priority": 1}'`
         `cp shard metadata get <new-id> priority`
  Then:  Returns "1"

TEST: metadata preserved on shard update
  Given: Shard with metadata = {"a": 1, "b": 2}
  When:  Shard content updated (not metadata)
  Then:  metadata still = {"a": 1, "b": 2}
```
