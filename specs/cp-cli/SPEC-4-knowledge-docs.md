# SPEC-4: Knowledge Documents

**Status:** Draft
**Depends on:** SPEC-2 (metadata column)
**Blocks:** Nothing

---

## Goal

Versioned, living documents for architecture, product vision, roadmap, and design
decisions. Long-lived reference documents that evolve as the system is built. Both
agents can read and update them. Version history preserved.

## What Exists

- `doc` shard type — 12 existing shards, mostly closed
- `design` shard type — 1 existing shard
- Static `docs/` folder — not agent-writable, not versioned in Context Palace

## What to Build

1. **`knowledge` shard type** with doc_type in metadata
2. **Versioning** — preserve previous versions on update, with change summary
3. **History** — show all versions with dates, authors, summaries
4. **Diff** — text diff between any two versions
5. **Append** — add to documents (for logs/journals) without replacing
6. **`cp knowledge` commands** — full CRUD + versioning

## Data Model

### Knowledge Shard Metadata

```json
{
    "doc_type": "architecture",
    "version": 3,
    "previous_version_id": "pf-arch-001-v2",
    "scope": "system",
    "components": ["gateway", "worker"],
    "last_reviewed": "2026-02-07",
    "reviewed_by": "agent-penfold"
}
```

### Versioning Model

```
pf-arch-001 (v3, status=open) ← current
    ──[previous-version]──▶ pf-arch-001-v2 (v2, status=closed)
        ──[previous-version]──▶ pf-arch-001-v1 (v1, status=closed)
```

On update:
1. Copy current shard content to new shard `{id}-v{N}` with status=closed
2. Update current shard content and increment `metadata.version`
3. Create `previous-version` edge from current to copy
4. Edge metadata: `{"change_summary": "Added pipeline diagram", "changed_by": "agent-penfold"}`

### Document Types

| doc_type | Purpose | Example |
|----------|---------|---------|
| architecture | System components, data flow, topology | "System Architecture" |
| vision | Product purpose, success criteria | "Product Vision" |
| roadmap | Planned work, priorities, timeline | "Development Roadmap" |
| decision | Technical decisions with rationale | "Decisions Log" |
| reference | Component-specific documentation | "Gateway API Reference" |

## Database Changes

### SQL Functions

```sql
-- Update knowledge document with versioning
CREATE OR REPLACE FUNCTION update_knowledge_doc(
    p_shard_id TEXT,
    p_new_content TEXT,
    p_change_summary TEXT,
    p_changed_by TEXT
) RETURNS TABLE (shard_id TEXT, version INT) AS $$
DECLARE
    current_version INT;
    current_content TEXT;
    version_shard_id TEXT;
BEGIN
    -- Get current version and content
    SELECT
        COALESCE((metadata->>'version')::int, 1),
        content
    INTO current_version, current_content
    FROM shards WHERE id = p_shard_id AND type = 'knowledge';

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Knowledge document % not found', p_shard_id;
    END IF;

    -- Check content actually changed
    IF current_content = p_new_content THEN
        RAISE EXCEPTION 'Content is identical to current version';
    END IF;

    -- Create version snapshot (copy current content to new shard)
    version_shard_id := p_shard_id || '-v' || current_version;
    INSERT INTO shards (id, project, title, content, type, status, creator, metadata, labels)
    SELECT
        version_shard_id, project,
        title || ' (v' || current_version || ')',
        content, 'knowledge', 'closed', creator,
        jsonb_set(metadata, '{version}', to_jsonb(current_version)),
        labels
    FROM shards WHERE id = p_shard_id;

    -- Create previous-version edge
    INSERT INTO edges (from_id, to_id, edge_type, metadata)
    VALUES (p_shard_id, version_shard_id, 'previous-version',
            jsonb_build_object(
                'change_summary', p_change_summary,
                'changed_by', p_changed_by,
                'changed_at', now()::text
            ));

    -- Update current shard
    UPDATE shards
    SET content = p_new_content,
        metadata = jsonb_set(
            jsonb_set(metadata, '{version}', to_jsonb(current_version + 1)),
            '{previous_version_id}', to_jsonb(version_shard_id)
        )
    WHERE id = p_shard_id;

    RETURN QUERY SELECT p_shard_id, current_version + 1;
END;
$$ LANGUAGE plpgsql;

-- Get version history for a knowledge document
CREATE OR REPLACE FUNCTION knowledge_history(p_shard_id TEXT)
RETURNS TABLE (
    version INT,
    changed_at TIMESTAMPTZ,
    changed_by TEXT,
    change_summary TEXT,
    shard_id TEXT
) AS $$
    WITH RECURSIVE versions AS (
        -- Current version
        SELECT
            s.id,
            COALESCE((s.metadata->>'version')::int, 1) as version,
            s.updated_at as changed_at,
            s.creator as changed_by,
            'Current version' as change_summary
        FROM shards s WHERE s.id = p_shard_id

        UNION ALL

        -- Previous versions via edges
        SELECT
            e.to_id,
            COALESCE((t.metadata->>'version')::int, 1),
            COALESCE((e.metadata->>'changed_at')::timestamptz, t.created_at),
            COALESCE(e.metadata->>'changed_by', t.creator),
            COALESCE(e.metadata->>'change_summary', 'No summary')
        FROM versions v
        JOIN edges e ON e.from_id = v.id AND e.edge_type = 'previous-version'
        JOIN shards t ON t.id = e.to_id
    )
    SELECT v.version, v.changed_at, v.changed_by, v.change_summary, v.id
    FROM versions v
    ORDER BY v.version DESC;
$$ LANGUAGE sql STABLE;
```

