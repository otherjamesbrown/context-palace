# SPEC-4: Knowledge Documents

**Status:** Draft
**Depends on:** SPEC-0 (CLI skeleton), SPEC-2 (metadata column)
**Blocks:** Nothing (SPEC-5 can use knowledge docs but doesn't depend on this spec)

---

## Goal

Versioned, living documents for architecture, product vision, roadmap, and design
decisions. Long-lived reference documents that evolve as the system is built. Both
agents can read and update them. Version history preserved. Provides the stable
knowledge layer that hierarchical memory (SPEC-6) can reference.

## What Exists

- `doc` shard type — 12 existing shards, mostly closed
- `design` shard type — 1 existing shard
- Static `docs/` folder — not agent-writable, not versioned in Context Palace
- `parent_id` column on shards (can express doc hierarchy but not used here)
- `create_shard()` SQL function (SPEC-0) — creates shards with metadata
- JSONB metadata column with GIN index (SPEC-2) — stores doc_type, version, etc.

## What to Build

1. **`knowledge` shard type** with doc_type in metadata — CLI creates, lists, filters by type
2. **Versioning** — preserve previous versions on update as closed snapshot shards linked via `previous-version` edge; change summary stored in edge metadata
3. **History** — show all versions reverse-chronologically with dates, authors, summaries
4. **Diff** — text diff (unified format) between any two versions
5. **Append** — atomically add to end of document content and version, for logs/journals
6. **`cp knowledge` commands** — create, list, show, update, append, history, diff (7 commands)
7. **Version lookup** — retrieve content at any historical version by number

## Data Model

### Schema Changes

No new tables or columns. Uses existing:
- `shards` table with `type = 'knowledge'`, `status`, `metadata JSONB`
- `edges` table with `edge_type = 'previous-version'`
- `metadata JSONB DEFAULT '{}'` (from SPEC-2 migration `002_metadata.sql`)
- GIN index on `metadata` (from SPEC-2)

### Storage Format

Knowledge shard metadata schema:

```json
{
    "doc_type": "architecture",
    "version": 3,
    "previous_version_id": "pf-arch-001-v2",
    "last_changed_by": "agent-penfold",
    "last_change_summary": "Added pipeline stage diagram"
}
```

| Field | Type | Required | Set By | Description |
|-------|------|----------|--------|-------------|
| `doc_type` | string | Yes (on create) | `cp knowledge create --doc-type` | One of: architecture, vision, roadmap, decision, reference |
| `version` | integer | Yes | SQL functions (auto-managed) | Current version number, starts at 1 |
| `previous_version_id` | string | No (null for v1) | `update_knowledge_doc()` / `append_knowledge_doc()` | Shard ID of the most recent snapshot |
| `last_changed_by` | string | Yes | SQL functions | Identity of the agent/user who made the most recent change. Source: `agent_name` from `~/.cp/config.yaml`, or `--agent` CLI flag if overridden. Falls back to `$USER` environment variable if neither is set. |
| `last_change_summary` | string | Yes | SQL functions | Change summary for the current version. Set to "Initial document" on create, then updated on each `update`/`append`. Copied to snapshot metadata before overwrite, so each snapshot carries its own summary. |

Version snapshot shard metadata inherits parent metadata with version frozen at the
snapshot's version number. Snapshots always have `status = 'closed'`.

Edge metadata for `previous-version` edges:

```json
{
    "change_summary": "Added pipeline stage diagram",
    "changed_by": "agent-penfold",
    "changed_at": "2026-02-07T12:00:00Z"
}
```

### Document Types

| doc_type | Purpose | Example |
|----------|---------|---------|
| architecture | System components, data flow, topology | "System Architecture" |
| vision | Product purpose, success criteria | "Product Vision" |
| roadmap | Planned work, priorities, timeline | "Development Roadmap" |
| decision | Technical decisions with rationale | "Decisions Log" |
| reference | Component-specific documentation | "Gateway API Reference" |

### Versioning Model

```
pf-arch-001 (v3, status=open) ← current
    ──[previous-version]──▶ pf-arch-001-v2 (v2, status=closed)
        ──[previous-version]──▶ pf-arch-001-v1 (v1, status=closed)
```

On update:
1. Lock current shard row (`SELECT ... FOR UPDATE`)
2. Copy current shard content to new snapshot shard `{id}-v{N}` with `status=closed`
3. Create `previous-version` edge from current shard to snapshot
4. Update current shard: new content, increment version, set `last_changed_by`, update `updated_at`
5. Return new version number

### Data Flow

#### `doc_type`

1. **WHO writes it?** `cp knowledge create --doc-type <type>` (user/agent via CLI)
2. **WHEN is it written?** On creation only. Immutable after.
3. **WHERE is it stored?** `shards.metadata->>'doc_type'`
4. **WHO reads it?** `cp knowledge list` (for display), `cp knowledge list --doc-type <type>` (for filtering)
5. **HOW is it queried?** `metadata @> '{"doc_type": "architecture"}'::jsonb` (uses GIN index)
6. **WHAT decisions does it inform?** Filtering and categorization in list views. Seed document templates.
7. **DOES it go stale?** No — immutable after creation. If doc purpose changes, create a new doc.

#### `version` (integer counter)

1. **WHO writes it?** `update_knowledge_doc()` and `append_knowledge_doc()` SQL functions (auto-increment)
2. **WHEN is it written?** On every content update or append. Set to 1 on create.
3. **WHERE is it stored?** `shards.metadata->>'version'` (current shard) and snapshot shards' metadata
4. **WHO reads it?** `cp knowledge show` (display), `cp knowledge list` (display), `cp knowledge history` (ordering), `cp knowledge show --version N` (lookup)
5. **HOW is it queried?** Direct metadata read for display; `knowledge_history()` recursive CTE for full chain; `knowledge_version()` for specific version lookup
6. **WHAT decisions does it inform?** Version display, snapshot naming (`{id}-v{N}`), history ordering
7. **DOES it go stale?** No — monotonically increasing, managed exclusively by SQL functions within transactions.

#### `previous_version_id`

1. **WHO writes it?** `update_knowledge_doc()` and `append_knowledge_doc()` SQL functions
2. **WHEN is it written?** On every content update. Null for v1 documents.
3. **WHERE is it stored?** `shards.metadata->>'previous_version_id'` on the current shard
4. **WHO reads it?** Not directly queried — the `previous-version` edge is the authoritative link. This field is a convenience denormalization.
5. **HOW is it queried?** Direct metadata read. The edge chain is the primary traversal path.
6. **WHAT decisions does it inform?** Quick access to the immediately previous version without edge traversal.
7. **DOES it go stale?** No — set atomically during update transaction. If the snapshot shard is deleted externally, this becomes a dangling reference. The edge would also be orphaned.

#### `last_change_summary` (on shard metadata)

1. **WHO writes it?** SQL functions: `create_shard()` sets "Initial document", `update_knowledge_doc()` and `append_knowledge_doc()` set the user-provided summary.
2. **WHEN is it written?** On creation (default "Initial document") and on every update/append. During update, the current shard's existing `last_change_summary` is first copied to the snapshot shard's metadata, then overwritten with the new summary.
3. **WHERE is it stored?** `shards.metadata->>'last_change_summary'` on both the current shard and all snapshot shards.
4. **WHO reads it?** `cp knowledge history` (display column), `cp knowledge show` (display).
5. **HOW is it queried?** `knowledge_history()` reads directly from each shard's metadata. No cross-join to edges needed.
6. **WHAT decisions does it inform?** Human/agent review of change history — what changed and why at each version.
7. **DOES it go stale?** No — immutable on snapshots, overwritten atomically on current shard during updates.

#### `change_summary` (on edges — audit trail)

1. **WHO writes it?** `update_knowledge_doc()` and `append_knowledge_doc()` — passed in from CLI `--summary` flag
2. **WHEN is it written?** On every update/append, stored on the `previous-version` edge.
3. **WHERE is it stored?** `edges.metadata->>'change_summary'` on `previous-version` edges
4. **WHO reads it?** `cp knowledge history` (display column)
5. **HOW is it queried?** `knowledge_history()` recursive CTE joins edges and reads metadata
6. **WHAT decisions does it inform?** Human/agent review of change history — what changed and why
7. **DOES it go stale?** No — immutable once written. Describes the transition, not the current state.

### Concurrency

**Update/Append race condition:** Two concurrent `update_knowledge_doc()` calls could
both read the same version N and attempt to create snapshot `{id}-v{N}`, causing a
primary key collision.

**Solution:** `SELECT ... FOR UPDATE` locks the shard row at the start of the transaction.
The second caller blocks until the first transaction commits, then reads the incremented
version number. No retry logic needed — PostgreSQL serializes the operations.

```sql
-- Inside update_knowledge_doc / append_knowledge_doc:
SELECT ... INTO current_version, current_content
FROM shards WHERE id = p_shard_id AND type = 'knowledge'
FOR UPDATE;  -- Row-level lock, held until COMMIT
```

**Isolation level:** Default `READ COMMITTED` is sufficient since the `FOR UPDATE` lock
prevents concurrent reads of stale version numbers.

**Snapshot shard deletion:** If an external process deletes a version snapshot shard,
the `previous-version` edge becomes orphaned. `knowledge_history()` will skip the
missing version (the JOIN excludes it). The version chain will have a gap but won't error.

## CLI Surface

### `cp knowledge create` — Create a knowledge document

```bash
# With inline body
cp knowledge create "System Architecture" \
    --doc-type architecture \
    --body "## Components\n..."

# With file body
cp knowledge create "System Architecture" \
    --doc-type architecture \
    --body-file arch.md
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--doc-type` | Yes | — | One of: architecture, vision, roadmap, decision, reference |
| `--body` | One of body/body-file required | — | Inline content |
| `--body-file` | One of body/body-file required | — | Read content from file |
| `--label` | No | — | Labels (repeatable) |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Validate `--doc-type` is one of the allowed types. Error if not.
2. Validate that `--body` or `--body-file` is provided. Error if neither.
3. If `--body-file`, read file contents. Error if file doesn't exist or is unreadable.
4. Call `create_shard()` (SPEC-0) with `type='knowledge'`, `metadata={"doc_type": <type>, "version": 1, "last_changed_by": <agent>, "last_change_summary": "Initial document"}`.
5. If embedding is configured (SPEC-1), generate embedding for `"knowledge: <title>\n\n<content>"`. On failure, proceed without embedding (graceful degradation).
6. Print shard ID and confirmation.

**Output (text):**
```
Created knowledge document pf-arch-001 (architecture, v1)
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-arch-001",
  "title": "System Architecture",
  "doc_type": "architecture",
  "version": 1,
  "created_at": "2026-02-07T12:00:00Z"
}
```

---

### `cp knowledge list` — List knowledge documents

```bash
# All knowledge docs
cp knowledge list

# Filter by doc_type
cp knowledge list --doc-type architecture

# JSON output
cp knowledge list -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--doc-type` | No | — | Filter by document type |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Query shards where `type = 'knowledge'` and `status = 'open'` and `project = <current>`.
2. If `--doc-type`, add filter `metadata @> '{"doc_type": "<type>"}'`.
3. Order by `updated_at DESC`.
4. Format as table or JSON.

**Output (text):**
```
ID             DOC TYPE       VERSION  UPDATED      TITLE
pf-arch-001    architecture   3        2026-02-07   System Architecture
pf-vision-001  vision         1        2026-01-15   Product Vision
pf-road-001    roadmap        5        2026-02-07   Development Roadmap
pf-dec-001     decision       12       2026-02-07   Decisions Log
```

**JSON output (`-o json`):**
```json
[
  {
    "id": "pf-arch-001",
    "title": "System Architecture",
    "doc_type": "architecture",
    "version": 3,
    "updated_at": "2026-02-07T12:00:00Z",
    "created_at": "2026-01-15T10:00:00Z"
  }
]
```

---

### `cp knowledge show` — Display a knowledge document

```bash
# Show current version
cp knowledge show pf-arch-001

# Show specific historical version
cp knowledge show pf-arch-001 --version 2

# JSON output
cp knowledge show pf-arch-001 -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--version` | No | current | Version number to display |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. If `--version` specified, call `knowledge_version(shard_id, version_num)` to get the snapshot shard ID. Error if version not found.
2. Fetch shard content, metadata, labels.
3. Display title, ID, doc_type, version, created/updated dates, labels, content.

**Output (text):**
```
System Architecture (pf-arch-001)
Type: architecture | Version: 3 | Updated: 2026-02-07
Labels: architecture, core

## Components
...
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-arch-001",
  "title": "System Architecture",
  "content": "## Components\n...",
  "doc_type": "architecture",
  "version": 3,
  "metadata": {"doc_type": "architecture", "version": 3, "last_changed_by": "agent-penfold"},
  "labels": ["architecture", "core"],
  "created_at": "2026-01-15T10:00:00Z",
  "updated_at": "2026-02-07T12:00:00Z"
}
```

---

### `cp knowledge update` — Update document content (versioned)

```bash
cp knowledge update pf-arch-001 \
    --body-file updated-arch.md \
    --summary "Added pipeline stage diagram"
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--body` | One required | — | New content (inline) |
| `--body-file` | One required | — | New content (from file) |
| `--summary` | Yes | — | Change summary describing what changed and why |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Validate `--summary` is provided. Error if missing: "Update requires --summary to describe the change."
2. Read new content from `--body` or `--body-file`.
3. Call `update_knowledge_doc(shard_id, new_content, summary, agent, project)`.
4. If embedding is configured (SPEC-1), regenerate embedding for updated content. On failure, log warning but don't fail the update.
5. Print new version number.

**Output (text):**
```
Updated pf-arch-001 to v4
Previous version preserved as pf-arch-001-v3
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-arch-001",
  "version": 4,
  "previous_version_id": "pf-arch-001-v3",
  "summary": "Added pipeline stage diagram"
}
```

---

### `cp knowledge append` — Append content to document (versioned)

```bash
cp knowledge append pf-dec-001 \
    --summary "Decision: Split CLI" \
    --body "## Decision: Split CLI into penf + cp

**Date:** 2026-02-07
**Decision:** Separate Context Palace tooling from Penfold-specific CLI
**Rationale:** Reusability across projects
"
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--body` | One required | — | Content to append (inline) |
| `--body-file` | One required | — | Content to append (from file) |
| `--summary` | Yes | — | Change summary |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Validate `--summary` is provided. Error if missing.
2. Read append content from `--body` or `--body-file`.
3. Call `append_knowledge_doc(shard_id, append_content, summary, agent, project)`.
   Internally: reads current content, concatenates with `\n\n`, then performs same
   versioning steps as `update_knowledge_doc`.
4. If embedding configured (SPEC-1), regenerate embedding. On failure, log warning.
5. Print new version number.

**Output (text):**
```
Appended to pf-dec-001, now v13
Previous version preserved as pf-dec-001-v12
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-dec-001",
  "version": 13,
  "previous_version_id": "pf-dec-001-v12",
  "summary": "Decision: Split CLI"
}
```

---

### `cp knowledge history` — Show version history

```bash
cp knowledge history pf-arch-001
cp knowledge history pf-arch-001 -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Call `knowledge_history(shard_id, project)`.
2. Format results as table or JSON.

**Output (text):**
```
VERSION  DATE        CHANGED BY       SUMMARY
3        2026-02-07  agent-penfold    Added pipeline stage diagram
2        2026-02-01  agent-mycroft    Updated deployment topology
1        2026-01-15  agent-penfold    Initial document
```

**JSON output (`-o json`):**
```json
[
  {
    "version": 3,
    "changed_at": "2026-02-07T12:00:00Z",
    "changed_by": "agent-penfold",
    "change_summary": "Added pipeline stage diagram",
    "shard_id": "pf-arch-001"
  },
  {
    "version": 2,
    "changed_at": "2026-02-01T09:00:00Z",
    "changed_by": "agent-mycroft",
    "change_summary": "Updated deployment topology",
    "shard_id": "pf-arch-001-v2"
  },
  {
    "version": 1,
    "changed_at": "2026-01-15T10:00:00Z",
    "changed_by": "agent-penfold",
    "change_summary": "Initial document",
    "shard_id": "pf-arch-001-v1"
  }
]
```

---

### `cp knowledge diff` — Diff between versions

```bash
# Diff current vs previous
cp knowledge diff pf-arch-001

# Diff specific versions
cp knowledge diff pf-arch-001 --from 1 --to 3

# JSON output
cp knowledge diff pf-arch-001 -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--from` | No | version N-1 | Source version number |
| `--to` | No | version N (current) | Target version number |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Resolve `--from` and `--to` version numbers. If neither specified, diff current (N) vs previous (N-1).
2. If `--from > --to`, swap them silently (always diff older→newer).
3. Call `knowledge_version(shard_id, from_version)` and `knowledge_version(shard_id, to_version)` to get content for both versions.
4. Generate unified diff.
5. Print diff or JSON.

**Output (text):**
```
--- pf-arch-001 v2
+++ pf-arch-001 v3
@@ -10,3 +10,7 @@
 ## Pipeline
-Workers process shards sequentially.
+Workers process shards in parallel with configurable concurrency.
+
+### Stage Diagram
+
+[diagram content]
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-arch-001",
  "from_version": 2,
  "to_version": 3,
  "diff": "--- pf-arch-001 v2\n+++ pf-arch-001 v3\n@@ -10,3 +10,7 @@..."
}
```

## Workflows

### Update Workflow

```
cp knowledge update <id> --body-file new.md --summary "..."
    │
    ▼
