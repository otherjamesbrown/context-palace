# SPEC-5: Unified Search & Graph CLI

**Status:** Draft
**Depends on:** SPEC-1 (semantic search), SPEC-2 (metadata column)
**Blocks:** SPEC-6 (hierarchical memory extends `cp memory`)

---

## Goal

Bring everything together. General shard operations (`cp shard`), semantic search
across all types (`cp recall`), edge navigation, label management, and enhanced agent
memory with semantic recall. This is the CLI layer that makes the Context Palace graph
navigable and useful for day-to-day agent and developer work.

## What Exists

- `palace task get/claim/progress/close` — task-only operations (5 commands)
- `penf memory add/list/search/resolve/defer` — memory-only, text search
- `penf backlog add/list/show/update/close` — backlog-only
- `penf message send/inbox/show/read` — messaging
- `penf session start/checkpoint/show/end` — sessions
- `penf context status/history/morning/project` — project context
- No general shard browser, no edge navigation, no label management, no semantic search
- `edges` table with columns: `from_id`, `to_id`, `edge_type`, `metadata JSONB`, `created_at`
- No unique constraint on edges (duplicates possible)
- `labels TEXT[]` column on shards with GIN index (SPEC-0)
- `embedding vector(768)` column on shards (SPEC-1)

## What to Build

1. **`cp shard` commands** — general CRUD for any shard type, edge browsing, label ops
2. **`cp recall`** — semantic search across all shard types (signature command)
3. **Enhanced `cp memory`** — labels, edges to related shards, semantic recall
4. **Edge navigation** — show linked shards, create/remove edges, tree view
5. **Label management** — add/remove labels atomically, query by label, label summary
6. **Edge type registry** — canonical list of valid edge types with semantics

## Data Model

### Schema Changes

```sql
-- Add unique constraint on edges to prevent duplicates
ALTER TABLE edges ADD CONSTRAINT edges_unique_triple
    UNIQUE (from_id, to_id, edge_type);
```

No other schema changes. SPEC-5 builds on:
- SPEC-1: `embedding` column + `semantic_search()` function
- SPEC-2: `metadata` column + `update_metadata()` function
- Existing: `labels`, `edges`, `shards` tables

**Pre-migration note:** If duplicate edges exist before applying `edges_unique_triple`, the
`ALTER TABLE` will fail. Run deduplication first:
```sql
DELETE FROM edges WHERE ctid NOT IN (
    SELECT min(ctid) FROM edges GROUP BY from_id, to_id, edge_type
);
```

### Storage Format

No new storage formats. This spec reads/writes data using formats defined in SPEC-0
(shards), SPEC-1 (embeddings), and SPEC-2 (metadata JSONB). Edge metadata format is
spec-dependent (SPEC-3 uses `lifecycle_trigger`, SPEC-4 uses `change_summary`, etc.).

### Used Indexes

```
idx_shards_type          — filter by shard type
idx_shards_status        — filter by status
idx_shards_labels        — GIN index for label overlap (&&)
idx_shards_embedding     — ivfflat for semantic similarity
idx_shards_metadata      — GIN index for metadata queries
idx_shards_project       — filter by project
idx_edges_from           — outgoing edges
idx_edges_to             — incoming edges
idx_edges_type           — filter by edge type
edges_unique_triple      — UNIQUE constraint preventing duplicate edges
search_vector            — tsvector for keyword search
```

### Edge Type Registry

Canonical list of valid edge types. New types can only be added via spec amendments.

| Edge Type | Direction Semantics | Introduced By | Description |
|-----------|-------------------|---------------|-------------|
| `blocked-by` | A blocked-by B: A cannot start until B is done | SPEC-0 | Dependency between shards |
| `blocks` | A blocks B: B cannot start until A is done | SPEC-0 | Inverse of blocked-by |
| `child-of` | A child-of B: A is a sub-item of B | SPEC-6 | Hierarchical parent-child |
| `discovered-from` | A discovered-from B: A was found during B | SPEC-0 | Provenance tracking |
| `extends` | A extends B: A builds on B | SPEC-0 | Extension relationship |
| `has-artifact` | A has-artifact B: B is an artifact of A | SPEC-3 | Test/artifact attachment |
| `implements` | A implements B: A is work toward B | SPEC-3 | Task-to-requirement link |
| `parent` | A parent B: A contains B | SPEC-0 | Inverse of child-of |
| `previous-version` | A previous-version B: B is A's prior version | SPEC-4 | Version chain link |
| `references` | A references B: A mentions or relates to B | SPEC-0 | General cross-reference |
| `relates-to` | A relates-to B: bidirectional association | SPEC-0 | Loose association |
| `replies-to` | A replies-to B: A is a response to B | SPEC-0 | Message threading |
| `triggered-by` | A triggered-by B: B caused A to be created | SPEC-0 | Causal chain |

### Data Flow

#### Shard embedding (write path)

1. **WHO writes it?** Go CLI code after `create_shard()` or `cp shard update` succeeds
2. **WHEN is it written?** Immediately after shard create/update, as a separate SQL UPDATE
3. **WHERE is it stored?** `shards.embedding` column (vector(768))
4. **WHO reads it?** `semantic_search()` (SPEC-1), `memory_recall()` (this spec)
5. **HOW is it queried?** `1 - (embedding <=> query_embedding)` cosine distance via ivfflat index
6. **WHAT decisions does it inform?** Search result ranking, memory recall relevance
7. **DOES it go stale?** Yes — if content is updated without re-embedding. The Go code must regenerate after every content update. If embedding fails, the shard is created/updated but embedding stays NULL/stale. Stale embedding only affects search ranking, not data integrity.

#### Edge creation

1. **WHO writes it?** `cp shard link`, `cp requirement link` (SPEC-3), `cp knowledge update` (SPEC-4, for previous-version), `cp memory add --references`, `cp memory add-sub` (SPEC-6, for child-of)
2. **WHEN is it written?** On explicit user/agent command
3. **WHERE is it stored?** `edges` table: `from_id`, `to_id`, `edge_type`, `metadata JSONB`
4. **WHO reads it?** `cp shard edges`, `cp shard show` (edge summary), `shard_edges()`, `has_circular_dependency()` (SPEC-3), `knowledge_history()` (SPEC-4), `memory_tree()` (SPEC-6)
5. **HOW is it queried?** `shard_edges()` for display, direct `SELECT` for existence checks
6. **WHAT decisions does it inform?** Graph navigation, dependency tracking, lifecycle triggers (SPEC-3 auto-transitions), version history (SPEC-4)
7. **DOES it go stale?** No — edges are explicit facts. They can become orphaned if linked shards are deleted.

#### Label array

1. **WHO writes it?** `cp shard create --label`, `cp shard label add/remove`, `cp memory add --label`
2. **WHEN is it written?** On create (initial labels) or explicit add/remove command
3. **WHERE is it stored?** `shards.labels TEXT[]` column with GIN index
4. **WHO reads it?** `cp shard list --label`, `cp shard show`, `cp shard labels`, `list_shards()`, `label_summary()`, `semantic_search()` (SPEC-1 label filter)
5. **HOW is it queried?** `labels && ARRAY[...]` for overlap (GIN-indexed), `unnest(labels)` for summary
6. **WHAT decisions does it inform?** Filtering, categorization, discovery
7. **DOES it go stale?** No — labels are explicit. Unused labels disappear from `label_summary()` naturally when no shards reference them.

### Concurrency

**Edge creation race condition:** Two agents creating the same edge simultaneously could
result in a duplicate. The UNIQUE constraint `edges_unique_triple` on `(from_id, to_id, edge_type)`
prevents this. Use `INSERT ... ON CONFLICT DO NOTHING` and check the affected row count.

```sql
INSERT INTO edges (from_id, to_id, edge_type, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT (from_id, to_id, edge_type) DO NOTHING;
-- If 0 rows affected, edge already existed
```

**Label modification race condition:** Two agents adding/removing labels concurrently.
Use atomic SQL array operations — `array_append`, `array_remove`, `array_cat` — in a
single UPDATE statement. No read-modify-write pattern.

```sql
-- Add labels atomically
UPDATE shards
SET labels = (
    SELECT ARRAY(SELECT DISTINCT unnest(COALESCE(labels, '{}') || $2))
)
WHERE id = $1;

-- Remove labels atomically
UPDATE shards
SET labels = array_remove(labels, $2)  -- for single label
WHERE id = $1;
```

**Shard content updates:** Last-write-wins via PostgreSQL MVCC. No locking needed for
`cp shard update` — it's a simple UPDATE. Knowledge docs have their own locking via
SPEC-4's `FOR UPDATE`.

**Circular dependency check:** The `has_circular_dependency()` function (SPEC-3) reads
the edge graph at the current snapshot. A concurrent edge insert could create a cycle
between the check and the insert. The UNIQUE constraint prevents exact duplicates, but
two concurrent "completing the cycle" inserts could both pass the check. This is an
accepted risk — the check is advisory, not transactional. The probability is very low
in practice (requires exact simultaneous inserts completing a specific cycle).

## CLI Surface

### `cp recall` — Semantic Search

The signature command. Single semantic search across everything in the project.

```bash
# Basic semantic search
cp recall "pipeline timeout issues"

# Filter by type (comma-separated)
cp recall "entity resolution" --type requirement,bug

# Filter by label
cp recall "deployment" --label architecture

# Filter by status (default: open only)
cp recall "timeout" --status open
cp recall "old decisions" --include-closed

# Adjust similarity threshold (default 0.3)
cp recall "vague query" --min-similarity 0.5

# Since filter (content created after date)
cp recall "deployment" --since 7d
cp recall "architecture" --since 2026-01-01

# Limit results (default 20)
cp recall "deployment" --limit 5

# JSON output for programmatic use
cp recall "entity management" -o json --limit 5

# Show content snippets (default: title only)
cp recall "entity" --show-snippet
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--type` | No | all types | Comma-separated shard types to include |
| `--label` | No | — | Comma-separated labels (OR/overlap) |
| `--status` | No | open | Comma-separated statuses. Mutually exclusive with `--include-closed` |
| `--include-closed` | No | false | Include all statuses. Mutually exclusive with `--status` |
| `--min-similarity` | No | 0.3 | Minimum cosine similarity (0.0-1.0) |
| `--since` | No | — | Time filter: duration (7d, 24h, 2w, 30m) or ISO date (2026-01-01). Duration suffixes: `d`=days, `h`=hours, `w`=weeks, `m`=minutes (not months). |
| `--limit` | No | 20 | Max results (1-1000) |
| `--show-snippet` | No | false | Show content preview under each result |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Validate flags. Error if `--status` and `--include-closed` both set. Error if `--min-similarity` outside 0.0-1.0. Error if `--limit` outside 1-1000.
2. Parse `--since`: if matches `\d+[dhwm]`, convert to Go `time.Duration`. If matches `\d{4}-\d{2}-\d{2}`, parse as date and compute `time.Since(date)`. Convert to `TIMESTAMPTZ` cutoff.
3. Embed the query text using configured embedding provider (SPEC-1). Error if no provider configured: "Semantic search requires embedding config. Use `cp shard list --search` for text search."
4. Call `semantic_search()` (SPEC-1) with all filters including `--since` as `p_since`.
5. Format results as table or JSON.

