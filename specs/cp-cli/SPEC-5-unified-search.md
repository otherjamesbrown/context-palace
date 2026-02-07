# SPEC-5: Unified Search & Graph CLI

**Status:** Draft
**Depends on:** SPEC-1 (semantic search), SPEC-2 (metadata column)
**Blocks:** Nothing

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

## What to Build

1. **`cp shard` commands** — general CRUD for any shard type, edge browsing, label ops
2. **`cp recall`** — semantic search across all shard types (signature command)
3. **Enhanced `cp memory`** — labels, edges to related shards, semantic recall
4. **Edge navigation** — show linked shards, create/remove edges
5. **Label management** — add/remove labels, query by label

## Data Model

No new database changes. SPEC-5 builds on:
- SPEC-1: `embedding` column + `semantic_search()` function
- SPEC-2: `metadata` column + `update_metadata()` function
- Existing: `labels`, `edges`, `shards` tables

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
search_vector            — tsvector for keyword search
```

## CLI Surface

### `cp recall` — Semantic Search

The signature command. Single semantic search across everything in the project.

```bash
# Basic semantic search
cp recall "pipeline timeout issues"
# Output:
#   SIMILARITY  TYPE         STATUS  ID          TITLE
#   0.94        bug          open    pf-c74eea   Timeout not applied after deploy
#   0.91        requirement  draft   pf-req-04   Structured Error Codes
#   0.88        task         closed  pf-3acaf1   Wire timeout config into worker
#   0.85        memory       open    pf-mem-12   Lesson: AI client vs heartbeat timeout
#   0.78        knowledge    open    pf-arch-01  System Architecture

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
# Output:
#   SIMILARITY  TYPE         ID          TITLE
#   0.91        requirement  pf-req-01   Entity Lifecycle Management
#     "Let me reject junk entities and manage filter rules. Currently..."
#   0.85        bug          pf-bug-03   Entity extraction missing display names
#     "77 of 93 entities have no display name. The extraction pipeline..."
```

### `cp shard` — General Shard Operations

Unified interface for browsing and managing any shard type.

```bash
# List shards with filters
cp shard list
cp shard list --type task --status open
cp shard list --type requirement,bug --status open --since 7d
cp shard list --creator agent-mycroft --limit 50
cp shard list --label architecture
cp shard list --search "timeout"    # text search (tsvector, not semantic)

# Output:
#   ID          TYPE         STATUS  CREATED      TITLE
#   pf-c74eea   bug          open    2026-02-06   Fixes STILL not working
#   pf-req-01   requirement  open    2026-02-07   Entity Lifecycle Management
#   pf-3acaf1   task         closed  2026-02-05   Wire timeout config

# Show shard detail (content + metadata + labels + edges)
cp shard show pf-c74eea
# Output:
#   Fixes STILL not working - root cause found
#   ──────────────────────────────────────────
#   ID:       pf-c74eea
#   Type:     bug
#   Status:   open
#   Creator:  agent-penfold
#   Created:  2026-02-06 21:30
#   Labels:   pipeline, timeout
#
#   Metadata:
#     root_cause: "AI client timeout hardcoded at 120s"
#     affects_requirement: pf-req-04
#
#   Content:
#     [full shard content]
#
#   Edges:
#     DIRECTION  EDGE TYPE      SHARD          TYPE    TITLE
#     outgoing   references     pf-3acaf1      task    Wire timeout config
#     incoming   discovered-from pf-session-01  session Session: 2026-02-06

# Create shard (any type)
cp shard create --type design \
    --title "Entity filter architecture" \
    --body-file design.md \
    --label architecture,entity \
    --meta '{"scope": "component", "components": ["worker"]}'

# Update shard content
cp shard update pf-abc123 --body-file updated.md

# Update shard title
cp shard update pf-abc123 --title "New Title"

# Close/reopen
cp shard close pf-abc123
cp shard reopen pf-abc123

# JSON output
cp shard list --type task -o json
cp shard show pf-abc123 -o json
```

### `cp shard edges` — Edge Navigation

Browse the graph. See what's connected to what.

```bash
# Show all edges for a shard
cp shard edges pf-abc123
# Output:
#   DIRECTION  EDGE TYPE        SHARD          TYPE         STATUS  TITLE
#   outgoing   implements       pf-req-01      requirement  open    Entity Lifecycle Mgmt
#   outgoing   references       pf-bug-03      bug          open    Missing display names
#   incoming   blocked-by       pf-req-05      requirement  open    Reprocessing Overrides
#   incoming   has-artifact     pf-test-01     test         open    Entity reject tests