## CLI Surface

```bash
# Create
cp knowledge create "System Architecture" \
    --doc-type architecture \
    --body-file arch.md
# Or:
cp knowledge create "System Architecture" \
    --doc-type architecture \
    --body "## Components\n..."

# List
cp knowledge list
# Output:
#   ID             DOC TYPE       VERSION  UPDATED      TITLE
#   pf-arch-001    architecture   3        2026-02-07   System Architecture
#   pf-vision-001  vision         1        2026-01-15   Product Vision
#   pf-road-001    roadmap        5        2026-02-07   Development Roadmap
#   pf-dec-001     decisions      12       2026-02-07   Decisions Log

# Read
cp knowledge show pf-arch-001

# Update (preserves previous version)
cp knowledge update pf-arch-001 \
    --body-file updated-arch.md \
    --summary "Added pipeline stage diagram"

# Append (for logs/journals — adds to end)
cp knowledge append pf-dec-001 \
    --summary "Decision: Split CLI" \
    --body "## Decision: Split CLI into penf + cp

**Date:** 2026-02-07
**Decision:** Separate Context Palace tooling from Penfold-specific CLI
**Rationale:** Reusability across projects
"

# Version history
cp knowledge history pf-arch-001
# Output:
#   VERSION  DATE        CHANGED BY       SUMMARY
#   3        2026-02-07  agent-penfold    Added pipeline stage diagram
#   2        2026-02-01  agent-mycroft    Updated deployment topology
#   1        2026-01-15  agent-penfold    Initial document

# Diff between versions
cp knowledge diff pf-arch-001 --from 2 --to 3
# Output: unified diff format

# Diff current vs previous
cp knowledge diff pf-arch-001
# Diffs version N vs N-1

# Read specific version
cp knowledge show pf-arch-001 --version 2

# JSON output
cp knowledge list -o json
cp knowledge history pf-arch-001 -o json
```

## Success Criteria

1. **Create:** `cp knowledge create` creates shard with type=knowledge, doc_type in
   metadata, version=1. Returns shard ID.
2. **List:** Shows all knowledge docs with doc_type, version, last updated date.
3. **Show:** Displays full content, metadata, version info.
4. **Update:** Preserves previous version as closed shard linked via `previous-version`
   edge. Version number increments. Change summary stored in edge metadata.
5. **Append:** Adds text to end of content. Creates version record (previous version
   preserved). Useful for decision logs, journals.
6. **History:** Shows all versions reverse-chronologically with dates, authors, summaries.
7. **Diff:** Text diff (unified format) between any two versions.
8. **Show version:** `--version N` shows content at a specific version.
9. **Both agents can update:** Changed_by field records who made each change.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Update with identical content | Error: "Content is identical to current version." |
| Concurrent updates | Last-write-wins. Both previous versions preserved (v2-a, v2-b). Warn in history. |
| Very large document (>100KB) | Allowed. Embedding truncates but content preserved. |
| Delete knowledge document | Soft delete (status=closed). All versions preserved. Warn. |
| Diff between non-adjacent versions | Works — fetches content of both versions and diffs. |
| Show non-existent version | Error: "Version 5 not found. Document has 3 versions." |
| Append to non-journal doc | Allowed. Works for any doc_type. Append is just "add to end and version." |
| Create duplicate doc_type | Allowed — can have multiple architecture docs for different components. |
| History of v1 document (never updated) | Shows single entry: version 1, creation date, creator. |

---

## Test Cases

### SQL Tests: update_knowledge_doc

```
TEST: update creates version snapshot
  Given: Knowledge doc 'test-doc' with version=1, content="Original"
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'Updated content', 'Changed X', 'agent-test')
  Then:  Returns shard_id='test-doc', version=2
         Shard 'test-doc' has content='Updated content', metadata.version=2
         Shard 'test-doc-v1' exists with content='Original', status='closed'
         Edge from 'test-doc' to 'test-doc-v1' with type='previous-version'

TEST: update increments version correctly
  Given: Knowledge doc with version=3
  When:  SELECT * FROM update_knowledge_doc(...)
  Then:  Version becomes 4
         Snapshot shard is '{id}-v3'

TEST: update rejects identical content
  Given: Knowledge doc with content="Same content"
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'Same content', 'No change', 'agent')
  Then:  RAISES EXCEPTION 'Content is identical'

TEST: update stores change summary in edge metadata
  Given: Knowledge doc exists
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'New', 'Added diagrams', 'agent-penfold')
  Then:  Edge metadata contains change_summary='Added diagrams', changed_by='agent-penfold'

TEST: update non-existent document
  Given: No shard with id 'nonexistent'
  When:  SELECT * FROM update_knowledge_doc('nonexistent', 'content', 'summary', 'agent')
  Then:  RAISES EXCEPTION 'Knowledge document nonexistent not found'

TEST: update non-knowledge shard
  Given: Shard exists but type='task'
  When:  SELECT * FROM update_knowledge_doc(...)
  Then:  RAISES EXCEPTION (NOT FOUND because WHERE type='knowledge')
```