**Output (text):**
```
SIMILARITY  TYPE         STATUS  ID          TITLE
0.94        bug          open    pf-c74eea   Timeout not applied after deploy
0.91        requirement  draft   pf-req-04   Structured Error Codes
0.88        task         closed  pf-3acaf1   Wire timeout config into worker
0.85        memory       open    pf-mem-12   Lesson: AI client vs heartbeat timeout
0.78        knowledge    open    pf-arch-01  System Architecture

5 results (min similarity: 0.30)
```

With `--show-snippet`:
```
SIMILARITY  TYPE         ID          TITLE
0.91        requirement  pf-req-01   Entity Lifecycle Management
  "Let me reject junk entities and manage filter rules. Currently..."
0.85        bug          pf-bug-03   Entity extraction missing display names
  "77 of 93 entities have no display name. The extraction pipeline..."
```

**JSON output (`-o json`):**
```json
[
  {
    "id": "pf-c74eea",
    "title": "Timeout not applied after deploy",
    "type": "bug",
    "status": "open",
    "similarity": 0.94,
    "labels": ["pipeline", "timeout"],
    "created_at": "2026-02-06T21:30:00Z",
    "snippet": "The AI client timeout was hardcoded at 120s..."
  }
]
```

---

### `cp shard list` — List shards with filters

```bash
cp shard list
cp shard list --type task --status open
cp shard list --type requirement,bug --status open --since 7d
cp shard list --creator agent-mycroft --limit 50
cp shard list --label architecture
cp shard list --search "timeout"    # text search (tsvector, not semantic)
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--type` | No | all types | Comma-separated shard types |
| `--status` | No | all statuses | Comma-separated statuses. Note: unlike `cp recall`, no default status filter |
| `--label` | No | — | Comma-separated labels (OR/overlap) |
| `--creator` | No | — | Filter by creator |
| `--search` | No | — | Text search (tsvector full-text search, not semantic) |
| `--since` | No | — | Time filter: duration or date |
| `--limit` | No | 20 | Max results (1-1000) |
| `--offset` | No | 0 | Skip N results for pagination |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Parse and validate all filter flags.
2. Convert `--since` to `TIMESTAMPTZ` cutoff (same as `cp recall`).
3. Call `list_shards()` SQL function with all filters.
4. Call `list_shards_count()` for total count (for pagination display).
5. Format results as table or JSON.

**Output (text):**
```
ID          TYPE         STATUS  CREATED      TITLE
pf-c74eea   bug          open    2026-02-06   Fixes STILL not working
pf-req-01   requirement  open    2026-02-07   Entity Lifecycle Management
pf-3acaf1   task         closed  2026-02-05   Wire timeout config

Showing 1-3 of 3 results
```

**JSON output (`-o json`):**
```json
{
  "total": 3,
  "offset": 0,
  "limit": 20,
  "results": [
    {
      "id": "pf-c74eea",
      "title": "Fixes STILL not working",
      "type": "bug",
      "status": "open",
      "creator": "agent-penfold",
      "labels": ["pipeline", "timeout"],
      "created_at": "2026-02-06T21:30:00Z",
      "updated_at": "2026-02-06T21:30:00Z",
      "snippet": "The root cause was found..."
    }
  ]
}
```

**Design note:** `cp shard list` defaults to ALL statuses (no filter), unlike `cp recall`
which defaults to `open` only. Rationale: `shard list` is a general browser — users
browsing shards may want to see everything. `recall` is a search tool — closed shards
are typically less relevant for active search.

---

### `cp shard show` — Show shard detail

```bash
cp shard show pf-c74eea
cp shard show pf-c74eea -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Call `shard_detail(shard_id)` to get shard fields and edge counts.
2. If no row returned, error: "Shard pf-xxx not found."
3. Call `shard_edges(shard_id)` to get edge list.
4. Format output with all sections.

**Output (text):**
```
Fixes STILL not working - root cause found
──────────────────────────────────────────
ID:       pf-c74eea
Type:     bug
Status:   open
Creator:  agent-penfold
Created:  2026-02-06 21:30
Labels:   pipeline, timeout

Metadata:
  root_cause: "AI client timeout hardcoded at 120s"
  affects_requirement: pf-req-04

Content:
  [full shard content]

Edges:
  DIRECTION  EDGE TYPE       SHARD          TYPE    TITLE
  outgoing   references      pf-3acaf1      task    Wire timeout config
  incoming   discovered-from pf-session-01  session Session: 2026-02-06
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-c74eea",
  "title": "Fixes STILL not working",
  "content": "[full content]",
  "type": "bug",
  "status": "open",
  "creator": "agent-penfold",
  "labels": ["pipeline", "timeout"],
  "metadata": {"root_cause": "AI client timeout hardcoded at 120s"},
  "created_at": "2026-02-06T21:30:00Z",
  "updated_at": "2026-02-06T21:30:00Z",
  "edges": [
    {
      "direction": "outgoing",
      "edge_type": "references",
      "shard_id": "pf-3acaf1",
      "title": "Wire timeout config",
      "type": "task",
      "status": "closed"
    }
  ]
}
```

---

### `cp shard create` — Create a shard

```bash
# With inline body
cp shard create --type design \
    --title "Entity filter architecture" \
    --body "## Design\n..." \
    --label architecture,entity \
    --meta '{"scope": "component"}'

# With file body
cp shard create --type design \
    --title "Entity filter architecture" \
    --body-file design.md \
    --label architecture,entity
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--type` | Yes | — | Shard type (task, bug, memory, design, etc.) |
| `--title` | Yes | — | Shard title |
| `--body` | No | — | Inline content. Mutually exclusive with `--body-file` |
| `--body-file` | No | — | Content from file. Mutually exclusive with `--body` |
| `--label` | No | — | Comma-separated labels |
| `--meta` | No | — | JSON metadata string |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Validate `--type` is provided. Error if missing.
2. Validate `--title` is provided. Error if missing.
3. If unknown type, warn: "Type 'foo' is not a known type. Create anyway? (y/n)." Allow custom types.
4. If `--body-file`, read file. Error if file not found.
5. If both `--body` and `--body-file`, error: "Cannot use both --body and --body-file."
6. Parse `--meta` as JSON. Error if invalid JSON.
7. Call `create_shard()` (SPEC-0) with type, title, content, metadata, labels.
8. If embedding configured (SPEC-1), generate embedding for `"<type>: <title>\n\n<content>"`. On failure, log warning and continue.
9. Print shard ID and confirmation.

**Output (text):**
```
Created shard pf-abc123 (design)
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-abc123",
  "type": "design",
  "title": "Entity filter architecture",
  "created_at": "2026-02-07T12:00:00Z"
}
```

---

### `cp shard update` — Update shard content or title

```bash
# Update content from inline
cp shard update pf-abc123 --body "Updated content"

# Update content from file
cp shard update pf-abc123 --body-file updated.md

# Update title
cp shard update pf-abc123 --title "New Title"

# Update both
cp shard update pf-abc123 --title "New Title" --body "New content"
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--body` | No | — | New content (inline). Mutually exclusive with `--body-file` |
| `--body-file` | No | — | New content (from file) |
| `--title` | No | — | New title |
| `-o` | No | text | Output format: text, json |

At least one of `--body`, `--body-file`, or `--title` is required.

**What it does (atomic):**
1. Validate at least one update field provided. Error if none.
2. If `--body-file`, read file. Error if not found.
3. If both `--body` and `--body-file`, error.
4. Fetch current shard to verify it exists. Error if not found.
5. If shard `type = 'knowledge'`, print warning: "This is a knowledge document. Use `cp knowledge update` to preserve version history. Updating directly."
6. Update shard content and/or title.
7. If content changed and embedding configured (SPEC-1), regenerate embedding. On failure, log warning.
8. Print confirmation.

**Note:** Metadata updates go through `cp shard metadata set` (SPEC-2). This command
only handles content and title.

**Output (text):**
```
Updated pf-abc123
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-abc123",
  "updated_fields": ["content", "title"],
  "updated_at": "2026-02-07T12:00:00Z"
}
```

---

### `cp shard close` / `cp shard reopen` — Status management

```bash
cp shard close pf-abc123
cp shard reopen pf-abc123
cp shard close pf-abc123 -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-o` | No | text | Output format: text, json |

**What `close` does (atomic):**
1. Verify shard exists. Error if not: "Shard pf-abc123 not found."
2. If already closed, no-op with message: "Shard pf-abc123 is already closed." (exit code 0)
3. `UPDATE shards SET status = 'closed', updated_at = now() WHERE id = $1 AND project = $2`
4. Print confirmation.

**What `reopen` does (atomic):**
1. Verify shard exists. Error if not: "Shard pf-abc123 not found."
2. If already open, no-op with message: "Shard pf-abc123 is already open." (exit code 0)
3. `UPDATE shards SET status = 'open', updated_at = now() WHERE id = $1 AND project = $2`
4. Print confirmation.

**Output (text):**
```
Closed pf-abc123
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-abc123",
  "status": "closed",
  "updated_at": "2026-02-07T12:00:00Z"
}
```

---

### `cp shard edges` — Edge Navigation

```bash
# Show all edges for a shard
cp shard edges pf-abc123

# Filter by direction
cp shard edges pf-abc123 --direction outgoing

# Filter by edge type
cp shard edges pf-abc123 --edge-type implements,references