[1] Validate --summary provided ──── missing ──▶ Error: "--summary required"
    │
    ▼
[2] Read new content (--body or --body-file)
    │
    ▼
[3] BEGIN TRANSACTION
    │
    ▼
[4] SELECT ... FOR UPDATE (lock shard row)
    │   │
    │   └── NOT FOUND ──▶ ROLLBACK ──▶ Error: "Knowledge document not found"
    │
    ▼
[5] Check content != current content
    │   │
    │   └── IDENTICAL ──▶ ROLLBACK ──▶ Error: "Content is identical"
    │
    ▼
[6] INSERT snapshot shard {id}-v{N}
    │   │
    │   └── PK CONFLICT (shouldn't happen with FOR UPDATE)
    │       ──▶ ROLLBACK ──▶ Error: "Version snapshot already exists"
    │
    ▼
[7] INSERT previous-version edge
    │
    ▼
[8] UPDATE current shard (content, version, updated_at, last_changed_by)
    │
    ▼
[9] COMMIT
    │
    ▼
[10] Regenerate embedding (SPEC-1, async-safe)
    │   │
    │   └── FAILURE ──▶ Log warning, continue (embedding stale until next update)
    │
    ▼
[11] Print confirmation

Non-interactive: All steps are non-interactive. No prompts or confirmations.
```

### Append Workflow

Same as Update Workflow except step [5] is replaced with:
- [5] Concatenate: `current_content || '\n\n' || new_content`
- No "identical content" check (appending always produces different content)

## SQL Functions

```sql
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
```

## Go Implementation Notes

### Package Structure

```
cp/
├── cmd/
│   └── knowledge.go          # Cobra commands: create, list, show, update, append, history, diff
└── internal/
    └── client/
        └── knowledge.go       # DB operations: CreateKnowledgeDoc, UpdateKnowledgeDoc, etc.
```

### Key Types

```go
package client

type KnowledgeDoc struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Content     string    `json:"content,omitempty"`
    DocType     string    `json:"doc_type"`
    Version     int       `json:"version"`
    Labels      []string  `json:"labels,omitempty"`
    Metadata    map[string]any `json:"metadata,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type VersionEntry struct {
    Version       int       `json:"version"`
    ChangedAt     time.Time `json:"changed_at"`
    ChangedBy     string    `json:"changed_by"`
    ChangeSummary string    `json:"change_summary"`
    ShardID       string    `json:"shard_id"`
}

type UpdateResult struct {
    ID                string `json:"id"`
    Version           int    `json:"version"`
    PreviousVersionID string `json:"previous_version_id"`
    Summary           string `json:"summary"`
}

// Valid document types
var ValidDocTypes = []string{"architecture", "vision", "roadmap", "decision", "reference"}
```

### Key Flows

```go
// UpdateKnowledgeDoc — calls the SQL function within a single connection
func (c *Client) UpdateKnowledgeDoc(ctx context.Context, id, content, summary, agent string) (*UpdateResult, error) {
    conn, err := c.pool.Acquire(ctx)
    if err != nil {
        return nil, fmt.Errorf("acquire connection: %w", err)
    }
    defer conn.Release()

    var result UpdateResult
    err = conn.QueryRow(ctx,
        "SELECT shard_id, version FROM update_knowledge_doc($1, $2, $3, $4, $5)",
        id, content, summary, agent, c.project,
    ).Scan(&result.ID, &result.Version)
    if err != nil {
        // Map PostgreSQL exceptions to user-friendly errors
        if strings.Contains(err.Error(), "not found") {
            return nil, fmt.Errorf("knowledge document %s not found", id)
        }
        if strings.Contains(err.Error(), "identical") {
            return nil, fmt.Errorf("content is identical to current version")
        }
        return nil, fmt.Errorf("update knowledge doc: %w", err)
    }

    result.PreviousVersionID = fmt.Sprintf("%s-v%d", id, result.Version-1)
    result.Summary = summary

    // Regenerate embedding (non-blocking failure)
    if c.embedder != nil {
        if embErr := c.EmbedShard(ctx, id); embErr != nil {
            log.Printf("warning: failed to regenerate embedding for %s: %v", id, embErr)
        }
    }

    return &result, nil
}

// DiffVersions — fetches content at two versions and produces unified diff
func (c *Client) DiffVersions(ctx context.Context, id string, from, to int) (string, error) {
    fromContent, err := c.GetVersionContent(ctx, id, from)
    if err != nil {
        return "", fmt.Errorf("fetch version %d: %w", from, err)
    }
    toContent, err := c.GetVersionContent(ctx, id, to)
    if err != nil {
        return "", fmt.Errorf("fetch version %d: %w", to, err)
    }

    diff := difflib.UnifiedDiff{
        A:        difflib.SplitLines(fromContent),
        B:        difflib.SplitLines(toContent),
        FromFile: fmt.Sprintf("%s v%d", id, from),
        ToFile:   fmt.Sprintf("%s v%d", id, to),
        Context:  3,
    }
    return difflib.GetUnifiedDiffString(diff)
}
```

**Diff library:** Use `github.com/pmezard/go-difflib/difflib` (standard Go diff library,
same as used by `testify`).

## Success Criteria

1. **Create:** `cp knowledge create` creates shard with `type=knowledge`, `doc_type` in
   metadata, `version=1`, `last_changed_by` set. Returns shard ID.
2. **List:** `cp knowledge list` shows all knowledge docs with doc_type, version, last
   updated date. `--doc-type` filter works. JSON output matches schema.
3. **Show:** `cp knowledge show` displays full content, metadata, version info, labels.
   JSON output matches schema.
4. **Update:** `update_knowledge_doc()` preserves previous version as closed snapshot shard
   linked via `previous-version` edge. Version increments. `updated_at` is refreshed.
   Change summary stored in edge metadata. `--summary` is required.
5. **Append:** `append_knowledge_doc()` concatenates content, then versions same as update.
   Atomic — no race between read and concatenate.
6. **History:** `knowledge_history()` returns all versions reverse-chronologically with
   dates, authors, summaries. Depth-limited to prevent infinite recursion.
7. **Diff:** Text diff (unified format) between any two versions. Default diffs current
   vs previous. `--from > --to` swaps silently.
8. **Show version:** `--version N` shows content at a specific version via
   `knowledge_version()`. Errors if version doesn't exist with the current version count.
9. **Both agents can update:** `last_changed_by` and edge `changed_by` record who made
   each change. History shows different agents correctly.
10. **Concurrency safe:** `FOR UPDATE` lock prevents version collision on concurrent updates.
11. **Project scoped:** All SQL functions take `p_project` parameter. No cross-project access.
12. **Closed doc protection:** `update_knowledge_doc()` and `append_knowledge_doc()` reject closed documents with a clear error. `cp knowledge show` works on closed docs (read-only).
13. **Change summary preserved per version:** Each shard (current and snapshot) carries its own `last_change_summary` in metadata. History reads from shard metadata, not edges.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Update with identical content | Error: "Content is identical to current version." |
| Concurrent updates | `FOR UPDATE` serializes — second update waits, then reads incremented version. No collision. |
| Very large document (>100KB) | Allowed. Embedding truncates content but full content preserved in shard. |
| Delete knowledge document | Use `cp shard close <id>` (SPEC-5). Status set to closed. All version snapshots preserved. |
| Diff between non-adjacent versions | Works — `knowledge_version()` fetches content at both versions, diff computed client-side. |
| Show non-existent version | Error: "Version 5 not found. Document has 3 versions." |
| Append to any doc_type | Allowed. Works for any doc_type. Append is concatenate + version. |
| Create duplicate doc_type | Allowed — can have multiple architecture docs for different components. |
| History of v1 document (never updated) | Single entry: version 1, creation date, `last_changed_by`, "Initial document". |
| Update without `--summary` | Error: "Update requires --summary to describe the change." |
| Append without `--summary` | Error: "Append requires --summary to describe the change." |
| Diff on v1 document (never updated) | Error: "Document has only 1 version. Nothing to diff." |
| Diff with `--from` > `--to` | Silently swapped. Diff always shows older→newer. |
| Invalid `--doc-type` on create | Error: "Invalid doc_type 'foo'. Valid types: architecture, vision, roadmap, decision, reference" |
| Neither `--body` nor `--body-file` | Error: "Either --body or --body-file is required." |
| `--body-file` with nonexistent file | Error: "Cannot read file 'missing.md': no such file or directory" |
| Version snapshot shard deleted externally | `knowledge_history()` skips the gap (JOIN excludes missing shards). `knowledge_version()` for that version returns error. |
| Snapshot ID already exists | Error: "Version snapshot pf-arch-001-v2 already exists" (guard clause in SQL). |
| `cp shard update` on type=knowledge | Bypasses versioning — updates content directly without creating snapshot. This is intentional: `cp shard update` is the low-level tool, `cp knowledge update` is the versioned wrapper. |
| `cp knowledge show` on closed document | Works — closed documents are still readable. Shows content with a "(closed)" indicator. |
| `cp knowledge update` on closed document | Error: "Knowledge document pf-arch-001 is closed. Reopen with cp shard reopen before updating." |
| `cp knowledge append` on closed document | Error: "Knowledge document pf-arch-001 is closed. Reopen with cp shard reopen before appending." |
| `cp knowledge list` shows closed docs? | No — list defaults to `status = 'open'`. Use `cp shard list --type knowledge --status closed` to find closed knowledge docs. |
| Embedding generation fails during update | Content update succeeds. Warning logged. Embedding stale until next successful embed. |
| Recursive CTE cycle in version edges | Depth cap at 1000 prevents infinite loop. |

---

## Cross-Spec Interactions

| Spec | Interaction |
|------|-------------|
| **SPEC-0** (CLI skeleton) | Uses `create_shard()` for knowledge doc creation. Inherits `--project` and `-o` flags. |
| **SPEC-1** (semantic search) | Knowledge docs get embeddings on create/update. Version snapshots (closed) searchable via `cp recall --include-closed`. Embedding regenerated by Go code after SQL function returns. |
| **SPEC-2** (metadata) | Uses JSONB metadata column for `doc_type`, `version`, `last_changed_by`, `previous_version_id`. GIN index used for `--doc-type` filtering. |
| **SPEC-3** (requirements) | No direct interaction. Requirements can reference knowledge docs via `relates-to` edges but this is not managed by SPEC-4. |
| **SPEC-5** (unified search) | Knowledge docs appear in `cp shard list --type knowledge`. `cp shard close <id>` is the delete mechanism. `cp shard update` on knowledge shards bypasses versioning (low-level tool). |
| **SPEC-6** (hierarchical memory) | Knowledge docs can be referenced by memory pointer blocks. Not managed by SPEC-4. |

## Test Cases

### SQL Tests: update_knowledge_doc

```
TEST: update creates version snapshot
  Given: Knowledge doc 'test-doc' with version=1, content="Original"
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'Updated content', 'Changed X', 'agent-test', 'test-project')
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
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'Same content', 'No change', 'agent', 'test-project')
  Then:  RAISES EXCEPTION 'Content is identical'

TEST: update stores change summary in edge metadata
  Given: Knowledge doc exists
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'New', 'Added diagrams', 'agent-penfold', 'test-project')
  Then:  Edge metadata contains change_summary='Added diagrams', changed_by='agent-penfold'

TEST: update non-existent document
  Given: No shard with id 'nonexistent'
  When:  SELECT * FROM update_knowledge_doc('nonexistent', 'content', 'summary', 'agent', 'test-project')
  Then:  RAISES EXCEPTION 'Knowledge document nonexistent not found'

TEST: update non-knowledge shard
  Given: Shard exists but type='task'
  When:  SELECT * FROM update_knowledge_doc(...)
  Then:  RAISES EXCEPTION 'not found' (WHERE type='knowledge' excludes it)

TEST: update sets updated_at
  Given: Knowledge doc, note original updated_at
  When:  SELECT * FROM update_knowledge_doc(...)
  Then:  Shard's updated_at > original updated_at

TEST: update sets last_changed_by in metadata
  Given: Knowledge doc created by agent-penfold
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'New', 'Change', 'agent-mycroft', 'test-project')
  Then:  Shard metadata->>'last_changed_by' = 'agent-mycroft'

TEST: update with FOR UPDATE prevents concurrent collision
  Given: Knowledge doc at v1
  When:  Two concurrent update_knowledge_doc calls
  Then:  Both succeed — first gets v2, second gets v3 (serialized by lock)
         Two snapshot shards exist: {id}-v1, {id}-v2

TEST: update guards against snapshot ID collision
  Given: Knowledge doc at v1, and shard 'test-doc-v1' already exists (manually created)
  When:  SELECT * FROM update_knowledge_doc(...)
  Then:  RAISES EXCEPTION 'Version snapshot test-doc-v1 already exists'

TEST: update copies last_change_summary to snapshot
  Given: Knowledge doc at v2, metadata last_change_summary='Updated topology'
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'New content', 'Added diagrams', 'agent', 'test-project')
  Then:  Snapshot 'test-doc-v2' metadata last_change_summary = 'Updated topology'
         Current shard metadata last_change_summary = 'Added diagrams'

TEST: update rejects closed document
  Given: Knowledge doc with status='closed'
  When:  SELECT * FROM update_knowledge_doc('test-doc', 'content', 'summary', 'agent', 'test-project')
  Then:  RAISES EXCEPTION 'Knowledge document test-doc is closed'
```

### SQL Tests: append_knowledge_doc

```
TEST: append concatenates content and versions
  Given: Knowledge doc with content="Line 1", version=1
  When:  SELECT * FROM append_knowledge_doc('test-doc', 'Line 2', 'Added line', 'agent', 'test-project')
  Then:  Returns version=2
         Content is "Line 1\n\nLine 2"
         Snapshot 'test-doc-v1' has content="Line 1"

TEST: append to empty content
  Given: Knowledge doc with content=""
  When:  SELECT * FROM append_knowledge_doc('test-doc', 'First content', 'Initial', 'agent', 'test-project')
  Then:  Content is "\n\nFirst content"
         Version incremented

TEST: append non-existent document
  Given: No such shard
  When:  SELECT * FROM append_knowledge_doc('missing', 'content', 'summary', 'agent', 'test-project')
  Then:  RAISES EXCEPTION 'not found'

TEST: append rejects closed document
  Given: Knowledge doc with status='closed'
  When:  SELECT * FROM append_knowledge_doc('test-doc', 'content', 'summary', 'agent', 'test-project')
  Then:  RAISES EXCEPTION 'Knowledge document test-doc is closed'
```

### SQL Tests: knowledge_history

```
TEST: history of new document
  Given: Knowledge doc v1, never updated
  When:  SELECT * FROM knowledge_history('test-doc', 'test-project')
  Then:  Returns 1 row: version=1, change_summary='Initial document'

TEST: history after updates
  Given: Knowledge doc updated twice (now v3)
         v2 update summary was "Updated topology", v3 update summary was "Added diagrams"
  When:  SELECT * FROM knowledge_history('test-doc', 'test-project')
  Then:  Returns 3 rows in order: v3, v2, v1
         v3 has change_summary='Added diagrams' (from current shard metadata)
         v2 has change_summary='Updated topology' (from v2-snapshot shard metadata)
         v1 has change_summary='Initial document' (from v1-snapshot shard metadata)

TEST: history includes changed_by from metadata
  Given: v1 created by agent-a, v2 updated by agent-b, v3 updated by agent-a
  When:  SELECT * FROM knowledge_history('test-doc', 'test-project')
  Then:  changed_by values are: agent-a (v3 from last_changed_by), agent-b (v2 from edge), agent-a (v1 from edge)

TEST: history with deleted snapshot shard
  Given: Knowledge doc at v3, but snapshot 'test-doc-v2' manually deleted
  When:  SELECT * FROM knowledge_history('test-doc', 'test-project')
  Then:  Returns v3 and v1 only (v2 skipped due to missing JOIN target)

TEST: history depth cap
  Given: Knowledge doc with 1000+ versions (hypothetical)
  When:  SELECT * FROM knowledge_history(...)
  Then:  Returns at most 1000 rows (depth cap prevents infinite recursion)
```

### SQL Tests: knowledge_version

```
TEST: get current version
  Given: Knowledge doc at v3
  When:  SELECT * FROM knowledge_version('test-doc', 3, 'test-project')
  Then:  Returns the current shard's content

TEST: get historical version
  Given: Knowledge doc at v3
  When:  SELECT * FROM knowledge_version('test-doc', 1, 'test-project')
  Then:  Returns content from 'test-doc-v1' snapshot shard

TEST: version out of range (too high)
  Given: Knowledge doc at v3
  When:  SELECT * FROM knowledge_version('test-doc', 5, 'test-project')
  Then:  RAISES EXCEPTION 'Version 5 not found. Document has 3 versions.'

TEST: version out of range (zero)
  Given: Knowledge doc at v3
  When:  SELECT * FROM knowledge_version('test-doc', 0, 'test-project')
  Then:  RAISES EXCEPTION 'Version 0 not found.'

TEST: version with missing snapshot
  Given: Knowledge doc at v3, snapshot 'test-doc-v1' deleted
  When:  SELECT * FROM knowledge_version('test-doc', 1, 'test-project')
  Then:  RAISES EXCEPTION 'Version 1 snapshot not found (shard test-doc-v1 may have been deleted)'

TEST: non-existent document
  Given: No such shard
  When:  SELECT * FROM knowledge_version('missing', 1, 'test-project')
  Then:  RAISES EXCEPTION 'Knowledge document missing not found'
```

### Go Unit Tests

```
TEST: formatDiff produces unified diff
  Given: Old content "Line 1\nLine 2\nLine 3"
         New content "Line 1\nLine 2 modified\nLine 3\nLine 4"
  When:  formatDiff(old, new, "doc v1", "doc v2")
  Then:  Output is valid unified diff with - and + lines

TEST: parseDocType validates known types
  Given: --doc-type "architecture"
  When:  parseDocType("architecture")
  Then:  Returns "architecture", nil

TEST: parseDocType rejects unknown type
  Given: --doc-type "nonsense"
  When:  parseDocType("nonsense")
  Then:  Returns error: "Invalid doc_type 'nonsense'. Valid types: architecture, vision, roadmap, decision, reference"

TEST: formatHistory table
  Given: 3 VersionEntry items
  When:  formatHistory(entries, "text")
  Then:  Aligned table with VERSION, DATE, CHANGED BY, SUMMARY columns

TEST: formatHistory JSON
  Given: 3 VersionEntry items
  When:  formatHistory(entries, "json")
  Then:  Valid JSON array with all fields populated

TEST: diff version swap
  Given: --from 5, --to 2
  When:  normalizeDiffVersions(from=5, to=2)
  Then:  Returns from=2, to=5

TEST: diff default versions
  Given: Document at version 4, no --from or --to flags
  When:  normalizeDiffVersions(from=0, to=0, currentVersion=4)
  Then:  Returns from=3, to=4

TEST: diff on v1 document
  Given: Document at version 1, no --from or --to flags
  When:  normalizeDiffVersions(from=0, to=0, currentVersion=1)
  Then:  Returns error: "Document has only 1 version. Nothing to diff."
```

### Integration Tests

```
TEST: create knowledge document
  When:  `cp knowledge create "Test Arch" --doc-type architecture --body "# Architecture"`
  Then:  Exit code 0, returns shard ID
         `cp knowledge show <id>` shows content, doc_type=architecture, version=1

TEST: create with body-file
  Given: File /tmp/test-doc.md with "# Test Content"
  When:  `cp knowledge create "Test Doc" --doc-type reference --body-file /tmp/test-doc.md`
  Then:  Exit code 0
         `cp knowledge show <id>` shows content "# Test Content"

TEST: create with invalid doc-type
  When:  `cp knowledge create "Test" --doc-type bogus --body "content"`
  Then:  Exit code 1, error mentions valid types

TEST: create without body
  When:  `cp knowledge create "Test" --doc-type architecture`
  Then:  Exit code 1, error: "Either --body or --body-file is required."

TEST: list knowledge documents
  Given: 2 knowledge documents created
  When:  `cp knowledge list`
  Then:  Table with both documents showing doc_type, version, updated date

TEST: list with doc-type filter
  Given: 1 architecture doc, 1 vision doc
  When:  `cp knowledge list --doc-type architecture`
  Then:  Only the architecture doc shown

TEST: list with no documents
  Given: No knowledge documents exist
  When:  `cp knowledge list`
  Then:  "No knowledge documents found."

TEST: show displays full content and metadata
  Given: Knowledge doc with labels and content
  When:  `cp knowledge show <id>`
  Then:  Displays title, doc_type, version, labels, full content

TEST: update preserves version
  Given: Knowledge doc with content "V1 content"
  When:  `cp knowledge update <id> --body "V2 content" --summary "Updated"`
  Then:  `cp knowledge show <id>` shows "V2 content", version=2
         `cp knowledge show <id> --version 1` shows "V1 content"

TEST: update without summary
  Given: Knowledge doc exists
  When:  `cp knowledge update <id> --body "New content"`
  Then:  Exit code 1, error: "--summary required"

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

TEST: append without summary
  Given: Knowledge doc exists
  When:  `cp knowledge append <id> --body "More content"`
  Then:  Exit code 1, error: "--summary required"

TEST: diff between versions
  Given: Knowledge doc v1="Hello", v2="Hello World"
  When:  `cp knowledge diff <id> --from 1 --to 2`
  Then:  Output shows unified diff with changes

TEST: diff defaults to current vs previous
  Given: Knowledge doc at v3
  When:  `cp knowledge diff <id>`
  Then:  Shows diff between v3 and v2

TEST: diff on v1 document
  Given: Knowledge doc never updated (v1)
  When:  `cp knowledge diff <id>`
  Then:  Exit code 1, error: "Document has only 1 version."

TEST: diff with from > to swaps
  Given: Knowledge doc at v3
  When:  `cp knowledge diff <id> --from 3 --to 1`
  Then:  Shows diff from v1 to v3 (swapped silently)

TEST: history shows all versions
  Given: Knowledge doc updated twice
  When:  `cp knowledge history <id>`
  Then:  3 rows showing version, date, author, summary for each

TEST: both agents can update
  Given: Knowledge doc created by agent-penfold
  When:  Update by agent-mycroft, then update by agent-penfold
  Then:  `cp knowledge history <id>` shows correct changed_by for each version

TEST: update identical content rejected
  Given: Knowledge doc with content "Same"
  When:  `cp knowledge update <id> --body "Same" --summary "No change"`
  Then:  Exit code 1, error: "Content is identical"

TEST: show non-existent version
  Given: Knowledge doc at v2
  When:  `cp knowledge show <id> --version 5`
  Then:  Exit code 1, error: "Version 5 not found. Document has 2 versions."

TEST: update closed document rejected
  Given: Knowledge doc exists, closed via `cp shard close <id>`
  When:  `cp knowledge update <id> --body "New" --summary "Change"`
  Then:  Exit code 1, error: "Knowledge document <id> is closed. Reopen with cp shard reopen before updating."

TEST: append closed document rejected
  Given: Knowledge doc exists, closed via `cp shard close <id>`
  When:  `cp knowledge append <id> --body "More" --summary "Add"`
  Then:  Exit code 1, error: "Knowledge document <id> is closed."

TEST: show closed document works
  Given: Knowledge doc exists, closed via `cp shard close <id>`
  When:  `cp knowledge show <id>`
  Then:  Exit code 0, shows content with "(closed)" indicator

TEST: JSON output for all commands
  Given: Knowledge doc exists with history
  When:  `cp knowledge list -o json`
  Then:  Valid JSON array matching schema
  When:  `cp knowledge show <id> -o json`
  Then:  Valid JSON object matching schema
  When:  `cp knowledge history <id> -o json`
  Then:  Valid JSON array matching schema
  When:  `cp knowledge diff <id> -o json`
  Then:  Valid JSON object with from_version, to_version, diff fields
```

---

## Pre-Submission Checklist

- [x] Every item in "What to Build" has: CLI section + SQL + success criterion + tests
- [x] Every data flow answers all 7 questions (who writes/when/where/who reads/how/what for/staleness)
- [x] Every command has: syntax + example + output + atomic steps + JSON schema
- [x] Every workflow has: flowchart + all branches + error recovery + non-interactive mode
- [x] Every success criterion has at least one test case
- [x] Concurrency is addressed (FOR UPDATE locking, race condition prevention)
- [x] No feature is "mentioned but not specced" (no TODO, TBD, or vague "handles/manages/tracks")
- [x] Edge cases cover: invalid input, empty state, conflicts, boundaries, cross-feature, failure recovery
- [x] Existing spec interactions documented (Cross-Spec Interactions table)
- [x] Sub-agent review completed (16 items found, 3 medium fixed: history change_summary, agent identity, closed doc behavior)