### SQL Tests: knowledge_history

```
TEST: history of new document
  Given: Knowledge doc v1, never updated
  When:  SELECT * FROM knowledge_history('test-doc')
  Then:  Returns 1 row: version=1, change_summary='Current version'

TEST: history after updates
  Given: Knowledge doc updated twice (now v3)
  When:  SELECT * FROM knowledge_history('test-doc')
  Then:  Returns 3 rows in order: v3, v2, v1
         v3 has change_summary='Current version'
         v2 and v1 have actual change summaries from edge metadata

TEST: history includes changed_by
  Given: v1 by agent-a, v2 by agent-b, v3 by agent-a
  When:  SELECT * FROM knowledge_history('test-doc')
  Then:  changed_by values are: agent-a, agent-b, agent-a (reverse order)
```

### Go Unit Tests

```
TEST: formatDiff produces unified diff
  Given: Old content "Line 1\nLine 2\nLine 3"
         New content "Line 1\nLine 2 modified\nLine 3\nLine 4"
  When:  formatDiff(old, new)
  Then:  Output is valid unified diff with +/- lines

TEST: parseDocType validates
  Given: --doc-type "architecture"
  When:  parseDocType is called
  Then:  Returns "architecture"

TEST: parseDocType rejects unknown
  Given: --doc-type "nonsense"
  When:  parseDocType is called
  Then:  Returns error listing valid types

TEST: formatHistory table
  Given: 3 version entries
  When:  formatHistory(entries, "text")
  Then:  Aligned table with VERSION, DATE, CHANGED BY, SUMMARY columns
```

### Integration Tests

```
TEST: create knowledge document
  When:  `cp knowledge create "Test Arch" --doc-type architecture --body "# Architecture"`
  Then:  Exit code 0, returns shard ID
         `cp knowledge show <id>` shows content, doc_type=architecture, version=1

TEST: update preserves version
  Given: Knowledge doc with content "V1 content"
  When:  `cp knowledge update <id> --body "V2 content" --summary "Updated"`
  Then:  `cp knowledge show <id>` shows "V2 content", version=2
         `cp knowledge show <id> --version 1` shows "V1 content"

TEST: multiple updates preserve chain
  Given: Knowledge doc
  When:  Update 3 times
  Then:  `cp knowledge history <id>` shows 4 versions (1 original + 3 updates)
         All versions accessible via --version flag

TEST: append adds to end
  Given: Knowledge doc with content "Line 1"
  When:  `cp knowledge append <id> --body "Line 2" --summary "Added line"`
  Then:  Content is "Line 1\n\nLine 2"
         Version incremented
         Previous version preserved

TEST: diff between versions
  Given: Knowledge doc v1="Hello", v2="Hello World"
  When:  `cp knowledge diff <id> --from 1 --to 2`
  Then:  Output shows diff with +World

TEST: diff defaults to current vs previous
  Given: Knowledge doc at v3
  When:  `cp knowledge diff <id>`
  Then:  Shows diff between v3 and v2

TEST: history shows all versions
  Given: Knowledge doc updated twice
  When:  `cp knowledge history <id>`
  Then:  3 rows showing version, date, author, summary for each

TEST: list shows all documents
  Given: 2 knowledge documents
  When:  `cp knowledge list`
  Then:  Table with both documents showing doc_type, version, updated date

TEST: update identical content rejected
  Given: Knowledge doc with content "Same"
  When:  `cp knowledge update <id> --body "Same" --summary "No change"`
  Then:  Exit code 1, error: "Content is identical"

TEST: show non-existent version
  Given: Knowledge doc at v2
  When:  `cp knowledge show <id> --version 5`
  Then:  Exit code 1, error: "Version 5 not found"
```

## Seed Documents

When `cp init --seed` is run on a project, create these knowledge documents:

```bash
cp knowledge create "System Architecture" --doc-type architecture \
    --body "# System Architecture\n\n(To be documented)"
cp knowledge create "Product Vision" --doc-type vision \
    --body "# Product Vision\n\n(To be documented)"
cp knowledge create "Development Roadmap" --doc-type roadmap \
    --body "# Development Roadmap\n\n(To be documented)"
cp knowledge create "Decisions Log" --doc-type decision \
    --body "# Decisions Log\n\nRecord key technical decisions here."
```