# Follow edges (tree view, default 2 hops)
cp shard edges pf-req-01 --follow
cp shard edges pf-req-01 --follow --max-depth 3

# JSON output
cp shard edges pf-abc123 -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--direction` | No | both | outgoing, incoming, or both |
| `--edge-type` | No | all | Comma-separated edge types |
| `--follow` | No | false | Tree view showing 2-hop graph |
| `--max-depth` | No | 2 | Max hops for --follow (1-5) |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Verify shard exists.
2. Call `shard_edges(shard_id, direction, edge_types)`.
3. If `--follow`, recursively fetch edges for each linked shard up to `--max-depth`. Track visited shard IDs to prevent cycles.
4. Format as table, tree, or JSON.

**Output (table mode):**
```
DIRECTION  EDGE TYPE        SHARD          TYPE         STATUS  TITLE
outgoing   implements       pf-req-01      requirement  open    Entity Lifecycle Mgmt
outgoing   references       pf-bug-03      bug          open    Missing display names
incoming   blocked-by       pf-req-05      requirement  open    Reprocessing Overrides
```

**Output (follow/tree mode):**
```
pf-req-01 "Entity Lifecycle Management"
├── implements
│   ├── pf-task-001 "Add reject endpoint" (closed)
│   ├── pf-task-002 "Add bulk reject" (open)
│   └── pf-task-003 "Add filter rules table" (open)
├── has-artifact
│   └── pf-test-001 "Entity reject tests" (open)
├── blocked-by (incoming)
│   └── pf-req-05 "Reprocessing Overrides" (draft)
│       └── (cycle: pf-req-01 already shown)
└── references (incoming)
    └── pf-bug-03 "Missing display names" (open)
```

**JSON output (`-o json`):**
```json
[
  {
    "direction": "outgoing",
    "edge_type": "implements",
    "shard_id": "pf-req-01",
    "title": "Entity Lifecycle Mgmt",
    "type": "requirement",
    "status": "open",
    "edge_metadata": null
  }
]
```

---

### `cp shard link` / `cp shard unlink` — Edge Management

```bash
# Create edges (flag name = edge type)
cp shard link pf-task-123 --implements pf-req-01
cp shard link pf-bug-03 --references pf-task-456
cp shard link pf-req-05 --blocked-by pf-req-03

# Remove edges
cp shard unlink pf-task-123 --implements pf-req-01

# Force unlink (skip confirmation)
cp shard unlink pf-task-123 --implements pf-req-01 --force
```

**What `link` does (atomic):**
1. Parse edge type and target from flags. Exactly one edge-type flag required.
2. Validate edge type is in the registry. Error if unknown.
3. Verify both shards exist. Error if either not found.
4. Verify not self-referencing. Error: "Cannot create edge from a shard to itself."
5. For `blocked-by` edges, call `has_circular_dependency()` (SPEC-3). Error if circular.
6. Insert edge with `ON CONFLICT DO NOTHING`.
7. If 0 rows affected, error: "Edge already exists: pf-a --implements--> pf-b."
8. Print confirmation.

**Note on SPEC-3 lifecycle interaction:** `cp shard link --implements` creates the edge
but does NOT trigger SPEC-3's automatic lifecycle transition (approved → in_progress).
That transition only happens via `cp requirement link --task`, which is the lifecycle-aware
command. `cp shard link` is the low-level graph tool. If lifecycle automation is desired,
use `cp requirement link --task` instead.

**What `unlink` does (atomic):**
1. Parse edge type and target from flags.
2. If `--force` not set, prompt: "Remove implements edge from pf-task-123 to pf-req-01? (y/n)"
3. If piped/non-interactive and no `--force`, error: "Use --force for non-interactive unlink."
4. Delete edge. If 0 rows affected, error: "No edge of type 'implements' from pf-a to pf-b."
5. Print confirmation.

---

### `cp shard label` — Label Management

```bash
# Add labels to a shard
cp shard label add pf-abc123 architecture pipeline

# Remove labels
cp shard label remove pf-abc123 pipeline

# List all labels in use (discovery)
cp shard label list
# Output:
#   LABEL           COUNT
#   architecture    12
#   pipeline        8
#   deployment      6

# JSON output
cp shard label list -o json
```

**What `add` does (atomic):**
1. Verify shard exists.
2. Call `add_shard_labels(shard_id, label_array)` — atomic SQL, no read-modify-write.
3. Print updated label list.

**What `remove` does (atomic):**
1. Verify shard exists.
2. Call `remove_shard_labels(shard_id, label_array)` — atomic SQL.
3. Print updated label list. Removing a label that doesn't exist is a no-op.

**What `list` does:**
1. Call `label_summary(project)`.
2. Format as table or JSON.

**JSON output (`-o json` for `label list`):**
```json
[
  {"label": "architecture", "count": 12},
  {"label": "pipeline", "count": 8}
]
```

**Naming note:** The subcommand is `cp shard label` (singular) with verbs `add`, `remove`,
`list`. This is consistent with the `cp shard link`/`unlink` pattern and avoids confusion
between singular and plural forms.

---

### Enhanced `cp memory`

Memory commands migrated from `penf` and enhanced with labels, links, and semantic recall.

```bash
# Add memory (basic — same as penf)
cp memory add "Nomad deploys are unreliable — always verify with version check"

# Add memory with labels
cp memory add "AI client timeout was hardcoded at 120s" \
    --label timeout,pipeline,lesson-learned

# Add memory with edge links
cp memory add "Entity display names missing because NER stage doesn't extract them" \
    --label entity,pipeline \
    --references pf-bug-03,pf-req-01

# List memories
cp memory list
cp memory list --label lesson-learned
cp memory list --since 7d

# Text search (same as penf)
cp memory search "timeout"

# Semantic recall (NEW — uses embedding)
cp memory recall "deployment issues"

# Resolve/defer (same as penf)
cp memory resolve pf-mem-12
cp memory defer pf-mem-08 --until 2026-02-14
```

#### `cp memory add` — Add a memory

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--label` | No | — | Labels (comma-separated or repeatable) |
| `--references` | No | — | Shard IDs to create `references` edges to (comma-separated) |
| `-o` | No | text | Output format: text, json |

**What `add` does (atomic):**
1. Create shard with `type='memory'`, content = the text argument.
2. If `--label`, set labels on create.
3. If `--references`, for each referenced shard ID: verify it exists, create `references` edge. If a referenced shard doesn't exist, warn but continue — memory is still created.
4. If embedding configured, generate embedding.
5. Print memory ID.

#### `cp memory list` — List memories

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--label` | No | — | Filter by label (comma-separated, OR match) |
| `--since` | No | — | Time filter: duration (7d, 24h) or ISO date |
| `--status` | No | open | Filter by status |
| `--roots` | No | false | Only show root memories (parent_id IS NULL). See SPEC-6 for hierarchy. |
| `--limit` | No | 20 | Max results |
| `-o` | No | text | Output format: text, json |

**What `list` does (atomic):**
1. Call `list_shards(project, types=['memory'], status, labels, NULL, NULL, since, limit)`.
   If `--roots`, add filter: `parent_id IS NULL`.
2. Format as table or JSON.

**Default behavior:** Shows all memories (root and child) in flat list. Use `--roots` to
show only root-level memories. See SPEC-6 `cp memory tree` for hierarchical views.

#### `cp memory recall` — Semantic search over memories

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--label` | No | — | Filter by label (comma-separated, OR match) |
| `--limit` | No | 10 | Max results |
| `--min-similarity` | No | 0.3 | Minimum cosine similarity (0.0-1.0) |
| `-o` | No | text | Output format: text, json |

**What `recall` does (atomic):**
1. Embed query using configured provider. Error if no provider: "Semantic recall requires embedding config. Use `cp memory search` for text search."
2. Call `memory_recall(project, embedding, labels, limit, min_similarity)`.
3. Format as table or JSON.

**JSON output (`-o json` for `memory recall`):**
```json
[
  {
    "id": "pf-mem-12",
    "content": "Lesson: AI client vs heartbeat timeout...",
    "similarity": 0.91,
    "labels": ["timeout", "pipeline", "lesson-learned"],
    "created_at": "2026-02-06T12:00:00Z"
  }
]
```

#### `cp memory search`, `resolve`, `defer` — Penf-compatible commands

These commands are identical to their `penf memory` equivalents:

- **`cp memory search <query>`** — Full-text search (tsvector) over memory shards. Same as `penf memory search`.
- **`cp memory resolve <id>`** — Close a memory shard (sets status=closed). Same as `penf memory resolve`.
- **`cp memory defer <id> --until <date>`** — Set a `deferred_until` metadata field. Same as `penf memory defer`.

No new flags or behavior. See `penf` documentation for full details.

**Relationship to `penf memory`:** `penf memory` retains its existing behavior (no labels,
no references, text search only). `cp memory` is the enhanced version with labels,
references, semantic recall. Both operate on the same `type='memory'` shards in the database.
Memory shards created via `cp shard create --type memory` are plain memory shards —
use `cp memory add` for the standard creation flow with labels/references, and
`cp memory add-sub` (SPEC-6) for hierarchical memories.

## Workflows

### `cp recall` Workflow

```
cp recall "query" [flags]
    │
    ▼
[1] Validate flags ──── --status + --include-closed ──▶ Error: "mutually exclusive"
    │                    --min-similarity > 1.0 ──▶ Error: "must be 0.0-1.0"
    │                    --limit < 1 or > 1000 ──▶ Error: "must be 1-1000"
    ▼
[2] Parse --since (if set)
    │   │
    │   └── bad format ──▶ Error: "Invalid duration. Use '7d', '24h', or '2026-01-01'."
    ▼
[3] Embed query via provider
    │   │
    │   └── no provider configured ──▶ Error: "Semantic search requires embedding config."
    │   └── provider error ──▶ Error: "Failed to embed query: <details>"
    ▼
[4] Call semantic_search() with all filters including since
    │
    ▼
[5] Format and print results

Non-interactive: All steps are non-interactive. No prompts.
```

### `cp shard link` Workflow