# Filter by direction
cp shard edges pf-abc123 --direction outgoing
cp shard edges pf-abc123 --direction incoming

# Filter by edge type
cp shard edges pf-abc123 --edge-type implements
cp shard edges pf-abc123 --edge-type blocked-by,references

# Create edges
cp shard link pf-task-123 --implements pf-req-01
cp shard link pf-bug-03 --references pf-task-456
cp shard link pf-req-05 --blocked-by pf-req-03
cp shard link pf-doc-01 --references pf-req-01

# Remove edges
cp shard unlink pf-task-123 --implements pf-req-01

# Follow edges (2-hop navigation)
cp shard edges pf-req-01 --follow
# Output:
#   pf-req-01 "Entity Lifecycle Management"
#   ├── implements
#   │   ├── pf-task-001 "Add reject endpoint" (closed)
#   │   ├── pf-task-002 "Add bulk reject" (open)
#   │   └── pf-task-003 "Add filter rules table" (open)
#   ├── has-artifact
#   │   └── pf-test-001 "Entity reject tests" (open)
#   ├── blocked-by (incoming)
#   │   └── pf-req-05 "Reprocessing Overrides" (draft)
#   └── references (incoming)
#       └── pf-bug-03 "Missing display names" (open)
```

### `cp shard label` — Label Management

```bash
# Add labels to a shard
cp shard label add pf-abc123 architecture pipeline
# Output: Labels: architecture, pipeline, timeout

# Remove labels
cp shard label remove pf-abc123 pipeline
# Output: Labels: architecture, timeout

# List all labels in use (for discovery)
cp shard labels
# Output:
#   LABEL           COUNT
#   architecture    12
#   pipeline        8
#   deployment      6
#   timeout         5
#   lesson-learned  4
#   entity          3

# Filter shards by label
cp shard list --label architecture
cp shard list --label architecture,pipeline   # shards with ANY of these labels
```

### Enhanced `cp memory`

Memory commands migrated from `penf` and enhanced with labels, links, and semantic recall.

```bash
# Add memory (basic — same as penf)
cp memory add "Nomad deploys are unreliable — always verify with version check"

# Add memory with labels
cp memory add "AI client timeout was hardcoded at 120s, not configurable" \
    --label timeout,pipeline,lesson-learned

# Add memory with edge links
cp memory add "Entity display names are missing because NER stage doesn't extract them" \
    --label entity,pipeline \
    --references pf-bug-03,pf-req-01

# List memories (same as penf)
cp memory list
cp memory list --label lesson-learned
cp memory list --since 7d

# Text search (same as penf)
cp memory search "timeout"

# Semantic recall (NEW — uses embedding)
cp memory recall "deployment issues"
# Output:
#   SIMILARITY  ID          CREATED      CONTENT
#   0.91        pf-mem-12   2026-02-06   Lesson: AI client vs heartbeat timeout...
#   0.87        pf-mem-08   2026-02-05   Nomad deploys are unreliable...
#   0.82        pf-mem-15   2026-02-07   Always verify with penf version after deploy...