```
cp shard link <from> --<edge-type> <to>
    │
    ▼
[1] Parse edge type flag ──── no flag ──▶ Error: "Specify edge type flag"
    │                         multiple flags ──▶ Error: "Exactly one edge type"
    ▼
[2] Validate edge type ──── unknown ──▶ Error: "Unknown type. Valid: ..."
    │
    ▼
[3] Check self-reference ──── from == to ──▶ Error: "Cannot link to itself"
    │
    ▼
[4] Verify both shards exist
    │   │
    │   └── not found ──▶ Error: "Shard <id> not found"
    ▼
[5] If blocked-by: check circular dependency
    │   │
    │   └── circular ──▶ Error: "Circular dependency detected"
    ▼
[6] INSERT ... ON CONFLICT DO NOTHING
    │   │
    │   └── 0 rows ──▶ Error: "Edge already exists"
    ▼
[7] Print confirmation

Non-interactive: All steps are non-interactive.
```

### `cp shard create` Workflow

```
cp shard create --type <type> --title <title> [--body|--body-file] [flags]
    │
    ▼
[1] Validate --type ──── missing ──▶ Error: "--type is required"
    │                     unknown ──▶ Prompt: "Unknown type. Create anyway?"
    ▼
[2] Validate --title ──── missing ──▶ Error: "--title is required"
    │
    ▼
[3] Read content (--body or --body-file)
    │   │
    │   └── both set ──▶ Error: "Cannot use both"
    │   └── file not found ──▶ Error: "Cannot read file"
    ▼
[4] Parse --meta as JSON ──── invalid ──▶ Error: "Invalid JSON"
    │
    ▼
[5] Call create_shard()
    │
    ▼
[6] Generate embedding (async-safe)
    │   │
    │   └── failure ──▶ Log warning, continue
    ▼
[7] Print shard ID

Non-interactive (except unknown type prompt). For non-interactive mode,
unknown types are rejected without prompt.
```

## SQL Functions

### Filtered Shard List

```sql
-- General shard listing with all filters
-- Note: p_since uses TIMESTAMPTZ (not INTERVAL) for consistency with semantic_search().
-- Go code computes the cutoff time: time.Now().Add(-duration) and passes it directly.
CREATE OR REPLACE FUNCTION list_shards(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since TIMESTAMPTZ DEFAULT NULL,
    p_limit INT DEFAULT 20,
    p_offset INT DEFAULT 0
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    type TEXT,
    status TEXT,
    creator TEXT,
    labels TEXT[],
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    snippet TEXT
) AS $$
    SELECT
        s.id, s.title, s.type, s.status, s.creator,
        s.labels, s.created_at, s.updated_at,
        LEFT(s.content, 200) AS snippet
    FROM shards s
    WHERE s.project = p_project
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_labels IS NULL OR s.labels && p_labels)
      AND (p_creator IS NULL OR s.creator = p_creator)
      AND (p_search IS NULL OR s.search_vector @@ plainto_tsquery(p_search))
      AND (p_since IS NULL OR s.created_at >= p_since)
    ORDER BY s.created_at DESC
    LIMIT p_limit
    OFFSET p_offset;
$$ LANGUAGE sql STABLE;

-- Count total matching shards (for pagination display)
CREATE OR REPLACE FUNCTION list_shards_count(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since TIMESTAMPTZ DEFAULT NULL
) RETURNS INT AS $$
    SELECT count(*)::int
    FROM shards s
    WHERE s.project = p_project
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_labels IS NULL OR s.labels && p_labels)
      AND (p_creator IS NULL OR s.creator = p_creator)
      AND (p_search IS NULL OR s.search_vector @@ plainto_tsquery(p_search))
      AND (p_since IS NULL OR s.created_at >= p_since);
$$ LANGUAGE sql STABLE;
```

### Shard Detail with Edges

```sql
-- Get shard with all edges
CREATE OR REPLACE FUNCTION shard_detail(p_shard_id TEXT)
RETURNS TABLE (
    id TEXT,
    title TEXT,
    content TEXT,
    type TEXT,
    status TEXT,
    creator TEXT,
    labels TEXT[],
    metadata JSONB,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    outgoing_edge_count INT,
    incoming_edge_count INT
) AS $$
    SELECT
        s.id, s.title, s.content, s.type, s.status, s.creator,
        s.labels, s.metadata, s.created_at, s.updated_at,
        (SELECT count(*) FROM edges e WHERE e.from_id = s.id)::int,
        (SELECT count(*) FROM edges e WHERE e.to_id = s.id)::int
    FROM shards s
    WHERE s.id = p_shard_id;
$$ LANGUAGE sql STABLE;

-- Get edges for a shard (both directions)
CREATE OR REPLACE FUNCTION shard_edges(
    p_shard_id TEXT,
    p_direction TEXT DEFAULT NULL,    -- 'outgoing', 'incoming', or NULL for both
    p_edge_types TEXT[] DEFAULT NULL
) RETURNS TABLE (
    direction TEXT,
    edge_type TEXT,
    linked_shard_id TEXT,
    linked_shard_title TEXT,
    linked_shard_type TEXT,
    linked_shard_status TEXT,
    edge_metadata JSONB
) AS $$
    -- Outgoing edges
    SELECT
        'outgoing'::text,
        e.edge_type,
        e.to_id,
        s.title,
        s.type,
        s.status,
        e.metadata
    FROM edges e
    JOIN shards s ON s.id = e.to_id
    WHERE e.from_id = p_shard_id
      AND (p_direction IS NULL OR p_direction = 'outgoing')
      AND (p_edge_types IS NULL OR e.edge_type = ANY(p_edge_types))

    UNION ALL

    -- Incoming edges
    SELECT
        'incoming'::text,
        e.edge_type,
        e.from_id,
        s.title,
        s.type,
        s.status,
        e.metadata
    FROM edges e
    JOIN shards s ON s.id = e.from_id
    WHERE e.to_id = p_shard_id
      AND (p_direction IS NULL OR p_direction = 'incoming')
      AND (p_edge_types IS NULL OR e.edge_type = ANY(p_edge_types))

    ORDER BY edge_type, direction, e.created_at;
$$ LANGUAGE sql STABLE;
```

### Label Operations

```sql
-- Add labels to a shard atomically (deduplicates)
CREATE OR REPLACE FUNCTION add_shard_labels(
    p_shard_id TEXT,
    p_labels TEXT[]
) RETURNS TEXT[] AS $$
DECLARE
    result_labels TEXT[];
BEGIN
    UPDATE shards
    SET labels = (
        SELECT ARRAY(
            SELECT DISTINCT unnest(COALESCE(labels, '{}') || p_labels)
            ORDER BY 1
        )
    ),
    updated_at = now()
    WHERE id = p_shard_id
    RETURNING labels INTO result_labels;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result_labels;
END;
$$ LANGUAGE plpgsql;

-- Remove labels from a shard atomically
CREATE OR REPLACE FUNCTION remove_shard_labels(
    p_shard_id TEXT,
    p_labels TEXT[]
) RETURNS TEXT[] AS $$
DECLARE
    result_labels TEXT[];
    lbl TEXT;
BEGIN
    -- Remove each label
    UPDATE shards
    SET labels = (
        SELECT ARRAY(
            SELECT unnest(COALESCE(labels, '{}'))
            EXCEPT
            SELECT unnest(p_labels)
            ORDER BY 1
        )
    ),
    updated_at = now()
    WHERE id = p_shard_id
    RETURNING labels INTO result_labels;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result_labels;
END;
$$ LANGUAGE plpgsql;
```

### Edge Operations

```sql
-- Create edge with duplicate prevention
-- Edge type validation is application-level only (in Go). This allows future edge types
-- to be added without schema migration. The canonical list is in the Edge Type Registry
-- above and enforced by the Go CLI before calling this function.
CREATE OR REPLACE FUNCTION create_edge(
    p_from_id TEXT,
    p_to_id TEXT,
    p_edge_type TEXT,
    p_metadata JSONB DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
    rows_affected INT;
BEGIN
    -- Verify both shards exist
    IF NOT EXISTS (SELECT 1 FROM shards WHERE id = p_from_id) THEN
        RAISE EXCEPTION 'Shard % not found', p_from_id;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM shards WHERE id = p_to_id) THEN
        RAISE EXCEPTION 'Shard % not found', p_to_id;
    END IF;

    -- Prevent self-reference
    IF p_from_id = p_to_id THEN
        RAISE EXCEPTION 'Cannot create edge from a shard to itself';
    END IF;

    -- Insert with duplicate prevention
    INSERT INTO edges (from_id, to_id, edge_type, metadata)
    VALUES (p_from_id, p_to_id, p_edge_type, COALESCE(p_metadata, '{}'))
    ON CONFLICT (from_id, to_id, edge_type) DO NOTHING;

    GET DIAGNOSTICS rows_affected = ROW_COUNT;

    IF rows_affected = 0 THEN
        RAISE EXCEPTION 'Edge already exists: % --%--> %', p_from_id, p_edge_type, p_to_id;
    END IF;

    RETURN true;
END;
$$ LANGUAGE plpgsql;

-- Delete edge
CREATE OR REPLACE FUNCTION delete_edge(
    p_from_id TEXT,
    p_to_id TEXT,
    p_edge_type TEXT
) RETURNS BOOLEAN AS $$
DECLARE
    rows_affected INT;
BEGIN
    DELETE FROM edges
    WHERE from_id = p_from_id AND to_id = p_to_id AND edge_type = p_edge_type;

    GET DIAGNOSTICS rows_affected = ROW_COUNT;

    IF rows_affected = 0 THEN
        RAISE EXCEPTION 'No edge of type ''%'' from % to %', p_edge_type, p_from_id, p_to_id;
    END IF;

    RETURN true;
END;
$$ LANGUAGE plpgsql;
```

### Label Summary

```sql
-- All labels in use with counts (excludes closed shards — closed shards'
-- labels don't appear. Use cp shard list --label X --status closed to find
-- closed shards with a specific label.)
CREATE OR REPLACE FUNCTION label_summary(p_project TEXT)
RETURNS TABLE (
    label TEXT,
    shard_count INT
) AS $$
    SELECT
        unnest(s.labels) AS label,
        count(*)::int AS shard_count
    FROM shards s
    WHERE s.project = p_project
      AND s.status != 'closed'
      AND s.labels IS NOT NULL
      AND array_length(s.labels, 1) > 0
    GROUP BY 1
    ORDER BY 2 DESC, 1;
$$ LANGUAGE sql STABLE;
```

### Shard Update

```sql
-- Update shard content and/or title. No versioning — this is the low-level update.
-- For knowledge documents, use SPEC-4's update_knowledge_doc() instead.
-- Returns updated shard fields for confirmation output.
CREATE OR REPLACE FUNCTION update_shard(
    p_shard_id TEXT,
    p_project TEXT,
    p_title TEXT DEFAULT NULL,
    p_content TEXT DEFAULT NULL
) RETURNS TABLE (
    id TEXT,
    updated_at TIMESTAMPTZ,
    title_changed BOOLEAN,
    content_changed BOOLEAN,
    shard_type TEXT
) AS $$
DECLARE
    current_type TEXT;
BEGIN
    -- Verify shard exists and get type
    SELECT s.type INTO current_type
    FROM shards s WHERE s.id = p_shard_id AND s.project = p_project;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    -- Update title and/or content
    UPDATE shards s
    SET title = COALESCE(p_title, s.title),
        content = COALESCE(p_content, s.content),
        updated_at = now()
    WHERE s.id = p_shard_id AND s.project = p_project;

    RETURN QUERY SELECT
        p_shard_id,
        now(),
        (p_title IS NOT NULL),
        (p_content IS NOT NULL),
        current_type;
END;
$$ LANGUAGE plpgsql;
```

### SPEC-1 Amendment: `semantic_search()` with `p_since`

The `--since` filter MUST be applied inside `semantic_search()` to avoid `--limit`
returning fewer results than requested. This amends SPEC-1's function signature:

```sql
-- AMENDED: adds p_since TIMESTAMPTZ parameter (8th parameter)
-- Original SPEC-1 signature has 7 parameters; this adds p_since after p_min_similarity.
-- Go code computes cutoff: time.Now().Add(-duration) → passes TIMESTAMPTZ.
CREATE OR REPLACE FUNCTION semantic_search(
    p_project TEXT,
    p_query_embedding vector(768),
    p_types TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_limit INT DEFAULT 20,
    p_min_similarity FLOAT DEFAULT 0.3,
    p_since TIMESTAMPTZ DEFAULT NULL          -- NEW: time cutoff
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    type TEXT,
    status TEXT,
    similarity FLOAT,
    snippet TEXT,
    labels TEXT[],
    created_at TIMESTAMPTZ
) AS $$
    SELECT
        s.id, s.title, s.type, s.status,
        1 - (s.embedding <=> p_query_embedding) AS similarity,
        LEFT(s.content, 200) AS snippet,
        s.labels,
        s.created_at
    FROM shards s
    WHERE s.project = p_project
      AND s.embedding IS NOT NULL
      AND 1 - (s.embedding <=> p_query_embedding) >= p_min_similarity
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_labels IS NULL OR s.labels && p_labels)
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_since IS NULL OR s.created_at >= p_since)     -- NEW
    ORDER BY s.embedding <=> p_query_embedding
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;
```

### Enhanced Memory

```sql
-- Semantic search limited to memory shards
CREATE OR REPLACE FUNCTION memory_recall(
    p_project TEXT,
    p_query_embedding vector(768),
    p_labels TEXT[] DEFAULT NULL,
    p_limit INT DEFAULT 10,
    p_min_similarity FLOAT DEFAULT 0.3
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    content TEXT,
    similarity FLOAT,
    labels TEXT[],
    created_at TIMESTAMPTZ
) AS $$
    SELECT
        s.id, s.title, s.content,
        1 - (s.embedding <=> p_query_embedding) AS similarity,
        s.labels, s.created_at
    FROM shards s
    WHERE s.project = p_project
      AND s.type = 'memory'
      AND s.status != 'closed'
      AND s.embedding IS NOT NULL
      AND 1 - (s.embedding <=> p_query_embedding) >= p_min_similarity
      AND (p_labels IS NULL OR s.labels && p_labels)
    ORDER BY s.embedding <=> p_query_embedding
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;
```

## Go Implementation Notes

### Package Structure (additions to SPEC-0)

```
cp/
├── cmd/
│   ├── recall.go               # cp recall
│   ├── shard.go                # cp shard list/show/create/update/close/reopen
│   ├── shard_edges.go          # cp shard edges/link/unlink
│   ├── shard_label.go          # cp shard label add/remove/list
│   └── memory.go               # Enhanced cp memory (add --label, recall)
└── internal/
    └── client/
        ├── search.go           # Semantic search wrapper (embed query + call function)
        ├── shards.go           # General shard CRUD (extend existing)
        ├── edges.go            # Edge CRUD operations
        └── labels.go           # Label operations
```

### Key Types

```go
// RecallOpts holds all flags for cp recall
type RecallOpts struct {
    Types          []string
    Labels         []string
    Status         []string
    IncludeClosed  bool
    MinSimilarity  float64
    Since          *time.Time  // Cutoff time (converted from duration or date)
    Limit          int
    ShowSnippet    bool
    OutputFormat   string
}

// SearchResult from semantic_search or recall
type SearchResult struct {
    ID         string    `json:"id"`
    Title      string    `json:"title"`
    Type       string    `json:"type"`
    Status     string    `json:"status"`
    Similarity float64   `json:"similarity"`
    Labels     []string  `json:"labels"`
    CreatedAt  time.Time `json:"created_at"`
    Snippet    string    `json:"snippet,omitempty"`
}

// Valid edge types (canonical registry)
var ValidEdgeTypes = []string{
    "blocked-by", "blocks", "child-of", "discovered-from", "extends",
    "has-artifact", "implements", "parent", "previous-version",
    "references", "relates-to", "replies-to", "triggered-by",
}
```

### Recall Flow

```go
func (c *Client) Recall(ctx context.Context, query string, opts RecallOpts) ([]SearchResult, error) {
    // 1. Embed the query
    embedding, err := c.embedder.Embed(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to embed query: %w", err)
    }

    // 2. Convert since to TIMESTAMPTZ cutoff
    var sinceCutoff *time.Time
    if opts.Since != nil {
        sinceCutoff = opts.Since
    }

    // 3. Build status filter
    status := opts.Status
    if opts.IncludeClosed {
        status = nil // no filter
    } else if status == nil {
        status = []string{"open"} // default
    }

    // 4. Call semantic_search SQL function (SPEC-1)
    // Note: --since is passed to the SQL function, not filtered post-query
    rows, err := c.conn.Query(ctx,
        "SELECT * FROM semantic_search($1, $2, $3, $4, $5, $6, $7, $8)",
        c.project, embedding, opts.Types, opts.Labels, status,
        opts.Limit, opts.MinSimilarity, sinceCutoff,
    )
    if err != nil {
        return nil, fmt.Errorf("semantic search: %w", err)
    }

    return scanSearchResults(rows)
}
```

**Important:** The `--since` filter MUST be applied inside the `semantic_search()` SQL
function, not post-query in Go. Post-query filtering would cause `--limit 5` to return
fewer than 5 results when some high-similarity results are filtered out by `--since`.
This requires adding a `p_since TIMESTAMPTZ` parameter to `semantic_search()` (SPEC-1
amendment). If SPEC-1 is not yet updated, use `p_since INTERVAL` and convert in Go.

### Edge Creation with Validation

```go
func (c *Client) CreateEdge(ctx context.Context, fromID, toID, edgeType string) error {
    // 1. Validate edge type
    if !isValidEdgeType(edgeType) {
        return fmt.Errorf("unknown edge type: %s. Valid types: %s",
            edgeType, strings.Join(ValidEdgeTypes, ", "))
    }

    // 2. For blocked-by edges, check circular dependencies
    if edgeType == "blocked-by" {
        circular, err := c.HasCircularDependency(ctx, fromID, toID)
        if err != nil {
            return fmt.Errorf("check circular: %w", err)
        }
        if circular {
            return fmt.Errorf("circular dependency detected")
        }
    }

    // 3. Call create_edge SQL function (handles existence check, self-ref, duplicate)
    _, err := c.conn.Exec(ctx,
        "SELECT create_edge($1, $2, $3)",
        fromID, toID, edgeType,
    )
    return mapPgError(err)
}
```

## Success Criteria

1. **`cp shard list`:** Lists any shard type with --type, --status, --label, --creator,
   --since, --search filters. Pagination via --limit and --offset. Total count shown.
2. **`cp shard show`:** Shows content, metadata, labels, and full edge list in one view.
   JSON output includes edges inline.
3. **`cp shard create`:** Creates shards of any type with content, labels, metadata.
   Generates embedding (SPEC-1). Warns on unknown type. Requires --type and --title.
4. **`cp shard update`:** Updates content and/or title. Regenerates embedding on content
   change. Warns when updating knowledge docs (use `cp knowledge update` instead).
5. **`cp shard edges`:** Shows all incoming and outgoing edges with linked shard info.
   Filters by direction and edge type.
6. **`cp shard edges --follow`:** Tree view of N-hop edge navigation (default 2, max 5).
   Detects cycles and marks revisited shards.
7. **`cp shard link`:** Creates typed edges between shards. Validates both exist, rejects
   self-reference, rejects circular blocked-by, prevents duplicates via UNIQUE constraint.
8. **`cp shard unlink`:** Removes edges. Confirms before removing unless `--force`.
   Non-interactive mode requires `--force`.
9. **`cp shard label add/remove`:** Adds/removes labels atomically (SQL array ops, no
   read-modify-write). Duplicate add is no-op. Missing remove is no-op.
10. **`cp shard label list`:** Lists all labels in use with counts (excludes closed shards).
11. **`cp recall`:** Semantic search across all types. Ranked by similarity. `--since`
    filter applied in SQL before LIMIT. Supports all filter flags.
12. **`cp memory add --label`:** Memory shards with labels on create.
13. **`cp memory add --references`:** Creates reference edges from memory to other shards.
    Missing reference targets produce warning, not error.