# Resolve/defer (same as penf)
cp memory resolve pf-mem-12
cp memory defer pf-mem-08 --until 2026-02-14
```

## SQL Functions

### Filtered Shard List

```sql
-- General shard listing with all filters
CREATE OR REPLACE FUNCTION list_shards(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since INTERVAL DEFAULT NULL,
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
      AND (p_since IS NULL OR s.created_at >= now() - p_since)
    ORDER BY s.created_at DESC
    LIMIT p_limit
    OFFSET p_offset;
$$ LANGUAGE sql STABLE;
```

### Shard Detail with Edges

```sql
-- Get shard with all edges
CREATE OR REPLACE FUNCTION shard_detail(p_shard_id TEXT)
RETURNS TABLE (
    -- Shard fields
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
    -- Edge counts
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

    ORDER BY edge_type, direction;
$$ LANGUAGE sql STABLE;
```

### Label Summary

```sql
-- All labels in use with counts
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
│   ├── shard.go                # cp shard list/show/create/update/close
│   ├── shard_edges.go          # cp shard edges/link/unlink
│   ├── shard_label.go          # cp shard label add/remove, cp shard labels
│   └── memory.go               # Enhanced cp memory (add --label, recall)
└── internal/
    └── client/
        ├── search.go           # Semantic search wrapper (embed query + call function)
        ├── shards.go           # General shard CRUD (extend existing)
        ├── edges.go            # Edge CRUD operations
        └── labels.go           # Label operations
```

### Recall Flow

```go
func Recall(ctx context.Context, query string, opts RecallOpts) ([]SearchResult, error) {
    // 1. Embed the query
    embedding, err := embeddingProvider.Embed(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to embed query: %w", err)
    }

    // 2. Build filter arrays
    types := parseCommaSep(opts.Types)
    labels := parseCommaSep(opts.Labels)
    status := parseStatusFilter(opts.Status, opts.IncludeClosed)

    // 3. Call semantic_search SQL function
    rows, err := db.Query(ctx,
        "SELECT * FROM semantic_search($1, $2, $3, $4, $5, $6, $7)",
        project, embedding, types, labels, status, opts.Limit, opts.MinSimilarity,
    )

    // 4. Apply --since filter (post-query if not in SQL)
    results := filterSince(rows, opts.Since)

    return results, nil
}
```

### Edge Creation with Validation

```go
func CreateEdge(ctx context.Context, fromID, toID, edgeType string) error {
    // 1. Verify both shards exist
    from, err := GetShard(ctx, fromID)
    if err != nil {
        return fmt.Errorf("shard %s not found", fromID)
    }
    to, err := GetShard(ctx, toID)
    if err != nil {
        return fmt.Errorf("shard %s not found", toID)
    }

    // 2. Validate edge type
    if !isValidEdgeType(edgeType) {
        return fmt.Errorf("unknown edge type: %s. Valid types: %s",
            edgeType, strings.Join(validEdgeTypes, ", "))
    }

    // 3. Check for duplicate edge
    exists, err := EdgeExists(ctx, fromID, toID, edgeType)
    if exists {
        return fmt.Errorf("edge already exists: %s --%s--> %s", fromID, edgeType, toID)
    }

    // 4. For blocked-by edges, check circular dependencies
    if edgeType == "blocked-by" {
        circular, err := HasCircularDependency(ctx, fromID, toID)
        if circular {
            return fmt.Errorf("circular dependency detected")
        }
    }

    // 5. Insert edge
    _, err = db.Exec(ctx,
        "INSERT INTO edges (from_id, to_id, edge_type) VALUES ($1, $2, $3)",
        fromID, toID, edgeType,
    )
    return err
}
```

## Success Criteria

1. **`cp shard list`:** Lists any shard type with --type, --status, --label, --creator,
   --since, --search filters. Pagination via --limit and --offset.
2. **`cp shard show`:** Shows content, metadata, labels, and edge summary in one view.
3. **`cp shard create`:** Creates shards of any type with content, labels, metadata.
   Generates embedding (SPEC-1).
4. **`cp shard update`:** Updates content and/or title. Regenerates embedding.
5. **`cp shard edges`:** Shows all incoming and outgoing edges with linked shard info.
6. **`cp shard edges --follow`:** Tree view of 2-hop edge navigation.
7. **`cp shard link`:** Creates typed edges between shards. Validates both exist.
   Rejects circular blocked-by dependencies.
8. **`cp shard unlink`:** Removes edges. Confirms before removing.
9. **`cp shard label add/remove`:** Adds/removes labels. Atomic array operations.
10. **`cp shard labels`:** Lists all labels in use with counts.
11. **`cp recall`:** Semantic search across all types. Ranked by similarity.
    Supports --type, --label, --status, --since, --min-similarity, --limit filters.
12. **`cp memory add --label`:** Memory shards with labels.
13. **`cp memory add --references`:** Creates edges from memory to referenced shards.
14. **`cp memory recall`:** Semantic search limited to memory type.
15. **JSON output:** Every command supports `-o json` with structured output.
16. **Both agents:** Same commands, shared graph, agent identity from config.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| `cp shard list` no filters | All open shards, newest first, limit 20. |
| `cp shard list` no results | Empty table, exit code 0. |
| `cp shard show` non-existent ID | Error: "Shard pf-xxx not found." Exit code 1. |
| `cp shard show` on message shard | Works. Unified interface for all types. |
| `cp shard create` without --type | Error: "--type is required." |
| `cp shard create` unknown type | Warn: "Type 'foo' is not a known type. Create anyway? (y/n)." Allow custom types. |
| `cp recall` very short query (1 word) | Works but may have low discrimination. Suggest --type filter. |
| `cp recall` no results above threshold | Empty result, exit code 0. Message: "No results above 0.3 similarity." |
| `cp recall` without embedding config | Error: "Semantic search requires embedding config. Use `cp shard list --search` for text search." |
| Edge to non-existent shard | Error: "Shard pf-xxx not found." FK constraint prevents. |
| Edge already exists | Error: "Edge already exists: pf-a --implements--> pf-b." |
| Unknown edge type | Error: "Unknown edge type: foo. Valid: blocked-by, implements, references, ..." |
| `cp shard link --blocked-by` circular | Error: "Circular dependency detected: pf-a -> pf-b -> pf-a." |
| `cp shard unlink` non-existent edge | Error: "No edge of type 'implements' from pf-a to pf-b." |
| `cp shard label add` duplicate label | No-op for that label, success. Labels are a set. |
| `cp shard label remove` non-existent label | No-op, success. |
| `cp shard labels` no labels in project | Empty table, exit code 0. |
| `cp memory add --references` invalid ID | Error: "Shard pf-xxx not found." Memory still created without edge. |
| `cp memory recall` no embedding config | Falls back to text search with warning. |
| `cp shard list --since 7d` mixed with --search | Both filters apply (AND logic). |
| `--since` with bad format | Error: "Invalid duration: '7x'. Use format like '7d', '24h', or '2026-01-01'." |
| `cp shard list --type task,bug` | Returns shards of either type (OR within filter). |
| `cp shard list --label a,b` | Returns shards with ANY of those labels (OR / overlap). |
| `--limit 0` | Error: "Limit must be 1-1000." |
| `--limit 5000` | Error: "Limit must be 1-1000." |
| Very large result set | Paginate with --offset. Show "Showing 1-20 of 347 results." |

---

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
  Given: 3 tasks, 2 bugs, 1 memory in project 'test'
  When:  SELECT * FROM list_shards('test', ARRAY['task','bug'])
  Then:  Returns 5 rows (tasks + bugs)

TEST: list_shards status filter
  Given: 3 open shards, 2 closed shards
  When:  SELECT * FROM list_shards('test', NULL, ARRAY['open'])
  Then:  Returns 3 rows (open only)

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
  When:  SELECT * FROM list_shards('test', NULL, NULL, NULL, NULL, NULL, '7 days')
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

### SQL Tests: shard_detail

```
TEST: shard_detail returns shard with counts
  Given: Shard 'test-1' with 3 outgoing edges, 2 incoming edges
  When:  SELECT * FROM shard_detail('test-1')
  Then:  Returns 1 row with outgoing_edge_count=3, incoming_edge_count=2

TEST: shard_detail includes metadata
  Given: Shard with metadata = '{"priority": 2, "category": "testing"}'
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

TEST: shard_edges direction filter outgoing
  Given: Shard A with 2 outgoing, 3 incoming edges
  When:  SELECT * FROM shard_edges('A', 'outgoing')
  Then:  Returns 2 rows (outgoing only)

TEST: shard_edges direction filter incoming
  Given: Shard A with 2 outgoing, 3 incoming edges
  When:  SELECT * FROM shard_edges('A', 'incoming')
  Then:  Returns 3 rows (incoming only)

TEST: shard_edges edge type filter
  Given: Shard A with implements, references, blocked-by edges
  When:  SELECT * FROM shard_edges('A', NULL, ARRAY['implements'])
  Then:  Returns only implements edges

TEST: shard_edges multiple edge type filter
  Given: Shard A with implements, references, blocked-by edges
  When:  SELECT * FROM shard_edges('A', NULL, ARRAY['implements','references'])
  Then:  Returns implements and references edges (not blocked-by)

TEST: shard_edges includes linked shard info
  Given: Edge from A to B, B has title="Task Title", type="task", status="open"
  When:  SELECT * FROM shard_edges('A')
  Then:  Row includes linked_shard_title="Task Title", linked_shard_type="task"

TEST: shard_edges includes edge metadata
  Given: Edge with metadata = '{"change_summary": "Added diagrams"}'
  When:  SELECT * FROM shard_edges('A')
  Then:  edge_metadata contains the JSON

TEST: shard_edges no edges
  Given: Shard with no edges
  When:  SELECT * FROM shard_edges('A')
  Then:  Returns 0 rows

TEST: shard_edges ordered by type then direction
  Given: Mixed edges of various types and directions
  When:  SELECT * FROM shard_edges('A')
  Then:  Results ordered by edge_type, then direction
```

### SQL Tests: label_summary

```
TEST: label_summary counts labels
  Given: 3 shards with 'arch' label, 2 with 'deploy', 1 with both
  When:  SELECT * FROM label_summary('test')
  Then:  arch=4, deploy=3 (shard with both counted for each)

TEST: label_summary excludes closed shards
  Given: Open shard with 'arch' label, closed shard with 'arch' label
  When:  SELECT * FROM label_summary('test')
  Then:  arch=1 (closed excluded)

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
  Then:  Does not error, returns only real labels
```

### SQL Tests: memory_recall

```
TEST: memory_recall returns similar memories
  Given: 3 memory shards with known embeddings:
         mem-a: embedding close to query (similarity ~0.9)
         mem-b: embedding moderate (~0.6)
         mem-c: embedding low (~0.2)
  When:  SELECT * FROM memory_recall('test', <query_vector>)
  Then:  Returns mem-a and mem-b (above 0.3 threshold)
         mem-a ranked first

TEST: memory_recall excludes non-memory shards
  Given: Memory shard (sim 0.9), task shard (sim 0.95)
  When:  SELECT * FROM memory_recall('test', <query_vector>)
  Then:  Returns only memory shard (task excluded)

TEST: memory_recall label filter
  Given: Memory A labels=['lesson-learned'], Memory B labels=['deployment']
  When:  SELECT * FROM memory_recall('test', <vec>, ARRAY['lesson-learned'])
  Then:  Returns only memory A

TEST: memory_recall excludes closed
  Given: Open memory (sim 0.9), closed/resolved memory (sim 0.85)
  When:  SELECT * FROM memory_recall('test', <vec>)
  Then:  Returns only open memory

TEST: memory_recall respects limit
  Given: 15 memory shards all above threshold
  When:  SELECT * FROM memory_recall('test', <vec>, NULL, 5)
  Then:  Returns exactly 5 (top 5 by similarity)

TEST: memory_recall returns full content
  Given: Memory shard with 500-char content
  When:  SELECT * FROM memory_recall('test', <vec>)
  Then:  content field contains full text (not truncated)

TEST: memory_recall no results
  Given: No memory shards with embeddings
  When:  SELECT * FROM memory_recall('test', <vec>)
  Then:  Returns 0 rows (not error)
```

### Go Unit Tests: Recall Command

```
TEST: parseRecallFlags defaults
  Given: `cp recall "query"` (no flags)
  When:  parseRecallFlags()
  Then:  types=nil, labels=nil, status=["open"], limit=20, minSimilarity=0.3,
         since=nil, includeClosed=false, showSnippet=false

TEST: parseRecallFlags with type filter
  Given: `cp recall "query" --type requirement,bug`
  When:  parseRecallFlags()
  Then:  types=["requirement", "bug"]

TEST: parseRecallFlags with include-closed
  Given: `cp recall "query" --include-closed`
  When:  parseRecallFlags()
  Then:  status=nil (no status filter — include everything)

TEST: parseRecallFlags with min-similarity
  Given: `cp recall "query" --min-similarity 0.6`
  When:  parseRecallFlags()
  Then:  minSimilarity=0.6

TEST: parseRecallFlags invalid min-similarity
  Given: `cp recall "query" --min-similarity 1.5`
  When:  parseRecallFlags()
  Then:  Error: "min-similarity must be between 0.0 and 1.0"

TEST: parseRecallFlags with since duration
  Given: `cp recall "query" --since 7d`
  When:  parseRecallFlags()
  Then:  since = 7 * 24h duration

TEST: parseRecallFlags with since date
  Given: `cp recall "query" --since 2026-01-01`
  When:  parseRecallFlags()
  Then:  since represents time since that date

TEST: parseRecallFlags invalid since
  Given: `cp recall "query" --since 7x`
  When:  parseRecallFlags()
  Then:  Error: "Invalid duration: '7x'. Use format like '7d', '24h', or '2026-01-01'."

TEST: parseSinceDuration with various formats
  Given: Inputs "7d", "24h", "30m", "2w"
  When:  parseSinceDuration(input)
  Then:  Returns 7d, 24h, 30m, 14d respectively

TEST: formatRecallResults text output
  Given: 3 results with similarity, type, status, id, title
  When:  formatRecallResults(results, "text")
  Then:  Aligned table with SIMILARITY, TYPE, STATUS, ID, TITLE columns
         Similarity formatted as 2 decimal places (e.g., "0.92")

TEST: formatRecallResults with snippets
  Given: 2 results with snippets, showSnippet=true
  When:  formatRecallResults(results, "text")
  Then:  Each result followed by indented snippet line

TEST: formatRecallResults JSON output
  Given: 3 results
  When:  formatRecallResults(results, "json")
  Then:  Valid JSON array with all fields

TEST: formatRecallResults empty results
  Given: 0 results
  When:  formatRecallResults(results, "text")
  Then:  "No results above 0.3 similarity."
```

### Go Unit Tests: Shard Commands

```
TEST: parseShardListFlags defaults
  Given: `cp shard list` (no flags)
  When:  parseShardListFlags()
  Then:  types=nil, status=nil, labels=nil, creator="", search="",
         since=nil, limit=20, offset=0

TEST: parseShardListFlags with all filters
  Given: `cp shard list --type task --status open --label arch --creator agent-penfold --since 7d --limit 50`
  When:  parseShardListFlags()
  Then:  All fields populated correctly

TEST: formatShardTable text output
  Given: 3 shards with varying field lengths
  When:  formatShardTable(shards, "text")
  Then:  Aligned table with ID, TYPE, STATUS, CREATED, TITLE columns
         Long titles truncated with "..."

TEST: formatShardDetail text output
  Given: Shard with content, metadata, labels, edges
  When:  formatShardDetail(shard, edges, "text")
  Then:  Sections: header, metadata (if non-empty), content, edges

TEST: formatShardDetail with empty metadata
  Given: Shard with metadata = {}
  When:  formatShardDetail(shard, edges, "text")
  Then:  Metadata section omitted

TEST: formatEdgeTable text output
  Given: 5 edges with direction, type, linked shard info
  When:  formatEdgeTable(edges, "text")
  Then:  Aligned table with DIRECTION, EDGE TYPE, SHARD, TYPE, STATUS, TITLE

TEST: formatEdgeTree follow mode
  Given: Shard with 3 outgoing edges, each linked shard has 1-2 edges
  When:  formatEdgeTree(shard, edges, "text")
  Then:  Tree view with proper indentation and branch characters

TEST: validateEdgeType valid
  Given: "implements"
  When:  validateEdgeType("implements")
  Then:  Returns nil

TEST: validateEdgeType invalid
  Given: "foo"
  When:  validateEdgeType("foo")
  Then:  Returns error listing valid types

TEST: validEdgeTypes includes all known types
  Given: validEdgeTypes constant
  Then:  Contains: blocked-by, blocks, child-of, discovered-from, extends,
         has-artifact, implements, parent, previous-version, references,
         relates-to, replies-to, triggered-by

TEST: parseLabelArgs valid
  Given: ["architecture", "pipeline"]
  When:  parseLabelArgs(args)
  Then:  Returns ["architecture", "pipeline"]

TEST: parseLabelArgs empty
  Given: []
  When:  parseLabelArgs(args)
  Then:  Returns error: "at least one label required"

TEST: formatLabelSummary text output
  Given: 5 labels with counts
  When:  formatLabelSummary(labels, "text")
  Then:  Aligned table with LABEL, COUNT columns
```

### Go Unit Tests: Enhanced Memory

```
TEST: parseMemoryAddFlags with labels
  Given: `cp memory add "text" --label lesson-learned,pipeline`
  When:  parseMemoryAddFlags()
  Then:  labels=["lesson-learned", "pipeline"]

TEST: parseMemoryAddFlags with references
  Given: `cp memory add "text" --references pf-bug-03,pf-req-01`
  When:  parseMemoryAddFlags()
  Then:  references=["pf-bug-03", "pf-req-01"]

TEST: parseMemoryRecallFlags defaults
  Given: `cp memory recall "query"` (no flags)
  When:  parseMemoryRecallFlags()
  Then:  labels=nil, limit=10, minSimilarity=0.3

TEST: formatMemoryRecall text output
  Given: 3 memory results with similarity, id, created_at, content
  When:  formatMemoryRecall(results, "text")
  Then:  Table with SIMILARITY, ID, CREATED, CONTENT columns
         Content truncated to ~80 chars with "..."
```

### Integration Tests: Recall

```
TEST: recall finds semantically similar shards
  Given: Shard created with "Nomad deployment failed because allocation didn't restart"
  When:  `cp recall "deployment problems"`
  Then:  Shard appears in results with similarity > 0.5

TEST: recall type filter works
  Given: Task shard and bug shard both about "timeout"
  When:  `cp recall "timeout" --type bug`
  Then:  Only bug shard returned

TEST: recall with no matches
  Given: Shards about software development
  When:  `cp recall "medieval castle architecture"`
  Then:  Empty result set, exit code 0, message about no results

TEST: recall --include-closed shows closed shards
  Given: Open bug about timeout, closed task about timeout
  When:  `cp recall "timeout"`
  Then:  Only open bug returned
  When:  `cp recall "timeout" --include-closed`
  Then:  Both returned

TEST: recall --min-similarity filters low matches
  Given: 3 shards with varying similarity to query
  When:  `cp recall "query" --min-similarity 0.8`
  Then:  Only high-similarity results returned

TEST: recall --since filters by date
  Given: Shard created 2 days ago, shard created 10 days ago
  When:  `cp recall "query" --since 7d`
  Then:  Only recent shard returned

TEST: recall --limit limits results
  Given: 10 shards all matching query
  When:  `cp recall "query" --limit 3`
  Then:  Exactly 3 results

TEST: recall --label filters by label
  Given: Shard with label='architecture', shard without
  When:  `cp recall "query" --label architecture`
  Then:  Only labeled shard returned

TEST: recall JSON output
  Given: Matching shards exist
  When:  `cp recall "query" -o json`
  Then:  Valid JSON array with id, title, type, status, similarity fields

TEST: recall without embedding config
  Given: No embedding section in config
  When:  `cp recall "query"`
  Then:  Exit code 1, error about missing embedding config
         Suggests text search alternative
```

### Integration Tests: Shard List/Show

```
TEST: shard list with type filter
  Given: 3 tasks, 2 bugs in project
  When:  `cp shard list --type task`
  Then:  3 rows, all type=task

TEST: shard list with status filter
  Given: 2 open, 3 closed shards
  When:  `cp shard list --status open`
  Then:  2 rows, all status=open

TEST: shard list with label filter
  Given: 2 shards with 'arch' label, 3 without
  When:  `cp shard list --label arch`
  Then:  2 rows

TEST: shard list with text search
  Given: Shards with various content
  When:  `cp shard list --search "timeout"`
  Then:  Only shards matching "timeout" in tsvector

TEST: shard list with combined filters
  Given: Various shards
  When:  `cp shard list --type task --status open --label pipeline`
  Then:  Only open tasks with pipeline label

TEST: shard list empty result
  Given: No matching shards
  When:  `cp shard list --type nonexistent`
  Then:  Empty table, exit code 0

TEST: shard list pagination
  Given: 30 shards
  When:  `cp shard list --limit 10`
  Then:  10 rows, shows pagination info
  When:  `cp shard list --limit 10 --offset 10`
  Then:  Next 10 rows

TEST: shard show full detail
  Given: Shard with content, metadata, labels, edges
  When:  `cp shard show <id>`
  Then:  Output contains all sections: header, metadata, content, edges

TEST: shard show JSON output
  Given: Shard exists
  When:  `cp shard show <id> -o json`
  Then:  Valid JSON with all fields including metadata and edge arrays

TEST: shard show non-existent
  Given: No shard with given ID
  When:  `cp shard show nonexistent`
  Then:  Exit code 1, error "Shard nonexistent not found"
```

### Integration Tests: Shard Create/Update

```
TEST: shard create with all options
  Given: Valid config
  When:  `cp shard create --type design --title "Test" --body "Content" --label arch,pipeline --meta '{"scope":"system"}'`
  Then:  Exit code 0, returns shard ID
         `cp shard show <id>` shows all fields correctly
         Shard has embedding (non-NULL)

TEST: shard create without type fails
  Given: Valid config
  When:  `cp shard create --title "Test" --body "Content"`
  Then:  Exit code 1, error "--type is required"

TEST: shard create from file
  Given: File with content at /tmp/test-body.md
  When:  `cp shard create --type doc --title "Test" --body-file /tmp/test-body.md`
  Then:  Shard content matches file content

TEST: shard update content
  Given: Shard with content "Original"
  When:  `cp shard update <id> --body "Updated"`
  Then:  Content is "Updated"
         Embedding regenerated (different from original)

TEST: shard update title
  Given: Shard with title "Original Title"
  When:  `cp shard update <id> --title "New Title"`
  Then:  Title is "New Title"

TEST: shard close and reopen
  Given: Open shard
  When:  `cp shard close <id>`
  Then:  Status = closed
  When:  `cp shard reopen <id>`
  Then:  Status = open
```

### Integration Tests: Edges

```
TEST: shard edges shows all edges
  Given: Shard A with outgoing edge to B, incoming edge from C
  When:  `cp shard edges A`
  Then:  2 rows showing both edges with direction, type, linked shard info

TEST: shard edges direction filter
  Given: Shard A with 2 outgoing, 3 incoming edges
  When:  `cp shard edges A --direction outgoing`
  Then:  2 rows (outgoing only)

TEST: shard edges type filter
  Given: Shard A with implements, references, blocked-by edges
  When:  `cp shard edges A --edge-type implements`
  Then:  Only implements edges shown

TEST: shard link creates edge
  Given: Shard A (task) and shard B (requirement)
  When:  `cp shard link A --implements B`
  Then:  `cp shard edges A` shows implements edge to B

TEST: shard link validates shards exist
  Given: Shard A exists, shard B does not
  When:  `cp shard link A --references nonexistent`
  Then:  Exit code 1, error "Shard nonexistent not found"

TEST: shard link rejects duplicate edge
  Given: Edge already exists from A --implements--> B
  When:  `cp shard link A --implements B`
  Then:  Exit code 1, error "Edge already exists"

TEST: shard link rejects circular blocked-by
  Given: A blocked-by B
  When:  `cp shard link B --blocked-by A`
  Then:  Exit code 1, error "Circular dependency detected"

TEST: shard unlink removes edge
  Given: Edge from A --implements--> B
  When:  `cp shard unlink A --implements B`
  Then:  `cp shard edges A` no longer shows that edge

TEST: shard unlink non-existent edge
  Given: No edge from A to B
  When:  `cp shard unlink A --implements B`
  Then:  Exit code 1, error "No edge of type 'implements' from A to B"

TEST: shard edges follow mode
  Given: A --implements--> B, B --blocked-by--> C
  When:  `cp shard edges A --follow`
  Then:  Tree output showing A -> B -> C with proper formatting
```

### Integration Tests: Labels

```
TEST: shard label add
  Given: Shard with labels=['arch']
  When:  `cp shard label add <id> pipeline deployment`
  Then:  Labels = ['arch', 'pipeline', 'deployment']

TEST: shard label add duplicate is no-op
  Given: Shard with labels=['arch', 'pipeline']
  When:  `cp shard label add <id> arch`
  Then:  Labels = ['arch', 'pipeline'] (unchanged)

TEST: shard label remove
  Given: Shard with labels=['arch', 'pipeline', 'deployment']
  When:  `cp shard label remove <id> pipeline`
  Then:  Labels = ['arch', 'deployment']

TEST: shard label remove non-existent is no-op
  Given: Shard with labels=['arch']
  When:  `cp shard label remove <id> nonexistent`
  Then:  Labels = ['arch'] (unchanged), exit code 0

TEST: shard labels summary
  Given: Various shards with labels
  When:  `cp shard labels`
  Then:  Table showing each label with count, ordered by count DESC

TEST: shard list --label filter works
  Given: 5 shards, 2 with 'arch' label
  When:  `cp shard list --label arch`
  Then:  Returns 2 shards
```

### Integration Tests: Enhanced Memory

```
TEST: memory add with labels
  Given: Valid config
  When:  `cp memory add "Test memory" --label lesson-learned,pipeline`
  Then:  `cp shard show <id>` shows labels = ['lesson-learned', 'pipeline']

TEST: memory add with references
  Given: Existing shard pf-bug-03
  When:  `cp memory add "Related to bug" --references pf-bug-03`
  Then:  Memory created
         `cp shard edges <memory-id>` shows references edge to pf-bug-03

TEST: memory add with references to non-existent shard
  Given: No shard with id 'nonexistent'
  When:  `cp memory add "Test" --references nonexistent`
  Then:  Memory created (still works)
         Warning: "Shard nonexistent not found. Memory created without edge."

TEST: memory recall semantic search
  Given: Memory about "Nomad deployment unreliable"
  When:  `cp memory recall "deployment problems"`
  Then:  Memory appears with similarity score

TEST: memory recall only returns memories
  Given: Memory about timeout, task about timeout
  When:  `cp memory recall "timeout"`
  Then:  Only memory returned (task excluded)

TEST: memory recall with label filter
  Given: Memory A label='lesson-learned', Memory B label='pipeline'
  When:  `cp memory recall "query" --label lesson-learned`
  Then:  Only Memory A returned

TEST: memory list with label filter
  Given: 3 memories, 1 with 'pipeline' label
  When:  `cp memory list --label pipeline`
  Then:  Returns 1 memory

TEST: memory list with since filter
  Given: Memory from 2 days ago, memory from 10 days ago
  When:  `cp memory list --since 7d`
  Then:  Returns only recent memory
```