14. **`cp memory recall`:** Semantic search limited to memory type.
15. **JSON output:** Every command supports `-o json` with defined schemas.
16. **Both agents:** Same commands, shared graph, agent identity from config.
17. **Edge uniqueness:** UNIQUE constraint on `(from_id, to_id, edge_type)` prevents duplicates.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| `cp shard list` no filters | All shards (any status), newest first, limit 20. |
| `cp shard list` no results | Empty table, exit code 0. |
| `cp shard show` non-existent ID | Error: "Shard pf-xxx not found." Exit code 1. |
| `cp shard show` on message shard | Works. Unified interface for all types. |
| `cp shard show` on version snapshot (e.g. pf-arch-001-v2) | Works. All shards treated equally regardless of origin. |
| `cp shard create` without --type | Error: "--type is required." |
| `cp shard create` without --title | Error: "--title is required." |
| `cp shard create` unknown type | Warn: "Type 'foo' is not a known type. Create anyway? (y/n)." In non-interactive mode, reject. |
| `cp shard create` with both --body and --body-file | Error: "Cannot use both --body and --body-file." |
| `cp shard create` with --body-file non-existent | Error: "Cannot read file 'missing.md': no such file or directory" |
| `cp shard create` with empty body | Allowed. Shard created with empty content. Embedding uses title only (SPEC-1). |
| `cp shard update` on knowledge shard | Warning: "This is a knowledge document. Use `cp knowledge update` to preserve version history." Update proceeds (low-level tool). |
| `cp shard update` with no update flags | Error: "At least one of --body, --body-file, or --title is required." |
| `cp shard close` already closed | No-op: "Shard pf-abc123 is already closed." |
| `cp shard reopen` already open | No-op: "Shard pf-abc123 is already open." |
| `cp recall` very short query (1 word) | Works but may have low discrimination. |
| `cp recall` no results above threshold | Empty result, exit code 0. "No results above 0.3 similarity." |
| `cp recall` without embedding config | Error: "Semantic search requires embedding config. Use `cp shard list --search` for text search." |
| `cp recall --status` and `--include-closed` both set | Error: "--status and --include-closed are mutually exclusive." |
| `cp recall --min-similarity 1.5` | Error: "min-similarity must be between 0.0 and 1.0" |
| `cp shard link` self-reference | Error: "Cannot create edge from a shard to itself." |
| `cp shard link` to non-existent shard | Error: "Shard pf-xxx not found." |
| `cp shard link` duplicate edge | Error: "Edge already exists: pf-a --implements--> pf-b." |
| `cp shard link` unknown edge type | Error: "Unknown edge type: foo. Valid: blocked-by, implements, references, ..." |
| `cp shard link --blocked-by` circular | Error: "Circular dependency detected." |
| `cp shard unlink` non-existent edge | Error: "No edge of type 'implements' from pf-a to pf-b." |
| `cp shard unlink` without --force in pipe | Error: "Use --force for non-interactive unlink." |
| `cp shard label add` duplicate label | No-op for that label. Labels are a set. |
| `cp shard label remove` non-existent label | No-op. |
| `cp shard label list` no labels in project | Empty table, exit code 0. |
| `cp shard edges --follow` with cycle | Shows each shard at most once. Marks revisited: "(cycle: pf-xxx already shown)". |
| `cp shard edges --follow --max-depth 10` | Error: "max-depth must be 1-5." |
| `cp memory add --references` invalid ID | Warning: "Shard pf-xxx not found. Memory created without edge." |
| `cp memory recall` no embedding config | Error: "Semantic recall requires embedding config. Use `cp memory search` for text search." |
| `--since` with bad format | Error: "Invalid duration: '7x'. Use format like '7d', '24h', or '2026-01-01'." |
| `cp shard list --type task,bug` | Returns shards of either type (OR within filter). |
| `cp shard list --label a,b` | Returns shards with ANY of those labels (OR / overlap). |
| `--limit 0` | Error: "Limit must be 1-1000." |
| `--limit 5000` | Error: "Limit must be 1-1000." |
| `--offset` without `--limit` | Allowed. Uses default limit (20) with the specified offset. |
| `cp shard delete` (no such command) | Intentionally omitted. Soft-delete via `cp shard close` is the pattern. Hard delete deferred to a future admin spec. Orphaned edges from externally-deleted shards are silently excluded by JOIN in `shard_edges()`. |
| Large result set | Paginate: "Showing 1-20 of 347 results." |
| `cp shard create` embedding fails | Shard created, warning logged, embedding NULL. |
| `cp shard update` embedding fails | Content updated, warning logged, embedding stale. |

---

## Cross-Spec Interactions

| Spec | Interaction |
|------|-------------|
| **SPEC-0** (CLI skeleton) | Uses `create_shard()` for all shard creation. Inherits `--project`, `-o`, `--debug` flags. |
| **SPEC-1** (semantic search) | `cp recall` wraps `semantic_search()`. Embedding generated on shard create/update. **Amendment included above:** `p_since TIMESTAMPTZ` parameter added to `semantic_search()` for correct `--since` + `--limit` behavior. Note: `semantic_search()` defaults `p_status` to NULL (all statuses), but `cp recall` overrides this to `["open"]` in Go. |
| **SPEC-2** (metadata) | Metadata read/written via SPEC-2's `update_metadata()` and GIN index. `cp shard show` displays metadata. Metadata updates go through `cp shard metadata set` (SPEC-2), not `cp shard update`. |
| **SPEC-3** (requirements) | `cp shard link --implements` creates the edge but does NOT trigger SPEC-3's lifecycle auto-transition (approved → in_progress). Use `cp requirement link --task` for lifecycle-aware linking. `cp shard link --blocked-by` uses `has_circular_dependency()` from SPEC-3. |
| **SPEC-4** (knowledge docs) | `cp shard update` on type=knowledge bypasses versioning. Warning printed. Knowledge version snapshots (closed shards) are visible via `cp shard show`. |
| **SPEC-5** defines the edge UNIQUE constraint and `create_edge()`/`delete_edge()` functions used by all specs that create edges. |
| **SPEC-6** (hierarchical memory) | Extends `cp memory` with `add-sub`, `tree`, `show --depth`, etc. Memory shards created via `cp shard create --type memory` are plain memories (no pointer blocks). SPEC-6 commands (`cp memory add-sub`, `cp memory show --depth`) handle hierarchical structure. |

## Test Cases

### SQL Tests: list_shards

```
TEST: list_shards returns shards for project
  Given: 5 shards in project 'test', 3 in project 'other'
  When:  SELECT * FROM list_shards('test')
  Then:  Returns 5 rows (project-isolated)

TEST: list_shards type filter
  Given: 3 tasks, 2 bugs, 1 memory in project 'test'
  When:  SELECT * FROM list_shards('test', ARRAY['task'])
  Then:  Returns 3 rows (tasks only)

TEST: list_shards multiple type filter
  Given: 3 tasks, 2 bugs, 1 memory
  When:  SELECT * FROM list_shards('test', ARRAY['task','bug'])
  Then:  Returns 5 rows (tasks + bugs)

TEST: list_shards status filter
  Given: 3 open shards, 2 closed shards
  When:  SELECT * FROM list_shards('test', NULL, ARRAY['open'])
  Then:  Returns 3 rows (open only)

TEST: list_shards default status (no filter)
  Given: 3 open, 2 closed shards
  When:  SELECT * FROM list_shards('test')
  Then:  Returns all 5 rows (no default status filter)

TEST: list_shards label filter
  Given: Shard A labels=['arch'], B labels=['deploy'], C labels=['arch','deploy']
  When:  SELECT * FROM list_shards('test', NULL, NULL, ARRAY['arch'])
  Then:  Returns A and C (both have 'arch')

TEST: list_shards label overlap (any match)
  Given: Shard A labels=['arch'], B labels=['deploy'], C labels=['arch','deploy']
  When:  SELECT * FROM list_shards('test', NULL, NULL, ARRAY['arch','deploy'])
  Then:  Returns A, B, and C (all have at least one matching label)

TEST: list_shards creator filter
  Given: 3 shards by agent-penfold, 2 by agent-mycroft
  When:  SELECT * FROM list_shards('test', NULL, NULL, NULL, 'agent-mycroft')
  Then:  Returns 2 rows

TEST: list_shards text search
  Given: Shard A content="timeout config", B content="deployment"
  When:  SELECT * FROM list_shards('test', NULL, NULL, NULL, NULL, 'timeout')
  Then:  Returns shard A

TEST: list_shards since filter
  Given: Shard A created 1 day ago, B created 10 days ago
  When:  SELECT * FROM list_shards('test', NULL, NULL, NULL, NULL, NULL, now() - interval '7 days')
  Then:  Returns shard A only

TEST: list_shards combined filters
  Given: 10 shards with various types, statuses, labels
  When:  SELECT * FROM list_shards('test', ARRAY['task'], ARRAY['open'], ARRAY['pipeline'])
  Then:  Returns only open tasks with 'pipeline' label

TEST: list_shards pagination
  Given: 25 shards
  When:  SELECT * FROM list_shards('test', NULL, NULL, NULL, NULL, NULL, NULL, 10, 0)
  Then:  Returns 10 rows (first page)
  When:  SELECT * FROM list_shards('test', NULL, NULL, NULL, NULL, NULL, NULL, 10, 10)
  Then:  Returns 10 rows (second page)

TEST: list_shards order by created_at DESC
  Given: 3 shards created at 9am, 10am, 11am
  When:  SELECT * FROM list_shards('test')
  Then:  11am first, 9am last

TEST: list_shards empty result
  Given: No shards matching filters
  When:  SELECT * FROM list_shards('test', ARRAY['nonexistent-type'])
  Then:  Returns 0 rows (not error)

TEST: list_shards includes snippet
  Given: Shard with 500-char content
  When:  SELECT * FROM list_shards('test')
  Then:  snippet is first 200 chars of content
```

### SQL Tests: list_shards_count

```
TEST: list_shards_count matches list_shards rows
  Given: 25 shards matching filters
  When:  SELECT list_shards_count('test', ARRAY['task'])
  Then:  Returns total count matching the same filters

TEST: list_shards_count with no matches
  Given: No matching shards
  When:  SELECT list_shards_count('test', ARRAY['nonexistent'])
  Then:  Returns 0
```

### SQL Tests: shard_detail

```
TEST: shard_detail returns shard with counts
  Given: Shard 'test-1' with 3 outgoing edges, 2 incoming edges
  When:  SELECT * FROM shard_detail('test-1')
  Then:  Returns 1 row with outgoing_edge_count=3, incoming_edge_count=2

TEST: shard_detail includes metadata
  Given: Shard with metadata = '{"priority": 2}'
  When:  SELECT * FROM shard_detail('test-1')
  Then:  metadata field contains the JSON object

TEST: shard_detail includes labels
  Given: Shard with labels = ['architecture', 'pipeline']
  When:  SELECT * FROM shard_detail('test-1')
  Then:  labels field contains both values

TEST: shard_detail non-existent shard
  Given: No shard with id 'nonexistent'
  When:  SELECT * FROM shard_detail('nonexistent')
  Then:  Returns 0 rows

TEST: shard_detail with no edges
  Given: Shard with no edges
  When:  SELECT * FROM shard_detail('test-1')
  Then:  outgoing_edge_count=0, incoming_edge_count=0
```

### SQL Tests: shard_edges

```
TEST: shard_edges returns both directions
  Given: Shard A with outgoing edge to B and incoming edge from C
  When:  SELECT * FROM shard_edges('A')
  Then:  Returns 2 rows: one outgoing (to B), one incoming (from C)

TEST: shard_edges direction filter
  Given: Shard A with 2 outgoing, 3 incoming edges
  When:  SELECT * FROM shard_edges('A', 'outgoing')
  Then:  Returns 2 rows (outgoing only)

TEST: shard_edges edge type filter
  Given: Shard A with implements, references, blocked-by edges
  When:  SELECT * FROM shard_edges('A', NULL, ARRAY['implements'])
  Then:  Returns only implements edges

TEST: shard_edges includes linked shard info
  Given: Edge from A to B, B has title="Task Title", type="task"
  When:  SELECT * FROM shard_edges('A')
  Then:  Row includes linked_shard_title, linked_shard_type

TEST: shard_edges includes edge metadata
  Given: Edge with metadata = '{"change_summary": "Added diagrams"}'
  When:  SELECT * FROM shard_edges('A')
  Then:  edge_metadata contains the JSON

TEST: shard_edges no edges
  Given: Shard with no edges
  When:  SELECT * FROM shard_edges('A')
  Then:  Returns 0 rows
```

### SQL Tests: add_shard_labels / remove_shard_labels

```
TEST: add_shard_labels adds new labels
  Given: Shard with labels=['arch']
  When:  SELECT add_shard_labels('test-1', ARRAY['pipeline', 'deploy'])
  Then:  Returns ['arch', 'deploy', 'pipeline'] (sorted, deduplicated)

TEST: add_shard_labels deduplicates
  Given: Shard with labels=['arch', 'pipeline']
  When:  SELECT add_shard_labels('test-1', ARRAY['arch', 'new'])
  Then:  Returns ['arch', 'new', 'pipeline'] (no duplicate 'arch')

TEST: add_shard_labels to shard with NULL labels
  Given: Shard with labels=NULL
  When:  SELECT add_shard_labels('test-1', ARRAY['first'])
  Then:  Returns ['first']

TEST: add_shard_labels non-existent shard
  Given: No such shard
  When:  SELECT add_shard_labels('missing', ARRAY['label'])
  Then:  RAISES EXCEPTION 'Shard missing not found'

TEST: remove_shard_labels removes labels
  Given: Shard with labels=['arch', 'pipeline', 'deploy']
  When:  SELECT remove_shard_labels('test-1', ARRAY['pipeline'])
  Then:  Returns ['arch', 'deploy']

TEST: remove_shard_labels non-existent label is no-op
  Given: Shard with labels=['arch']
  When:  SELECT remove_shard_labels('test-1', ARRAY['nonexistent'])
  Then:  Returns ['arch'] (unchanged)

TEST: remove_shard_labels non-existent shard
  Given: No such shard
  When:  SELECT remove_shard_labels('missing', ARRAY['label'])
  Then:  RAISES EXCEPTION 'Shard missing not found'
```

### SQL Tests: create_edge / delete_edge

```
TEST: create_edge creates edge
  Given: Shards A and B exist
  When:  SELECT create_edge('A', 'B', 'references')
  Then:  Returns true. Edge exists in edges table.

TEST: create_edge prevents duplicate
  Given: Edge A --references--> B already exists
  When:  SELECT create_edge('A', 'B', 'references')
  Then:  RAISES EXCEPTION 'Edge already exists'

TEST: create_edge prevents self-reference
  Given: Shard A exists
  When:  SELECT create_edge('A', 'A', 'references')
  Then:  RAISES EXCEPTION 'Cannot create edge from a shard to itself'

TEST: create_edge validates from shard exists
  Given: Shard B exists, shard 'missing' does not
  When:  SELECT create_edge('missing', 'B', 'references')
  Then:  RAISES EXCEPTION 'Shard missing not found'

TEST: create_edge validates to shard exists
  Given: Shard A exists, shard 'missing' does not
  When:  SELECT create_edge('A', 'missing', 'references')
  Then:  RAISES EXCEPTION 'Shard missing not found'

TEST: create_edge with metadata
  Given: Shards A and B exist
  When:  SELECT create_edge('A', 'B', 'references', '{"note": "related"}')
  Then:  Edge metadata contains {"note": "related"}

TEST: delete_edge removes edge
  Given: Edge A --references--> B exists
  When:  SELECT delete_edge('A', 'B', 'references')
  Then:  Returns true. Edge no longer exists.

TEST: delete_edge non-existent
  Given: No edge from A to B
  When:  SELECT delete_edge('A', 'B', 'references')
  Then:  RAISES EXCEPTION 'No edge of type references from A to B'
```

### SQL Tests: label_summary

```
TEST: label_summary counts labels
  Given: 3 shards with 'arch' label, 2 with 'deploy', 1 with both
  When:  SELECT * FROM label_summary('test')
  Then:  arch=4, deploy=3

TEST: label_summary excludes closed shards
  Given: Open shard with 'arch', closed shard with 'arch'
  When:  SELECT * FROM label_summary('test')
  Then:  arch=1

TEST: label_summary ordered by count DESC
  Given: Labels with counts 5, 2, 8
  When:  SELECT * FROM label_summary('test')
  Then:  Count 8 first, count 2 last

TEST: label_summary empty project
  Given: No shards in project
  When:  SELECT * FROM label_summary('test')
  Then:  Returns 0 rows

TEST: label_summary ignores null/empty labels
  Given: Shards with NULL labels and empty array labels
  When:  SELECT * FROM label_summary('test')
  Then:  Returns only real labels
```

### SQL Tests: memory_recall

```
TEST: memory_recall returns similar memories
  Given: 3 memory shards with known embeddings (sim ~0.9, ~0.6, ~0.2)
  When:  SELECT * FROM memory_recall('test', <query_vector>)
  Then:  Returns high and medium similarity (above 0.3), highest first

TEST: memory_recall excludes non-memory shards
  Given: Memory shard (sim 0.9), task shard (sim 0.95)
  When:  SELECT * FROM memory_recall('test', <query_vector>)
  Then:  Returns only memory shard

TEST: memory_recall label filter
  Given: Memory A labels=['lesson'], Memory B labels=['deploy']
  When:  SELECT * FROM memory_recall('test', <vec>, ARRAY['lesson'])
  Then:  Returns only Memory A

TEST: memory_recall excludes closed
  Given: Open memory (sim 0.9), closed memory (sim 0.85)
  When:  SELECT * FROM memory_recall('test', <vec>)
  Then:  Returns only open memory

TEST: memory_recall respects limit
  Given: 15 memory shards all above threshold
  When:  SELECT * FROM memory_recall('test', <vec>, NULL, 5)
  Then:  Returns exactly 5

TEST: memory_recall no results
  Given: No memory shards with embeddings
  When:  SELECT * FROM memory_recall('test', <vec>)
  Then:  Returns 0 rows
```

### Go Unit Tests

```
TEST: parseRecallFlags defaults
  Given: `cp recall "query"` (no flags)
  When:  parseRecallFlags()
  Then:  types=nil, labels=nil, status=["open"], limit=20, minSimilarity=0.3

TEST: parseRecallFlags with type filter
  Given: `cp recall "query" --type requirement,bug`
  When:  parseRecallFlags()
  Then:  types=["requirement", "bug"]

TEST: parseRecallFlags with include-closed
  Given: `cp recall "query" --include-closed`
  When:  parseRecallFlags()
  Then:  status=nil (no filter)

TEST: parseRecallFlags with explicit status
  Given: `cp recall "query" --status closed`
  When:  parseRecallFlags()
  Then:  status=["closed"]

TEST: parseRecallFlags status and include-closed conflict
  Given: `cp recall "query" --status open --include-closed`
  When:  parseRecallFlags()
  Then:  Error: "mutually exclusive"

TEST: parseRecallFlags with min-similarity
  Given: `cp recall "query" --min-similarity 0.6`
  When:  parseRecallFlags()
  Then:  minSimilarity=0.6

TEST: parseRecallFlags invalid min-similarity
  Given: `cp recall "query" --min-similarity 1.5`
  When:  parseRecallFlags()
  Then:  Error: "must be between 0.0 and 1.0"

TEST: parseRecallFlags with since duration
  Given: `cp recall "query" --since 7d`
  When:  parseRecallFlags()
  Then:  since = time 7 days ago

TEST: parseRecallFlags with since date
  Given: `cp recall "query" --since 2026-01-01`
  When:  parseRecallFlags()
  Then:  since = 2026-01-01T00:00:00Z

TEST: parseRecallFlags invalid since
  Given: `cp recall "query" --since 7x`
  When:  parseRecallFlags()
  Then:  Error: "Invalid duration"

TEST: parseSinceDuration with various formats
  Given: "7d", "24h", "30m", "2w"
  When:  parseSinceDuration(input)
  Then:  Returns correct time.Time cutoffs

TEST: formatRecallResults text output
  Given: 3 results with similarity, type, status, id, title
  When:  formatRecallResults(results, "text", false)
  Then:  Aligned table with SIMILARITY, TYPE, STATUS, ID, TITLE columns

TEST: formatRecallResults with snippets
  Given: 2 results with snippets, showSnippet=true
  When:  formatRecallResults(results, "text", true)
  Then:  Each result followed by indented snippet line

TEST: formatRecallResults JSON output
  Given: 3 results
  When:  formatRecallResults(results, "json", false)
  Then:  Valid JSON array

TEST: formatRecallResults empty results
  Given: 0 results
  When:  formatRecallResults(results, "text", false)
  Then:  "No results above 0.3 similarity."

TEST: parseShardListFlags defaults
  Given: `cp shard list` (no flags)
  When:  parseShardListFlags()
  Then:  types=nil, status=nil (all statuses), labels=nil, limit=20, offset=0

TEST: validateEdgeType valid
  Given: "implements"
  When:  validateEdgeType("implements")
  Then:  Returns nil

TEST: validateEdgeType invalid
  Given: "foo"
  When:  validateEdgeType("foo")
  Then:  Error listing valid types

TEST: validEdgeTypes includes all known types
  Given: ValidEdgeTypes constant
  Then:  Contains all 13 types from registry

TEST: parseLabelArgs valid
  Given: ["architecture", "pipeline"]
  When:  parseLabelArgs(args)
  Then:  Returns ["architecture", "pipeline"]

TEST: parseLabelArgs empty
  Given: []
  When:  parseLabelArgs(args)
  Then:  Error: "at least one label required"

TEST: formatEdgeTree with cycle detection
  Given: Edge graph with A -> B -> A cycle
  When:  formatEdgeTree(A, edges, visited)
  Then:  Shows A, then B, then "(cycle: A already shown)" for the back-edge
```

### Integration Tests

```
TEST: recall finds semantically similar shards
  Given: Shard about "Nomad deployment failed"
  When:  `cp recall "deployment problems"`
  Then:  Shard appears with similarity > 0.5

TEST: recall type filter works
  Given: Task and bug both about "timeout"
  When:  `cp recall "timeout" --type bug`
  Then:  Only bug returned

TEST: recall with no matches
  Given: Shards about software development
  When:  `cp recall "medieval castle architecture"`
  Then:  Empty result, exit code 0

TEST: recall --include-closed shows closed
  Given: Open bug about timeout, closed task about timeout
  When:  `cp recall "timeout"`
  Then:  Only open bug
  When:  `cp recall "timeout" --include-closed`
  Then:  Both returned

TEST: recall --since filters in SQL (not post-query)
  Given: 10 shards, 3 created in last 7 days, 7 older
  When:  `cp recall "query" --since 7d --limit 5`
  Then:  Returns at most 3 results (not 5 with post-filtering)

TEST: recall --show-snippet shows content preview
  Given: Shard with content
  When:  `cp recall "query" --show-snippet`
  Then:  Output includes indented content preview

TEST: recall JSON output
  Given: Matching shards
  When:  `cp recall "query" -o json`
  Then:  Valid JSON array with id, title, type, status, similarity fields

TEST: recall without embedding config
  Given: No embedding section in config
  When:  `cp recall "query"`
  Then:  Error about missing config, suggests text search

TEST: shard list with type filter
  Given: 3 tasks, 2 bugs
  When:  `cp shard list --type task`
  Then:  3 rows, all type=task

TEST: shard list default shows all statuses
  Given: 2 open, 3 closed shards
  When:  `cp shard list`
  Then:  5 rows (all statuses)

TEST: shard list with label filter
  Given: 2 shards with 'arch' label, 3 without
  When:  `cp shard list --label arch`
  Then:  2 rows

TEST: shard list with text search
  Given: Shards with various content
  When:  `cp shard list --search "timeout"`
  Then:  Only shards matching in tsvector

TEST: shard list pagination shows total
  Given: 30 shards
  When:  `cp shard list --limit 10`
  Then:  10 rows, "Showing 1-10 of 30 results"
  When:  `cp shard list --limit 10 --offset 10`
  Then:  Next 10 rows, "Showing 11-20 of 30 results"

TEST: shard show full detail
  Given: Shard with content, metadata, labels, edges
  When:  `cp shard show <id>`
  Then:  All sections displayed

TEST: shard show JSON includes edges
  Given: Shard with edges
  When:  `cp shard show <id> -o json`
  Then:  JSON includes edges array

TEST: shard show non-existent
  Given: No such shard
  When:  `cp shard show nonexistent`
  Then:  Exit code 1, "not found"

TEST: shard create with all options
  Given: Valid config
  When:  `cp shard create --type design --title "Test" --body "Content" --label arch --meta '{"scope":"system"}'`
  Then:  Shard created with all fields, has embedding

TEST: shard create without type fails
  When:  `cp shard create --title "Test" --body "Content"`
  Then:  Error: "--type is required"

TEST: shard create without title fails
  When:  `cp shard create --type design --body "Content"`
  Then:  Error: "--title is required"

TEST: shard create embedding failure is graceful
  Given: Embedding provider returns error
  When:  `cp shard create --type design --title "Test" --body "Content"`
  Then:  Shard created (exit code 0), embedding is NULL, warning printed

TEST: shard update content
  Given: Shard with content "Original"
  When:  `cp shard update <id> --body "Updated"`
  Then:  Content is "Updated", embedding regenerated

TEST: shard update title
  Given: Shard with title "Original Title"
  When:  `cp shard update <id> --title "New Title"`
  Then:  Title changed

TEST: shard update knowledge shard warns
  Given: Shard with type=knowledge
  When:  `cp shard update <id> --body "New content"`
  Then:  Warning about using cp knowledge update, content still updated

TEST: shard close
  Given: Open shard
  When:  `cp shard close <id>`
  Then:  Status is 'closed', updated_at refreshed

TEST: shard reopen
  Given: Closed shard
  When:  `cp shard reopen <id>`
  Then:  Status is 'open', updated_at refreshed

TEST: shard close already closed
  Given: Closed shard
  When:  `cp shard close <id>`
  Then:  No-op message: "Shard <id> is already closed." Exit code 0.

TEST: shard reopen already open
  Given: Open shard
  When:  `cp shard reopen <id>`
  Then:  No-op message: "Shard <id> is already open." Exit code 0.

TEST: shard close non-existent
  Given: No shard with id
  When:  `cp shard close nonexistent`
  Then:  Error: "Shard nonexistent not found." Exit code 1.

TEST: shard close JSON output
  Given: Open shard
  When:  `cp shard close <id> -o json`
  Then:  Valid JSON with id, status="closed", updated_at

TEST: shard link creates edge
  Given: Shards A and B
  When:  `cp shard link A --implements B`
  Then:  Edge visible in `cp shard edges A`

TEST: shard link self-reference rejected
  Given: Shard A
  When:  `cp shard link A --references A`
  Then:  Error: "Cannot create edge from a shard to itself"

TEST: shard link validates shards exist
  Given: Shard A exists, B does not
  When:  `cp shard link A --references nonexistent`
  Then:  Error: "not found"

TEST: shard link rejects duplicate
  Given: Edge already exists
  When:  `cp shard link A --implements B` again
  Then:  Error: "already exists"

TEST: shard link rejects circular blocked-by
  Given: A blocked-by B
  When:  `cp shard link B --blocked-by A`
  Then:  Error: "Circular dependency"

TEST: shard unlink removes edge
  Given: Edge from A to B
  When:  `cp shard unlink A --implements B --force`
  Then:  Edge gone

TEST: shard unlink non-existent
  Given: No such edge
  When:  `cp shard unlink A --implements B --force`
  Then:  Error: "No edge"

TEST: shard label add
  Given: Shard with labels=['arch']
  When:  `cp shard label add <id> pipeline deploy`
  Then:  Labels = ['arch', 'deploy', 'pipeline']

TEST: shard label add duplicate is no-op
  Given: Shard with labels=['arch']
  When:  `cp shard label add <id> arch`
  Then:  Labels unchanged

TEST: shard label remove
  Given: Shard with labels=['arch', 'pipeline']
  When:  `cp shard label remove <id> pipeline`
  Then:  Labels = ['arch']

TEST: shard label list shows counts
  Given: Various shards with labels
  When:  `cp shard label list`
  Then:  Table with label counts, ordered by count DESC

TEST: shard edges follow mode with cycle
  Given: A --implements--> B, B --blocked-by--> A
  When:  `cp shard edges A --follow`
  Then:  Tree showing A, B, then "(cycle: A already shown)"

TEST: two agents share graph
  Given: Agent-penfold creates shard, agent-mycroft creates edge to it
  When:  Both agents query
  Then:  Both see the shard and edge

TEST: memory add with labels
  When:  `cp memory add "Test memory" --label lesson-learned,pipeline`
  Then:  Shard has labels

TEST: memory add with references
  Given: Shard pf-bug-03 exists
  When:  `cp memory add "Related to bug" --references pf-bug-03`
  Then:  Memory created, references edge exists

TEST: memory add with references to non-existent shard
  Given: No shard 'nonexistent'
  When:  `cp memory add "Test" --references nonexistent`
  Then:  Memory created, warning printed, no edge

TEST: memory list with label filter
  Given: Memory A labels=['lesson-learned'], B labels=['pipeline'], C labels=['lesson-learned','pipeline']
  When:  `cp memory list --label lesson-learned`
  Then:  Returns A and C

TEST: memory list with since filter
  Given: Memory A created 2 days ago, B created 10 days ago
  When:  `cp memory list --since 7d`
  Then:  Returns A only

TEST: memory list --roots shows only root memories
  Given: Root memory A (parent_id=NULL), child memory B (parent_id=A)
  When:  `cp memory list --roots`
  Then:  Returns A only (B excluded)

TEST: memory list default shows all memories
  Given: Root memory A, child memory B (parent_id=A)
  When:  `cp memory list`
  Then:  Returns A and B

TEST: memory recall semantic search
  Given: Memory about "Nomad deployment"
  When:  `cp memory recall "deployment problems"`
  Then:  Memory appears with similarity

TEST: memory recall only returns memories
  Given: Memory and task both about "timeout"
  When:  `cp memory recall "timeout"`
  Then:  Only memory returned
```

---

## Pre-Submission Checklist

- [x] Every item in "What to Build" has: CLI section + SQL + success criterion + tests
- [x] Every data flow answers all 7 questions (who writes/when/where/who reads/how/what for/staleness)
- [x] Every command has: syntax + example + output + atomic steps + JSON schema
- [x] Every workflow has: flowchart + all branches + error recovery + non-interactive mode
- [x] Every success criterion has at least one test case
- [x] Concurrency is addressed (UNIQUE constraint, atomic label ops, MVCC)
- [x] No feature is "mentioned but not specced" (memory search/resolve/defer documented as penf-compatible)
- [x] Edge cases cover: invalid input, empty state, conflicts, boundaries, cross-feature, failure recovery
- [x] Existing spec interactions documented (Cross-Spec Interactions table)
- [x] Sub-agent review completed (20 items found: 1 High fixed (semantic_search amendment), 8 Medium fixed (ROW_COUNT type, close/reopen, update SQL, memory subcommands, since consistency, edge type validation note), 9 Low addressed)
