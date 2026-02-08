# SPEC-6: Hierarchical Memory

**Status:** Draft
**Depends on:** SPEC-5 (enhanced memory), SPEC-1 (semantic search)
**Blocks:** Nothing

---

## Goal

Give AI agents access to an effectively infinite knowledge base via lazy-loaded
hierarchical memory. Root memories provide orientation and contain inline pointers
to child memories. Agents load the first layer on startup, then follow pointers
on-demand — only pulling detail when they need it.

The hierarchy is self-describing: each parent memory contains a structured JSON
block summarizing what its children contain and *when you'd need them*. This means
an agent can navigate the tree without querying edges — the pointers are right there
in the content.

Access telemetry tracks which memories get read and at what depth, enabling periodic
review to promote frequently-accessed deep memories upward and keep the tree efficient.

## What Exists

- `parent_id` column on shards table (indexed, FK to shards)
- `child-of` / `parent` edge types in the graph
- `penf memory add/list/search/resolve/defer` — flat memory, text search
- SPEC-5 `cp memory` — labels, references, semantic recall (flat)
- No hierarchy, no sub-memories, no access tracking

## What to Build

1. **Sub-memory pointer block** — structured JSON section in parent content
2. **`cp memory add-sub`** — create child + AI-generated summary + update parent
3. **`cp memory delete`** — remove shard + remove pointer from parent (atomic)
4. **`cp memory move`** — re-parent a memory (update old parent, new parent, shard)
5. **`cp memory promote`** — move a child up one level
6. **`cp memory show --depth N`** — expand children inline
7. **`cp memory tree`** — full hierarchy view with access stats
8. **`cp memory sync`** — reconcile pointer blocks from graph (safety net)
9. **`cp memory hot`** — most-accessed memories by depth (promotion candidates)
10. **Access telemetry** — track reads in shard metadata

## Data Model

### Parent-Child Relationship

The `parent_id` column on the shards table is the source of truth for the tree
structure. When a sub-memory is created, its `parent_id` is set to the parent
memory's ID. A `child-of` edge is also created for graph navigation consistency.

```
parent memory (pf-aa1)
  ├── parent_id: NULL (root)
  ├── content: "...main text...\n[sub-memories block]"
  │
  └── child memory (pf-aa2)
        ├── parent_id: pf-aa1
        ├── content: "...child text..."
        └── child-of edge: pf-aa2 → pf-aa1
              └── edge metadata: {"summary": "If deploy succeeds but service unchanged"}
```

### Summary Storage (Dual Write)

Trigger-phrase summaries are stored in **two places**:

1. **Parent's sub-memories pointer block** (in `shards.content`) — for inline agent
   navigation. Agents read the parent content and see the summaries in-context.
2. **`child-of` edge metadata** (in `edges.metadata` JSONB) — for SQL-queryable access.
   `memory_tree()` joins edges to return summaries without parsing content.

Both are written in the same transaction. The pointer block is the primary agent-facing
interface. The edge metadata enables efficient tree queries. `cp memory sync` reconciles
both if they drift.

### Sub-Memory Pointer Block

Each parent memory's content ends with a structured JSON block containing
summaries of its children. The block is delimited by markers that the system
can reliably parse.

**Format:**

```
[main memory prose — the actual knowledge content]

<!-- sub-memories -->
[
  {"id": "pf-aa2", "title": "Troubleshooting", "summary": "If deploy succeeds but service unchanged, or allocations don't restart"},
  {"id": "pf-aa5", "title": "Rollback", "summary": "Steps to revert a bad deploy including Nomad job rollback"},
  {"id": "pf-aa6", "title": "Version History", "summary": "What version is running where, how to check deployed commits"}
]
<!-- /sub-memories -->
```

**Design decisions:**

- **JSON array** — one object per child, machine-parseable, unambiguous for CRUD
- **HTML comment delimiters** — `<!-- sub-memories -->` / `<!-- /sub-memories -->` are
  unlikely to appear in normal content, visually distinct, and the AI reads them fine
- **Summary is a trigger phrase** — describes *when* you'd need the child, not just
  *what* it contains. "If deploy succeeds but service unchanged" is better than
  "Deployment troubleshooting notes"
- **Title is short** — the display label, used in `tree` output
- **Ordering** — array order matches display order (insertion order by default). Reordering is out of scope for this spec — future work if needed.

### Rendered Output

When `cp memory show` displays the memory, the pointer block is rendered as a
readable list:

```
Deployment
──────────
ID:       pf-aa1
Type:     memory
Status:   open
Children: 3

[main memory prose content]

Sub-memories:
  pf-aa2  Troubleshooting     If deploy succeeds but service unchanged, or allocations don't restart
  pf-aa5  Rollback            Steps to revert a bad deploy including Nomad job rollback
  pf-aa6  Version History     What version is running where, how to check deployed commits
```

### Access Telemetry

Every `cp memory show` call increments telemetry counters in the shard's metadata:

```json
{
  "access_count": 14,
  "last_accessed": "2026-02-07T15:30:00Z",
  "access_log": [
    {"at": "2026-02-07T15:30:00Z", "by": "agent-penfold", "depth": 2},
    {"at": "2026-02-07T14:00:00Z", "by": "agent-penfold", "depth": 0}
  ]
}
```

- `access_count` — total reads (simple counter, always incremented)
- `last_accessed` — timestamp of most recent read
- `access_log` — rolling window of last 50 accesses with agent and depth context
- `depth` — how deep in the tree this memory sits (0 = root, 1 = child, etc.)

The `access_log` has a 50-entry cap. Oldest entries are dropped when the cap is hit.
This keeps the metadata from growing unbounded while retaining enough history for
weekly review.

### Data Flow

#### `parent_id` (shard column)

1. **WHO writes it?** `cp memory add-sub` (sets on child creation), `cp memory move` (updates), `cp memory promote` (updates)
2. **WHEN is it written?** On child creation, move, or promote.
3. **WHERE is it stored?** `shards.parent_id` column (indexed FK)
4. **WHO reads it?** `memory_tree()`, `memory_children()`, `memory_path()`, `memory_hot()`, `cp memory sync`, `cp memory delete` (to find parent for pointer removal)
5. **HOW is it queried?** Direct column read. `WHERE parent_id = $1` for children. `WHERE parent_id IS NULL` for roots.
6. **WHAT decisions does it inform?** Tree structure, navigation, sync validation, move/promote targets.
7. **DOES it go stale?** No — updated atomically in transactions. If a parent shard is deleted externally, `parent_id` becomes a dangling FK. `cp memory sync` detects this and can fix it.

#### Sub-memory pointer block (in shard content)

1. **WHO writes it?** `cp memory add-sub` (appends entry), `cp memory delete` (removes entry), `cp memory move` (removes from old parent, adds to new), `cp memory promote` (same as move), `cp memory sync` (reconciles)
2. **WHEN is it written?** On any child mutation (add, delete, move, promote) or manual sync.
3. **WHERE is it stored?** Embedded in `shards.content` between `<!-- sub-memories -->` and `<!-- /sub-memories -->` delimiters. JSON array format.
4. **WHO reads it?** `cp memory show` (renders as readable list), `cp memory sync` (validates), AI agents (read pointers in-context to decide what to load).
5. **HOW is it queried?** Parsed by Go code `ParseSubMemories()` function. Not SQL-queryable (embedded in content). This is intentional — the block exists so the AI agent can read it in-context without querying edges.
6. **WHAT decisions does it inform?** Agent navigation — which sub-memories to load. Trigger summaries tell the agent WHEN they'd need the child.
7. **DOES it go stale?** Yes — if edges/parent_id are modified without updating the pointer block (e.g., external SQL manipulation). `cp memory sync` is the safety net.

#### Summary in edge metadata (child-of edge)

1. **WHO writes it?** `cp memory add-sub` (creates edge with summary), `cp memory move` (deletes old edge, creates new with same summary), `cp memory promote` (same as move), `cp memory sync` (reconciles).
2. **WHEN is it written?** On child creation, move, promote, or sync. Always in the same transaction as the pointer block update.
3. **WHERE is it stored?** `edges.metadata` JSONB on the `child-of` edge: `{"summary": "trigger phrase"}`.
4. **WHO reads it?** `memory_tree()` SQL function (LEFT JOIN edges), `cp memory tree`, `cp memory hot`.
5. **HOW is it queried?** `e.metadata->>'summary'` via LEFT JOIN in `memory_tree()`. SQL-queryable without parsing content.
6. **WHAT decisions does it inform?** Tree display with summaries, JSON API output. Avoids Go-side content parsing for tree queries.
7. **DOES it go stale?** Can drift from the pointer block if one is updated without the other. Both are always written in the same transaction, so drift only occurs from external SQL manipulation. `cp memory sync` reconciles both.

#### Access telemetry (in shard metadata)

1. **WHO writes it?** `memory_touch()` SQL function, called by `cp memory show`.
2. **WHEN is it written?** On every `cp memory show` call (including when expanded via `--depth`).
3. **WHERE is it stored?** `shards.metadata` JSONB: `access_count` (int), `last_accessed` (timestamp), `access_log` (array of {at, by, depth}).
4. **WHO reads it?** `cp memory tree --stats`, `cp memory hot`, `cp memory show` (displays count in header).
5. **HOW is it queried?** `metadata->>'access_count'` for count. `metadata->'access_log'` for full log. Both via direct metadata reads in SQL functions.
6. **WHAT decisions does it inform?** Promotion candidates (hot memories), tree efficiency review, usage patterns.
7. **DOES it go stale?** The `access_log` has a 50-entry rolling window. Old entries are dropped. `access_count` is monotonically increasing. No decay — this is intentional. A memory that was heavily accessed a month ago but not since still shows high count. `last_accessed` indicates recency.

### Concurrency

**Concurrent `add-sub` to same parent:** Two agents adding sub-memories to the same parent
simultaneously. Both read the current pointer block, both append, second commit overwrites
the first's entry.

**Strategy: `SELECT ... FOR UPDATE`** on parent shards. All commands that modify pointer blocks
(`add-sub`, `delete`, `move`, `promote`, `sync`) acquire `FOR UPDATE` locks on affected parent
shards within their transaction. This serializes concurrent writes to the same parent's pointer
block. Same pattern as SPEC-4 knowledge document updates.

For `move` and `promote` which touch two parents (old and new), locks are acquired in shard ID
order (lexicographic) to prevent deadlocks.

**Concurrent `memory_touch`:** Multiple agents reading the same memory simultaneously.
Each calls `memory_touch()` which does a single `UPDATE ... SET metadata = jsonb_set(...)`.
PostgreSQL MVCC means concurrent touches may lose `access_log` entries (last writer wins
for the array), but `access_count` increment is safe because `jsonb_set` reads the current
value inside the UPDATE. This is an accepted trade-off — the access log is approximate,
not authoritative.

**Delete during read:** If one agent deletes a memory while another is reading it,
the reader's `cp memory show` may fail with "not found." This is expected behavior.

## AI-Assisted Summary Creation

The hardest part of this system is writing good summaries. Bad summaries make
child memories invisible — the agent won't know to follow the link. The `add-sub`
command uses AI to generate the summary and optionally update the parent.

### AI Provider

Summary generation uses **Google Gemini (gemini-2.0-flash)** — the same vendor as
the embedding provider, reusing the existing `GOOGLE_API_KEY`. Flash is cheap and
fast enough for trigger-phrase summaries.

**Config** (in `~/.cp/config.yaml`, alongside the existing `embedding:` section):

```yaml
embedding:
  provider: google
  model: gemini-embedding-001
  api_key_env: GOOGLE_API_KEY

generation:
  provider: google
  model: gemini-2.0-flash
  api_key_env: GOOGLE_API_KEY   # Same key as embedding
```

**Interface** (new `Generator` in `internal/generation/` — separate from embedding):

```go
// Generator generates text from a prompt using an LLM.
type Generator interface {
    Generate(ctx context.Context, prompt string) (string, error)
}

type GenerationConfig struct {
    Provider  string `yaml:"provider"`   // "google"
    Model     string `yaml:"model"`      // "gemini-2.0-flash"
    APIKeyEnv string `yaml:"api_key_env"`
}
```

Do not unify with the embedding `Provider` interface — embedding and generation are
different shapes. One interface, one Google implementation, no extra abstraction.

**Timeout/retry:** 30-second timeout, no retry on failure. If summary generation
fails, the `add-sub` command fails — don't create a child with no summary. The
user can retry, or use `--no-ai --summary "..."` to bypass.

### Workflow

```
User runs: cp memory add-sub pf-aa1 --title "Troubleshooting" --body-file troubleshoot.md
                                │
                                ▼
                    ┌──────────────────────┐
                    │  1. Read child body   │
                    │  2. Read parent body   │
                    └──────────┬───────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │  3. AI generates:     │
                    │     - Trigger summary │
                    │     - Parent review   │
                    └──────────┬───────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │  4. Present for       │
                    │     approval          │
                    └──────────┬───────────┘
                               │
                          ┌────┴────┐
                          │ Approve │ Edit │ Cancel │
                          └────┬────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │  5. Atomic commit:    │
                    │     - Create child    │
                    │     - Set parent_id   │
                    │     - Create edge     │
                    │     - Update parent   │
                    │       content         │
                    └──────────────────────┘
```

### AI Summary Prompt

The `add-sub` command sends the following to the AI for summary generation:

```
You are writing a pointer summary for a hierarchical memory system used by AI agents.

PARENT MEMORY (ID: {parent_id}):
---
{parent_content without sub-memories block}
---

NEW CHILD MEMORY (title: {child_title}):
---
{child_content}
---

Generate TWO things:

1. TRIGGER SUMMARY (max 120 chars):
   Write a one-line summary that tells the AI agent WHEN they would need to read
   this child memory. Focus on symptoms, situations, or questions — not just what
   the memory contains.

   Good: "If deploy succeeds but service unchanged, or allocations don't restart"
   Bad:  "Deployment troubleshooting information"

   Good: "When entity counts look wrong or junk entities appear"
   Bad:  "Entity quality issues and fixes"

2. PARENT REVIEW:
   Does the parent memory's prose need updating given this new child? If the parent
   says something that the child contradicts, clarifies, or significantly extends,
   suggest specific edits. If the parent is fine as-is, say "No changes needed."

   Example: If parent says "deployment is straightforward" but the child documents
   5 troubleshooting scenarios, suggest softening the parent text.

Respond as JSON:
{
  "summary": "trigger phrase here",
  "parent_needs_update": true/false,
  "parent_edits": "description of suggested edits, or null"
}
```

### Approval Flow

After AI generation, the CLI presents:

```
Sub-memory: "Troubleshooting" → parent "Deployment" (pf-aa1)

Summary: If deploy succeeds but service unchanged, or allocations don't restart

Parent update suggested (review only — not auto-applied):
  The AI suggests: "Deployment is straightforward via Nomad."
  → "Deployment uses Nomad. See troubleshooting sub-memory for common issues."

[A]pprove summary  [E]dit summary  [C]ancel
```

Options:
- **Approve** — accept trigger summary, commit. Parent edit suggestion is displayed for
  informational purposes only — the user can manually apply it afterward if desired.
- **Edit** — opens summary in `$EDITOR` for manual tweaking, then commit
- **Cancel** — abort, nothing created

**Parent edits are suggestions only.** The AI's `parent_edits` field is displayed in the
approval prompt so the user is aware, but the system never auto-applies changes to the
parent's prose. This avoids the complexity of diff/patch logic and the risk of the AI
making unwanted edits to carefully written content.

### Non-Interactive Mode

For scripted/automated use:

```bash
# Skip AI, provide summary directly
cp memory add-sub pf-aa1 --title "Troubleshooting" \
    --body-file troubleshoot.md \
    --summary "If deploy succeeds but service unchanged" \
    --no-ai

# Accept AI summary without review (parent edit suggestion is silently skipped)
cp memory add-sub pf-aa1 --title "Troubleshooting" \
    --body-file troubleshoot.md \
    --auto-approve
```

When `--auto-approve` is used, the AI-generated trigger summary is accepted and the parent
edit suggestion is discarded. This is intentional — auto-approve is for scripted/batch use
where only the pointer summary matters.

## CLI Surface

### `cp memory add-sub` — Create Sub-Memory

```bash
# Interactive (AI-assisted)
cp memory add-sub <parent-id> --title "Troubleshooting" \
    --body "If the service fails to start after deploy..."

# From file
cp memory add-sub <parent-id> --title "Troubleshooting" \
    --body-file troubleshoot.md

# With labels
cp memory add-sub <parent-id> --title "Troubleshooting" \
    --body-file troubleshoot.md \
    --label deployment,nomad

# Non-interactive (provide summary, skip AI)
cp memory add-sub <parent-id> --title "Troubleshooting" \
    --body-file troubleshoot.md \
    --summary "If deploy succeeds but service unchanged" \
    --no-ai

# Auto-approve AI suggestion
cp memory add-sub <parent-id> --title "Troubleshooting" \
    --body-file troubleshoot.md \
    --auto-approve
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--title` | Yes | — | Child memory title |
| `--body` | One required | — | Child content (inline) |
| `--body-file` | One required | — | Child content (from file) |
| `--label` | No | — | Labels (comma-separated or repeatable) |
| `--summary` | No (unless `--no-ai`) | — | Manual trigger summary (skips AI generation) |
| `--no-ai` | No | false | Skip AI summary generation. Requires `--summary`. |
| `--auto-approve` | No | false | Accept AI suggestion without interactive review |
| `-o` | No | text | Output format: text, json |

**What it does:**

Pre-transaction (no locks held):
1. Verify parent exists and is type 'memory'. Error if not.
1b. Check parent depth (via `memory_path()`). If depth >= 5, warn: "This memory will be
   at depth N. Deep hierarchies increase access latency. Continue? (y/n)". Skip in `--auto-approve`.
2. Read child content from `--body` or `--body-file`. Error if neither.
3. If `--no-ai` and no `--summary`, error: "--summary required when using --no-ai."
4. If not `--no-ai`: call AI (Gemini Flash, 30s timeout) to generate trigger summary
   and parent review suggestion.
5. If not `--auto-approve` and not `--no-ai`: present for interactive approval. Display
   parent edit suggestion for informational purposes (never auto-applied).
6. If `--auto-approve`: accept AI summary, discard parent edit suggestion silently.
7. Generate embedding for child content (HTTP to Gemini embedding API).

Atomic transaction:
8. In a transaction:
   a. `SELECT ... FOR UPDATE` on parent shard (prevents concurrent pointer block corruption)
   b. Create child shard via `tx.QueryRow(ctx, "SELECT create_shard($1,...)", ...)` — call
      the `create_shard()` SQL function directly on the transaction handle
   c. Store pre-computed embedding on child shard
   d. Create `child-of` edge from child to parent, with summary in edge metadata:
      `{"summary": "trigger phrase here"}`
   e. Append entry to parent's sub-memories JSON block
9. Return child shard ID.

**Why embedding is outside the transaction:** Embedding requires an HTTP call to
Google's API. Holding a `FOR UPDATE` lock during an external call risks lock contention
and timeouts. Pre-computing the embedding keeps the transaction fast (SQL only).

**Output (text):**
```
Created sub-memory pf-aa2 "Troubleshooting" under pf-aa1 "Deployment"
Summary: If deploy succeeds but service unchanged, or allocations don't restart
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-aa2",
  "title": "Troubleshooting",
  "parent_id": "pf-aa1",
  "summary": "If deploy succeeds but service unchanged, or allocations don't restart"
}
```

### `cp memory delete` — Delete Memory

```bash
cp memory delete <id>

# Force delete (skip confirmation)
cp memory delete <id> --force

# Delete including all children (recursive)
cp memory delete <id> --recursive
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--force` | No | false | Skip confirmation prompt |
| `--recursive` | No | false | Delete all descendants too |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. If memory has children and `--recursive` not set → error
2. If memory has a parent → `SELECT ... FOR UPDATE` on parent shard, then remove
   entry from parent's sub-memories block
3. Delete the shard (CASCADE removes edges)
4. If `--recursive` → delete all descendants depth-first

**Confirmation prompt (without --force):**
```
Delete memory "Troubleshooting" (pf-aa2)?
  Parent: "Deployment" (pf-aa1) — pointer will be removed
  Children: 2 (use --recursive to delete them too)
[y/N]
```

**Output (text):**
```
Deleted memory "Troubleshooting" (pf-aa2)
Removed pointer from parent "Deployment" (pf-aa1)
```

**JSON output (`-o json`):**
```json
{
  "deleted": ["pf-aa2"],
  "parent_updated": "pf-aa1"
}
```

For `--recursive`:
```json
{
  "deleted": ["pf-aa2", "pf-aa3", "pf-aa4"],
  "parent_updated": "pf-aa1"
}
```

### `cp memory move` — Re-Parent Memory

```bash
cp memory move <id> <new-parent-id>

# Move to root (no parent)
cp memory move <id> --root
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--root` | No | false | Move to root (no parent). Mutually exclusive with new-parent-id arg. |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Verify target is not self: error "Cannot move memory to itself."
2. Verify target is not a descendant of this memory (call `memory_path()` from target
   and check if this memory's ID appears). Error "Cannot move to own descendant (would create cycle)."
3. `SELECT ... FOR UPDATE` on all affected parent shards (old parent, new parent).
   Lock in shard ID order to prevent deadlocks.
4. Read existing summary from old parent's sub-memories block (preserve it for new parent)
5. Remove entry from old parent's sub-memories block
6. Add entry (with preserved summary) to new parent's sub-memories block
7. Update shard's `parent_id`
8. Delete old `child-of` edge, create new `child-of` edge with same summary in edge metadata
9. If `--root`, set parent_id to NULL and remove edge. Skip steps 6-8.

**Output (text):**
```
Moved "Troubleshooting" (pf-aa2) from "Deployment" (pf-aa1) to "Infrastructure" (pf-xx1)
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-aa2",
  "old_parent": "pf-aa1",
  "new_parent": "pf-xx1"
}
```

### `cp memory promote` — Move Up One Level

```bash
cp memory promote <id>
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
- If memory is depth 2 (grandchild), becomes depth 1 (child of grandparent)
- If memory is depth 1 (child), becomes depth 0 (root)
- Equivalent to `cp memory move <id> <grandparent-id>` or `cp memory move <id> --root`
- Uses `SELECT ... FOR UPDATE` on affected parent shards (same locking as `move`)

**Output (text):**
```
Promoted "Troubleshooting" (pf-aa2) from depth 1 to root
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-aa2",
  "old_parent": "pf-aa1",
  "new_parent": null,
  "new_depth": 0
}
```

### `cp memory show` — Show Memory (New Command)

**Note:** This is a new command introduced by SPEC-6. There is no `cp memory show` in
SPEC-5 (SPEC-5 provides `cp shard show` for generic shard viewing). `cp memory show` is
memory-specific: it renders the sub-memories pointer block, tracks access telemetry, and
supports `--depth` expansion. For non-memory shards, use `cp shard show`.

```bash
# Show memory with sub-memory pointers rendered
cp memory show <id>

# Expand children inline (pull whole subtree)
cp memory show <id> --depth 1    # show + immediate children content
cp memory show <id> --depth 2    # show + children + grandchildren

# JSON output
cp memory show <id> -o json
cp memory show <id> --depth 1 -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--depth` | No | 0 | Expand children inline to this depth. 0=pointers only, 1=children, 2=grandchildren. Max 5. |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Fetch shard and verify it exists. Error if not found.
2. Call `memory_touch(id, agent, depth)` to record access.
3. Parse sub-memories block from content.
4. If `--depth > 0`, call `memory_children()` and recursively fetch content up to `--depth`. Each child also triggers `memory_touch()`.
5. If `--depth > 0` and total descendants > 50, warn: "Expanding N memories. This may produce large output."
6. Format as rendered text or JSON.

**JSON output (`-o json`, depth 0):**
```json
{
  "id": "pf-aa1",
  "title": "Deployment",
  "content": "Overview of how we deploy...",
  "labels": ["infrastructure", "deployment"],
  "access_count": 14,
  "last_accessed": "2026-02-07T15:30:00Z",
  "children": [
    {"id": "pf-aa2", "title": "Troubleshooting", "summary": "If deploy succeeds but service unchanged"},
    {"id": "pf-aa5", "title": "Rollback", "summary": "Steps to revert a bad deploy"},
    {"id": "pf-aa6", "title": "Version History", "summary": "What version is running where"}
  ]
}
```

**JSON output (`-o json`, depth 1):**
Same as above but each child object includes full `content`, `labels`, `access_count`, and nested `children` array.

**Default output (depth 0):**
```
Deployment
──────────
ID:       pf-aa1
Type:     memory
Status:   open
Labels:   infrastructure, deployment
Children: 3
Accessed: 14 times (last: 2h ago)

Overview of how we deploy and manage the Penfold system. Nomad-based,
Gateway + Worker split. Binary builds happen on dev01, deploy via Nomad
job spec update...

Sub-memories:
  pf-aa2  Troubleshooting     If deploy succeeds but service unchanged
  pf-aa5  Rollback            Steps to revert a bad deploy
  pf-aa6  Version History     What version is running where
```

**Depth 1 output:**
```
Deployment
──────────
[...same header...]

[...parent content...]

Sub-memories (expanded):

  ┌─ pf-aa2: Troubleshooting
  │  Labels: deployment, nomad
  │  Children: 2
  │
  │  If the gateway reports a new version but behavior hasn't changed,
  │  the worker likely wasn't redeployed. Check with:
  │    nomad job status penfold-worker
  │  [...]
  │
  │  Sub-memories:
  │    pf-aa3  Nomad Stuck       When allocation shows "running" but binary is old
  │    pf-aa4  Gateway vs Worker When to deploy which component
  │
  ├─ pf-aa5: Rollback
  │  [...]
  │
  └─ pf-aa6: Version History
     [...]
```

### `cp memory tree` — Hierarchy View

```bash
# Full tree from all roots
cp memory tree

# Tree from specific root
cp memory tree <root-id>

# With access stats
cp memory tree --stats

# Only show N levels deep
cp memory tree --max-depth 2
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--stats` | No | false | Show access counts and promotion markers |
| `--max-depth` | No | unlimited | Max tree depth to display (1-10) |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Call `memory_tree(project, root_id)`.
2. If `--max-depth`, filter results to `depth <= max_depth`.
3. Format as indented tree (text) or nested JSON.
4. For `--stats`: the ★ promotion marker is computed Go-side by comparing each node's
   `access_count` to its parent's `access_count` in the result set.

**JSON output (`-o json`):**
```json
[
  {
    "id": "pf-aa1", "title": "deployment", "depth": 0, "access_count": 14,
    "children": [
      {"id": "pf-aa2", "title": "troubleshooting", "depth": 1, "access_count": 28,
       "summary": "If deploy succeeds but service unchanged", "children": [...]},
      {"id": "pf-aa5", "title": "rollback", "depth": 1, "access_count": 3,
       "summary": "Steps to revert a bad deploy", "children": []}
    ]
  }
]
```

Non-root nodes include a `summary` field (the trigger phrase from the parent's pointer block).
Root nodes omit `summary` (they have no parent pointer).

**Output:**
```
deployment (pf-aa1)
├── troubleshooting (pf-aa2)
│   ├── nomad-allocation-stuck (pf-aa3)
│   └── gateway-vs-worker (pf-aa4)
├── rollback (pf-aa5)
└── version-history (pf-aa6)
entities (pf-bb1)
├── dedup-strategy (pf-bb2)
└── filter-list (pf-bb3)
pipeline (pf-cc1)
└── stage-inspection (pf-cc2)
```

**With --stats:**
```
deployment (pf-aa1)                    14 reads  last: 2h ago
├── troubleshooting (pf-aa2)           28 reads  last: 1h ago  ★
│   ├── nomad-allocation-stuck (pf-aa3) 12 reads  last: 3h ago
│   └── gateway-vs-worker (pf-aa4)      9 reads  last: 1d ago
├── rollback (pf-aa5)                   3 reads  last: 5d ago
└── version-history (pf-aa6)            7 reads  last: 4h ago
```

The ★ marker flags memories that are accessed more frequently than their parent
(promotion candidates).

### `cp memory hot` — Promotion Candidates

```bash
cp memory hot

# Filter to specific minimum depth
cp memory hot --min-depth 2

# Limit results
cp memory hot --limit 10
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--min-depth` | No | 1 | Minimum tree depth to consider |
| `--limit` | No | 20 | Max results |
| `-o` | No | text | Output format: text, json |

**JSON output (`-o json`):**
```json
[
  {
    "id": "pf-aa2", "title": "Troubleshooting", "depth": 1,
    "access_count": 28, "parent_id": "pf-aa1",
    "parent_title": "Deployment", "parent_access_count": 14
  }
]
```

**Output:**
```
PROMOTION CANDIDATES (children accessed more than parent)
─────────────────────────────────────────────────────────
ID         DEPTH  READS  PARENT READS  TITLE                    PARENT
pf-aa2     1      28     14            Troubleshooting          Deployment
pf-bb2     1      19     8             Dedup Strategy           Entities
pf-aa3     2      12     28            Nomad Stuck              Troubleshooting

Suggestion: pf-aa2 has 2x parent reads — consider promoting to root.
```

### `cp memory sync` — Reconcile Pointer Blocks

Safety net for when the JSON pointer block in a parent gets out of sync with
the actual graph.

```bash
# Dry run — show what would change
cp memory sync --dry-run

# Fix all parents
cp memory sync

# Fix specific parent
cp memory sync <parent-id>
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--dry-run` | No | false | Report differences without fixing |
| `-o` | No | text | Output format: text, json |

**What it does:**
1. For each memory with children (via `parent_id`):
   - Read the sub-memories JSON block from content
   - Read summaries from `child-of` edge metadata
   - Query actual children from the shards table
   - Compare: missing entries, stale entries, orphaned entries, edge/pointer mismatches
2. Report differences
3. If not dry-run: regenerate the JSON block from actual children AND update edge metadata
   - Preserves existing summaries where child still exists (prefers edge metadata as
     source of truth if pointer block and edge disagree)
   - Removes entries for deleted children
   - Adds placeholder entries for children without summaries
     (summary = "No summary — update this memory's pointer block manually or re-add with add-sub")
   - Ensures `child-of` edge metadata matches pointer block summary for each child

**Output (text, dry-run with discrepancies):**
```
Sync check: 3 parents, 2 discrepancies

  pf-aa1 "Deployment":
    MISSING: child pf-aa7 "Monitoring" not in pointer block
  pf-bb1 "Entities":
    STALE: pointer to pf-deleted-99 — shard no longer exists

Run `cp memory sync` to fix.
```

**Output (text, fix applied):**
```
Sync: 3 parents checked, 2 fixed

  pf-aa1 "Deployment": added pointer for pf-aa7 (placeholder summary)
  pf-bb1 "Entities": removed stale pointer to pf-deleted-99
```

**Output (text, all consistent):**
```
All pointer blocks are in sync. No changes needed.
```

**JSON output (`-o json`, dry-run):**
```json
{
  "parents_checked": 3,
  "discrepancies": [
    {"parent": "pf-aa1", "type": "missing_pointer", "child": "pf-aa7", "child_title": "Monitoring"},
    {"parent": "pf-bb1", "type": "stale_pointer", "child": "pf-deleted-99"}
  ],
  "fixed": false
}
```

**JSON output (`-o json`, fix):**
```json
{
  "parents_checked": 3,
  "discrepancies": [
    {"parent": "pf-aa1", "type": "missing_pointer", "child": "pf-aa7", "child_title": "Monitoring"},
    {"parent": "pf-bb1", "type": "stale_pointer", "child": "pf-deleted-99"}
  ],
  "fixed": true
}
```

### `cp memory add` — Create Root Memory (unchanged from SPEC-5)

```bash
# Existing behavior, unchanged
cp memory add "Deployment overview for Penfold system..." \
    --label deployment,infrastructure
```

Root memories created with `cp memory add` have no sub-memories block initially.
The block is created automatically when the first `add-sub` call adds a child.

### `cp memory list` — Flat by Default (amended in SPEC-5)

`cp memory list` shows all memories (root and child) in a flat list by default.
To see only root-level memories, use `--roots`:

```bash
# All memories (flat)
cp memory list

# Root memories only
cp memory list --roots

# Hierarchical view (SPEC-6)
cp memory tree
```

The `--roots` flag is added to SPEC-5's `cp memory list` flag table. The full
hierarchical view is `cp memory tree` (this spec).

## Database Changes

### Migration File

All SQL changes in this spec go into `cp/migrations/007_hierarchical_memory.sql`.

### Amendment to `list_shards()` (SPEC-5)

Add `p_parent_id_null BOOLEAN DEFAULT FALSE` parameter to the existing `list_shards()`
function. When true, adds `AND s.parent_id IS NULL` to the WHERE clause. This keeps
the `--roots` filter in SQL alongside the existing pagination, label, and type filters.

```sql
-- Add to existing list_shards() signature (append parameter):
--   p_parent_id_null BOOLEAN DEFAULT FALSE
-- Add to WHERE clause:
--   AND (NOT p_parent_id_null OR s.parent_id IS NULL)
CREATE OR REPLACE FUNCTION list_shards(
    p_project TEXT,
    p_types TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_creator TEXT DEFAULT NULL,
    p_search TEXT DEFAULT NULL,
    p_since TIMESTAMPTZ DEFAULT NULL,
    p_limit INT DEFAULT 50,
    p_offset INT DEFAULT 0,
    p_parent_id_null BOOLEAN DEFAULT FALSE  -- NEW: filter to root shards only
) RETURNS TABLE (
    id TEXT, title TEXT, type TEXT, status TEXT, creator TEXT,
    labels TEXT[], created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, snippet TEXT
) AS $$
    SELECT
        s.id, s.title, s.type, s.status, s.creator,
        s.labels, s.created_at, s.updated_at,
        left(s.content, 200) AS snippet
    FROM shards s
    WHERE s.project = p_project
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_status IS NULL OR s.status = ANY(p_status))
      AND (p_labels IS NULL OR s.labels @> p_labels)
      AND (p_creator IS NULL OR s.creator = p_creator)
      AND (p_search IS NULL OR s.search_vector @@ plainto_tsquery('english', p_search))
      AND (p_since IS NULL OR s.created_at >= p_since)
      AND (NOT p_parent_id_null OR s.parent_id IS NULL)
    ORDER BY s.created_at DESC
    LIMIT p_limit OFFSET p_offset;
$$ LANGUAGE sql STABLE;
```

### SQL Functions

```sql
-- Get memory tree (recursive), with summary from child-of edge metadata
CREATE OR REPLACE FUNCTION memory_tree(
    p_project TEXT,
    p_root_id TEXT DEFAULT NULL  -- NULL = all roots
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    parent_id TEXT,
    depth INT,
    status TEXT,
    labels TEXT[],
    access_count INT,
    last_accessed TIMESTAMPTZ,
    child_count INT,
    summary TEXT  -- from child-of edge metadata (NULL for roots)
) AS $$
    WITH RECURSIVE tree AS (
        -- Base case: root memories (or specific root)
        SELECT
            s.id, s.title, s.parent_id, 0 AS depth,
            s.status, s.labels, s.metadata, s.created_at
        FROM shards s
        WHERE s.project = p_project
          AND s.type = 'memory'
          AND s.status != 'closed'
          AND (
              (p_root_id IS NOT NULL AND s.id = p_root_id)
              OR
              (p_root_id IS NULL AND s.parent_id IS NULL)
          )

        UNION ALL

        -- Recursive case: children (depth-limited for safety)
        SELECT
            s.id, s.title, s.parent_id, t.depth + 1,
            s.status, s.labels, s.metadata, s.created_at
        FROM shards s
        JOIN tree t ON s.parent_id = t.id
        WHERE s.project = p_project
          AND s.type = 'memory'
          AND s.status != 'closed'
          AND t.depth < 20  -- Safety cap: prevent runaway recursion from cycles
    )
    SELECT
        t.id, t.title, t.parent_id, t.depth,
        t.status, t.labels,
        COALESCE((t.metadata->>'access_count')::int, 0),
        (t.metadata->>'last_accessed')::timestamptz,
        (SELECT count(*) FROM shards c
         WHERE c.parent_id = t.id AND c.type = 'memory' AND c.status != 'closed')::int,
        e.metadata->>'summary'  -- NULL for roots (no child-of edge)
    FROM tree t
    LEFT JOIN edges e ON e.from_id = t.id
                     AND e.to_id = t.parent_id
                     AND e.edge_type = 'child-of'
    ORDER BY t.depth, t.created_at;
$$ LANGUAGE sql STABLE;


-- Get direct children of a memory
CREATE OR REPLACE FUNCTION memory_children(
    p_project TEXT,
    p_parent_id TEXT
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    status TEXT,
    labels TEXT[],
    access_count INT,
    last_accessed TIMESTAMPTZ,
    child_count INT,
    content TEXT
) AS $$
    SELECT
        s.id, s.title, s.status, s.labels,
        COALESCE((s.metadata->>'access_count')::int, 0),
        (s.metadata->>'last_accessed')::timestamptz,
        (SELECT count(*) FROM shards c
         WHERE c.parent_id = s.id AND c.type = 'memory' AND c.status != 'closed')::int,
        s.content
    FROM shards s
    WHERE s.project = p_project
      AND s.parent_id = p_parent_id
      AND s.type = 'memory'
      AND s.status != 'closed'
    ORDER BY s.created_at;
$$ LANGUAGE sql STABLE;


-- Get path from root to a specific memory.
-- Walks UP from target to root via parent_id. The CTE depth starts at 0 for
-- the target and increments toward the root. The final SELECT inverts this:
-- real_depth = max(depth) - depth, so root=0, target=max. This gives a
-- root-first ordered path.
-- Note: no project filter needed — shard IDs are globally unique and parent_id
-- is a direct FK walk within the same project.
CREATE OR REPLACE FUNCTION memory_path(p_memory_id TEXT)
RETURNS TABLE (
    id TEXT,
    title TEXT,
    depth INT
) AS $$
    WITH RECURSIVE path AS (
        SELECT s.id, s.title, s.parent_id, 0 AS depth
        FROM shards s WHERE s.id = p_memory_id

        UNION ALL

        SELECT s.id, s.title, s.parent_id, p.depth + 1
        FROM shards s
        JOIN path p ON s.id = p.parent_id
        WHERE p.depth < 20  -- Safety cap
    )
    SELECT id, title, (SELECT max(depth) FROM path) - depth AS real_depth
    FROM path
    ORDER BY real_depth;
$$ LANGUAGE sql STABLE;


-- Promotion candidates: children accessed more than parent
CREATE OR REPLACE FUNCTION memory_hot(
    p_project TEXT,
    p_min_depth INT DEFAULT 1,
    p_limit INT DEFAULT 20
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    depth INT,
    access_count INT,
    parent_id TEXT,
    parent_title TEXT,
    parent_access_count INT
) AS $$
    WITH RECURSIVE tree AS (
        SELECT s.id, s.title, s.parent_id, 0 AS depth, s.metadata
        FROM shards s
        WHERE s.project = p_project
          AND s.type = 'memory'
          AND s.status != 'closed'
          AND s.parent_id IS NULL

        UNION ALL

        SELECT s.id, s.title, s.parent_id, t.depth + 1, s.metadata
        FROM shards s
        JOIN tree t ON s.parent_id = t.id
        WHERE s.project = p_project
          AND s.type = 'memory' AND s.status != 'closed'
          AND t.depth < 20  -- Safety cap
    )
    SELECT
        c.id, c.title, c.depth,
        COALESCE((c.metadata->>'access_count')::int, 0) AS access_count,
        p.id, p.title,
        COALESCE((p.metadata->>'access_count')::int, 0) AS parent_access_count
    FROM tree c
    JOIN shards p ON p.id = c.parent_id
    WHERE c.depth >= p_min_depth
      AND COALESCE((c.metadata->>'access_count')::int, 0) >
          COALESCE((p.metadata->>'access_count')::int, 0)
    ORDER BY COALESCE((c.metadata->>'access_count')::int, 0) DESC
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;


-- Increment access telemetry
CREATE OR REPLACE FUNCTION memory_touch(
    p_memory_id TEXT,
    p_agent TEXT,
    p_depth INT DEFAULT 0
) RETURNS VOID AS $$
DECLARE
    new_entry JSONB;
    old_log JSONB;
    new_log JSONB;
BEGIN
    new_entry := jsonb_build_object(
        'at', now()::text,
        'by', p_agent,
        'depth', p_depth
    );

    -- Read current log, prepend new entry, cap at 50
    SELECT COALESCE(metadata->'access_log', '[]'::jsonb)
    INTO old_log
    FROM shards WHERE id = p_memory_id;

    -- Build new log: new entry first, then first 49 of old log
    SELECT COALESCE(to_jsonb(array_agg(elem ORDER BY ord)), '[]'::jsonb)
    INTO new_log
    FROM (
        SELECT new_entry AS elem, 0 AS ord
        UNION ALL
        SELECT value, ordinality::int
        FROM jsonb_array_elements(old_log) WITH ORDINALITY
        WHERE ordinality <= 49
    ) sub;

    UPDATE shards
    SET metadata = jsonb_set(
        jsonb_set(
            jsonb_set(
                COALESCE(metadata, '{}'::jsonb),
                '{access_count}',
                to_jsonb(COALESCE((metadata->>'access_count')::int, 0) + 1)
            ),
            '{last_accessed}',
            to_jsonb(now()::text)
        ),
        '{access_log}',
        new_log
    )
    WHERE id = p_memory_id;
END;
$$ LANGUAGE plpgsql VOLATILE;
```

## Go Implementation Notes

### Package Structure

```
cp/
├── cmd/
│   ├── memory.go              # Existing from SPEC-5
│   ├── memory_sub.go          # add-sub, delete, move, promote
│   ├── memory_tree.go         # tree, hot, sync
│   └── memory_show.go         # Enhanced show with --depth
└── internal/
    ├── client/
    │   ├── memory_hierarchy.go # Tree queries, parent_id ops
    │   └── memory_telemetry.go # Touch, access log
    ├── generation/
    │   ├── generator.go        # Generator interface + GenerationConfig
    │   └── google.go           # GoogleGenerator (gemini-2.0-flash)
    ├── pointer/
    │   ├── parse.go            # Parse sub-memories block from content
    │   ├── render.go           # Render pointer block + renderWithBlock
    │   └── update.go           # Add/remove entries in block
    └── summary/
        └── generate.go         # AI summary prompt building + response parsing
```

### Pointer Block Parser

```go
const (
    SubMemoryStart = "<!-- sub-memories -->"
    SubMemoryEnd   = "<!-- /sub-memories -->"
)

type SubMemoryEntry struct {
    ID      string `json:"id"`
    Title   string `json:"title"`
    Summary string `json:"summary"`
}

// ParseSubMemories extracts the JSON block from memory content.
// Returns the main content (without the block) and the parsed entries.
func ParseSubMemories(content string) (mainContent string, entries []SubMemoryEntry, err error) {
    startIdx := strings.Index(content, SubMemoryStart)
    if startIdx == -1 {
        return content, nil, nil // No sub-memories block
    }
    endIdx := strings.Index(content, SubMemoryEnd)
    if endIdx == -1 {
        return content, nil, fmt.Errorf("found %s without closing %s", SubMemoryStart, SubMemoryEnd)
    }

    mainContent = strings.TrimRight(content[:startIdx], "\n")
    jsonBlock := content[startIdx+len(SubMemoryStart) : endIdx]
    jsonBlock = strings.TrimSpace(jsonBlock)

    if err := json.Unmarshal([]byte(jsonBlock), &entries); err != nil {
        return mainContent, nil, fmt.Errorf("invalid sub-memories JSON: %w", err)
    }
    return mainContent, entries, nil
}

// AppendSubMemory adds an entry to the pointer block.
// Creates the block if it doesn't exist.
func AppendSubMemory(content string, entry SubMemoryEntry) (string, error) {
    mainContent, entries, err := ParseSubMemories(content)
    if err != nil {
        return "", err
    }

    entries = append(entries, entry)
    return renderWithBlock(mainContent, entries)
}

// RemoveSubMemory removes an entry by ID from the pointer block.
func RemoveSubMemory(content string, childID string) (string, error) {
    mainContent, entries, err := ParseSubMemories(content)
    if err != nil {
        return "", err
    }

    filtered := make([]SubMemoryEntry, 0, len(entries))
    for _, e := range entries {
        if e.ID != childID {
            filtered = append(filtered, e)
        }
    }

    if len(filtered) == 0 {
        return mainContent, nil // Remove the block entirely
    }
    return renderWithBlock(mainContent, filtered)
}

// renderWithBlock serializes entries as indented JSON and wraps in delimiters.
func renderWithBlock(mainContent string, entries []SubMemoryEntry) (string, error) {
    jsonBytes, err := json.MarshalIndent(entries, "", "  ")
    if err != nil {
        return "", fmt.Errorf("failed to serialize sub-memories: %w", err)
    }
    return fmt.Sprintf("%s\n\n%s\n%s\n%s\n",
        mainContent, SubMemoryStart, string(jsonBytes), SubMemoryEnd), nil
}
```

### Add-Sub Flow

```go
func AddSubMemory(ctx context.Context, parentID string, opts AddSubOpts) (string, error) {
    // === Pre-transaction: validation, AI, embedding (no locks held) ===

    // 1. Load parent memory
    parent, err := GetShard(ctx, parentID)
    if err != nil {
        return "", fmt.Errorf("parent %s not found: %w", parentID, err)
    }
    if parent.Type != "memory" {
        return "", fmt.Errorf("parent %s is type '%s', expected 'memory'", parentID, parent.Type)
    }

    // 2. Generate summary (AI or manual)
    var summary string
    if opts.Summary != "" {
        summary = opts.Summary  // Manual, skip AI
    } else {
        result, err := generator.Generate(ctx, buildSummaryPrompt(parent, opts.Title, opts.Body))
        if err != nil {
            return "", fmt.Errorf("summary generation failed: %w", err)
        }
        parsed, err := parseSummaryResponse(result)
        if err != nil {
            return "", fmt.Errorf("failed to parse AI response: %w", err)
        }
        summary = parsed.Summary

        if !opts.AutoApprove {
            approved, editedSummary := promptApproval(parsed)
            if !approved {
                return "", fmt.Errorf("cancelled by user")
            }
            summary = editedSummary
        }
        // Note: parsed.ParentEdits is displayed in the prompt but never applied.
    }

    // 3. Pre-compute embedding (HTTP call — outside transaction)
    embeddingText := embedding.BuildEmbeddingText("memory", opts.Title, opts.Body)
    vector, _ := embeddingProvider.Embed(ctx, embeddingText)

    // === Atomic transaction: all SQL, no HTTP calls ===

    tx, err := db.Begin(ctx)

    // 4a. Lock parent shard (FOR UPDATE prevents concurrent pointer block corruption)
    parent, err = getShardForUpdate(tx, parentID)

    // 4b. Create child shard — call create_shard() SQL directly on tx
    var childID string
    err = tx.QueryRow(ctx,
        "SELECT create_shard($1, $2, $3, $4, 'memory', $5, $6, NULL, NULL)",
        parent.Project, config.Agent, opts.Title, opts.Body, opts.Labels, parentID,
    ).Scan(&childID)

    // 4c. Store pre-computed embedding
    _, err = tx.Exec(ctx,
        "UPDATE shards SET embedding = $1 WHERE id = $2", vector, childID)

    // 4d. Create child-of edge with summary in metadata
    edgeMeta := fmt.Sprintf(`{"summary": %s}`, jsonQuote(summary))
    _, err = tx.Exec(ctx,
        "INSERT INTO edges (from_id, to_id, edge_type, metadata) VALUES ($1, $2, 'child-of', $3::jsonb)",
        childID, parentID, edgeMeta)

    // 4e. Update parent content (add pointer block entry)
    parentContent, _ := AppendSubMemory(parent.Content, SubMemoryEntry{
        ID:      childID,
        Title:   opts.Title,
        Summary: summary,
    })
    _, err = tx.Exec(ctx,
        "UPDATE shards SET content = $1, updated_at = now() WHERE id = $2",
        parentContent, parentID)

    tx.Commit()
    return childID, nil
}
```

## Success Criteria

1. **`add-sub` creates child with pointer:** Child shard created with correct
   `parent_id`. Parent content updated with JSON entry in sub-memories block.
   `child-of` edge exists. Embedding generated.
2. **AI summary generation:** Prompt produces trigger-phrase summaries that describe
   *when* to read the child, not just *what* it contains. Parent review catches
   stale text.
3. **Interactive approval:** User can approve summary, edit summary, or cancel.
   Parent edit suggestions are displayed but never auto-applied. `--no-ai` and
   `--auto-approve` modes work for automation (`--auto-approve` silently discards
   parent edit suggestions).
4. **`delete` is atomic:** Shard removed, pointer removed from parent, edge removed.
   `--recursive` deletes entire subtree. Refuses if has children without `--recursive`.
5. **`move` updates both parents:** Old parent's pointer removed, new parent's pointer
   added (preserving existing summary). `parent_id` and edge updated.
6. **`promote` moves up one level:** Grandchild becomes child, child becomes root.
   Pointers updated in both old and new parents.
7. **`show --depth N` expands inline:** Depth 0 shows pointers. Depth 1 shows
   children's full content. Depth 2 includes grandchildren. Each read increments
   telemetry.
8. **`tree` shows full hierarchy:** Indented tree with IDs. `--stats` shows access
   counts and flags promotion candidates with ★.
9. **`hot` identifies promotion candidates:** Lists memories accessed more than their
   parent, sorted by access count descending.
10. **`sync` reconciles:** Detects missing/orphaned/stale entries. `--dry-run` reports
    without changing. Fix mode regenerates blocks from actual children.
11. **Access telemetry:** Every `show` call increments `access_count`, updates
    `last_accessed`, appends to `access_log` (capped at 50).
12. **Semantic search finds children:** `cp memory recall` and `cp recall` find
    sub-memories regardless of tree position (embeddings work at all levels).
13. **JSON output:** All commands support `-o json` with structured output.
14. **`cp memory list --roots`:** (SPEC-5 amendment) Shows only memories with `parent_id IS NULL`.
    Default `cp memory list` shows all memories (root and child) in flat list.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| `add-sub` to non-memory shard | Error: "Shard pf-xxx is type 'task', expected 'memory'." |
| `add-sub` to non-existent parent | Error: "Shard pf-xxx not found." |
| `add-sub` creates deeply nested (depth > 5) | Warn: "This memory will be at depth 6. Deep hierarchies increase access latency. Continue? (y/n)" |
| `delete` memory with children (no --recursive) | Error: "Memory has 3 children. Use --recursive to delete subtree, or move children first." |
| `delete` root memory with children | Same as above — must use --recursive or move children. |
| `delete` child memory | Shard deleted + pointer removed from parent. |
| `move` to self | Error: "Cannot move memory to itself." |
| `move` to own descendant | Error: "Cannot move memory to its own descendant (would create cycle)." |
| `move` to non-memory shard | Error: "Target pf-xxx is type 'task', expected 'memory'." |
| `promote` root memory (depth 0) | Error: "Memory is already at root level." |
| `show --depth 3` on root with 100 descendants | Works but warn: "Expanding 100 memories. This may produce large output." |
| `tree` with no memories | Empty output, exit code 0. |
| `tree` with orphaned memory (parent_id references deleted shard) | Shown as root with warning marker. `sync` would fix. |
| `sync --dry-run` all consistent | "All pointer blocks are in sync. No changes needed." |
| `sync` finds child without pointer in parent | Adds placeholder entry: "No summary — needs regeneration." |
| `sync` finds pointer to deleted child | Removes the stale pointer entry. |
| Sub-memories block has invalid JSON | `show` displays raw block with warning. `sync` can fix. |
| `add-sub --no-ai` without `--summary` | Error: "--summary required when using --no-ai." |
| `hot` with no children accessed more than parent | "No promotion candidates found." Exit code 0. |
| Two agents add-sub to same parent concurrently | `FOR UPDATE` on parent shard serializes the operations — second transaction blocks until first commits, then reads updated pointer block. |
| Memory content contains `<!-- sub-memories -->` text | Unlikely but possible. Parser takes first occurrence. Document as reserved marker. |
| `show --depth 6` (exceeds max 5) | Error: "Maximum depth is 5." |
| `add-sub` when parent has no sub-memories block yet | Block is created automatically with single entry. |
| `delete --recursive` on large subtree (50+) | Deletes depth-first. Confirm: "Delete Troubleshooting and 47 descendants? (y/n)" |

---

## Test Cases

### SQL Tests: memory_tree

```
TEST: memory_tree returns full hierarchy
  Given: Root A with children B, C. B has child D.
  When:  SELECT * FROM memory_tree('test')
  Then:  Returns 4 rows: A(depth=0), B(depth=1), C(depth=1), D(depth=2)
         Ordered by depth then created_at

TEST: memory_tree from specific root
  Given: Root A with child B. Root X with child Y.
  When:  SELECT * FROM memory_tree('test', 'A')
  Then:  Returns A and B only (X and Y excluded)

TEST: memory_tree includes access counts
  Given: Memory A with metadata.access_count = 14
  When:  SELECT * FROM memory_tree('test')
  Then:  A row has access_count = 14

TEST: memory_tree includes child counts
  Given: Memory A with 3 children
  When:  SELECT * FROM memory_tree('test')
  Then:  A row has child_count = 3

TEST: memory_tree excludes closed memories
  Given: Root A, child B (open), child C (closed)
  When:  SELECT * FROM memory_tree('test')
  Then:  Returns A and B only

TEST: memory_tree excludes non-memory shards
  Given: Memory A with parent_id = NULL. Task T with parent_id = NULL.
  When:  SELECT * FROM memory_tree('test')
  Then:  Returns only A (task excluded)

TEST: memory_tree empty project
  Given: No memory shards in project
  When:  SELECT * FROM memory_tree('test')
  Then:  Returns 0 rows
```

### SQL Tests: memory_children

```
TEST: memory_children returns direct children only
  Given: Parent A, children B and C, grandchild D (child of B)
  When:  SELECT * FROM memory_children('test', 'A')
  Then:  Returns B and C only (D excluded — not direct child)

TEST: memory_children includes content
  Given: Child B with content "Full troubleshooting guide..."
  When:  SELECT * FROM memory_children('test', 'A')
  Then:  B row includes full content

TEST: memory_children includes child counts
  Given: Child B has 2 children, child C has 0
  When:  SELECT * FROM memory_children('test', 'A')
  Then:  B has child_count=2, C has child_count=0

TEST: memory_children no children
  Given: Memory A with no children
  When:  SELECT * FROM memory_children('test', 'A')
  Then:  Returns 0 rows
```

### SQL Tests: memory_path

```
TEST: memory_path for root
  Given: Root memory A
  When:  SELECT * FROM memory_path('A')
  Then:  Returns 1 row: A at depth 0

TEST: memory_path for grandchild
  Given: Root A → child B → grandchild C
  When:  SELECT * FROM memory_path('C')
  Then:  Returns 3 rows: A(depth=0), B(depth=1), C(depth=2)

TEST: memory_path order
  Given: Root A → child B → grandchild C
  When:  SELECT * FROM memory_path('C')
  Then:  Rows ordered root-first: A, B, C
```

### SQL Tests: memory_hot

```
TEST: memory_hot finds promotion candidates
  Given: Parent A (access_count=10), child B (access_count=20)
  When:  SELECT * FROM memory_hot('test')
  Then:  Returns B with access_count=20, parent_access_count=10

TEST: memory_hot excludes children with fewer reads than parent
  Given: Parent A (access_count=20), child B (access_count=5)
  When:  SELECT * FROM memory_hot('test')
  Then:  Returns 0 rows (B not a candidate)

TEST: memory_hot respects min_depth
  Given: Root A (10 reads), child B (20 reads), grandchild C (30 reads)
  When:  SELECT * FROM memory_hot('test', 2)
  Then:  Returns only C (depth=2, meets min_depth filter)

TEST: memory_hot ordered by access count
  Given: Child B (20 reads), child C (30 reads), both above parent
  When:  SELECT * FROM memory_hot('test')
  Then:  C first (30 reads), B second (20 reads)

TEST: memory_hot no candidates
  Given: All children have fewer reads than parents
  When:  SELECT * FROM memory_hot('test')
  Then:  Returns 0 rows
```

### SQL Tests: memory_touch

```
TEST: memory_touch increments counter
  Given: Memory with access_count = 5
  When:  SELECT memory_touch('mem-1', 'agent-penfold', 0)
  Then:  access_count = 6

TEST: memory_touch initializes counter
  Given: Memory with no metadata
  When:  SELECT memory_touch('mem-1', 'agent-penfold', 0)
  Then:  access_count = 1, last_accessed set, access_log has 1 entry

TEST: memory_touch updates last_accessed
  Given: Memory with last_accessed = yesterday
  When:  SELECT memory_touch('mem-1', 'agent-penfold', 0)
  Then:  last_accessed = now()

TEST: memory_touch appends to access_log
  Given: Memory with access_log containing 3 entries
  When:  SELECT memory_touch('mem-1', 'agent-penfold', 2)
  Then:  access_log has 4 entries, newest first
         Newest entry has by='agent-penfold', depth=2

TEST: memory_touch caps access_log at 50
  Given: Memory with access_log containing 50 entries
  When:  SELECT memory_touch('mem-1', 'agent-penfold', 0)
  Then:  access_log has 50 entries (oldest dropped)
         Newest entry is the one just added
```

### Go Unit Tests: Pointer Block

```
TEST: ParseSubMemories no block
  Given: Content "Just some text with no block"
  When:  ParseSubMemories(content)
  Then:  mainContent = "Just some text with no block"
         entries = nil, err = nil

TEST: ParseSubMemories with block
  Given: Content with valid sub-memories block (2 entries)
  When:  ParseSubMemories(content)
  Then:  mainContent = text before the block (trimmed)
         entries = 2 SubMemoryEntry structs with correct fields

TEST: ParseSubMemories invalid JSON
  Given: Content with sub-memories markers but invalid JSON inside
  When:  ParseSubMemories(content)
  Then:  Returns mainContent + error about invalid JSON

TEST: ParseSubMemories unclosed block
  Given: Content with opening marker but no closing marker
  When:  ParseSubMemories(content)
  Then:  Returns error about missing closing marker

TEST: AppendSubMemory creates block
  Given: Content "Just text" (no existing block)
  When:  AppendSubMemory(content, SubMemoryEntry{ID: "pf-1", ...})
  Then:  Returns content + block with 1 entry

TEST: AppendSubMemory adds to existing block
  Given: Content with block containing 2 entries
  When:  AppendSubMemory(content, SubMemoryEntry{ID: "pf-3", ...})
  Then:  Block now has 3 entries, original 2 preserved

TEST: RemoveSubMemory removes entry
  Given: Content with block containing entries pf-1, pf-2, pf-3
  When:  RemoveSubMemory(content, "pf-2")
  Then:  Block has pf-1 and pf-3. pf-2 removed.

TEST: RemoveSubMemory last entry removes block
  Given: Content with block containing 1 entry (pf-1)
  When:  RemoveSubMemory(content, "pf-1")
  Then:  Returns content without any block markers

TEST: RemoveSubMemory non-existent ID
  Given: Content with block containing pf-1, pf-2
  When:  RemoveSubMemory(content, "pf-99")
  Then:  Block unchanged (pf-1, pf-2 still present)

TEST: roundtrip parse → append → parse
  Given: Original content with 2 entries
  When:  Parse → append pf-3 → parse again
  Then:  3 entries, mainContent identical to original
```

### Go Unit Tests: Summary Generation

```
TEST: buildSummaryPrompt includes parent and child
  Given: Parent content "Deploy overview...", child title "Troubleshooting", child body "If service fails..."
  When:  buildSummaryPrompt(parent, "Troubleshooting", childBody)
  Then:  Prompt contains parent content, child title, child body
         Prompt instructs trigger-phrase format

TEST: parseSummaryResponse valid JSON
  Given: AI response '{"summary": "If deploy fails...", "parent_needs_update": false, "parent_edits": null}'
  When:  parseSummaryResponse(response)
  Then:  summary = "If deploy fails...", parentNeedsUpdate = false

TEST: parseSummaryResponse with parent edits
  Given: AI response with parent_needs_update=true and parent_edits="Change line 3..."
  When:  parseSummaryResponse(response)
  Then:  parentNeedsUpdate = true, parentEdits = "Change line 3..."

TEST: parseSummaryResponse truncates long summary
  Given: AI returns summary > 120 chars
  When:  parseSummaryResponse(response)
  Then:  Warns about length but still returns it (user can edit in approval)

TEST: parseSummaryResponse invalid JSON
  Given: AI returns non-JSON response
  When:  parseSummaryResponse(response)
  Then:  Returns error, falls back to prompting user for manual summary
```

### Integration Tests: Add-Sub

```
TEST: add-sub creates child with pointer
  Given: Root memory pf-aa1
  When:  `cp memory add-sub pf-aa1 --title "Troubleshooting" --body "..." --no-ai --summary "If deploy fails"`
  Then:  New shard created with parent_id = pf-aa1
         `cp memory show pf-aa1` shows sub-memories block with entry
         `cp shard edges <child-id>` shows child-of edge to pf-aa1

TEST: add-sub with AI generates summary
  Given: Root memory with deployment content
  When:  `cp memory add-sub pf-aa1 --title "Troubleshooting" --body-file troubleshoot.md --auto-approve`
  Then:  Child created, summary auto-generated, parent pointer block updated

TEST: add-sub to non-memory fails
  Given: Task shard pf-task-1
  When:  `cp memory add-sub pf-task-1 --title "X" --body "Y" --no-ai --summary "Z"`
  Then:  Exit code 1, error about type mismatch

TEST: add-sub to non-existent parent fails
  Given: No shard pf-xxx
  When:  `cp memory add-sub pf-xxx --title "X" --body "Y" --no-ai --summary "Z"`
  Then:  Exit code 1, error "Shard pf-xxx not found"

TEST: add-sub no-ai without summary fails
  Given: Root memory pf-aa1
  When:  `cp memory add-sub pf-aa1 --title "X" --body "Y" --no-ai`
  Then:  Exit code 1, error "--summary required when using --no-ai"

TEST: add-sub creates first block
  Given: Root memory with no sub-memories block
  When:  `cp memory add-sub pf-aa1 --title "X" --body "Y" --no-ai --summary "Z"`
  Then:  Parent content now has sub-memories block with 1 entry

TEST: add-sub appends to existing block
  Given: Root memory with 2 entries in block
  When:  `cp memory add-sub pf-aa1 --title "Third" --body "..." --no-ai --summary "S"`
  Then:  Block now has 3 entries, original 2 preserved

TEST: add-sub with labels
  Given: Root memory pf-aa1
  When:  `cp memory add-sub pf-aa1 --title "X" --body "Y" --no-ai --summary "Z" --label deployment,nomad`
  Then:  Child shard has labels = ['deployment', 'nomad']
```

### Integration Tests: Delete

```
TEST: delete leaf memory
  Given: Root A with child B (no grandchildren)
  When:  `cp memory delete B --force`
  Then:  B deleted. A's block no longer contains B entry.

TEST: delete memory with children fails without recursive
  Given: Root A with child B, B has child C
  When:  `cp memory delete B`
  Then:  Exit code 1, error about children

TEST: delete recursive
  Given: Root A with child B, B has children C and D
  When:  `cp memory delete B --recursive --force`
  Then:  B, C, D all deleted. A's block no longer contains B.

TEST: delete root memory (no parent)
  Given: Root memory A with no children
  When:  `cp memory delete A --force`
  Then:  A deleted. No parent to update.
```

### Integration Tests: Move

```
TEST: move child to different parent
  Given: Root A with child B. Root X exists.
  When:  `cp memory move B X`
  Then:  B.parent_id = X. A's block no longer has B. X's block now has B (with same summary).

TEST: move to root
  Given: Root A with child B
  When:  `cp memory move B --root`
  Then:  B.parent_id = NULL. A's block no longer has B. B is now a root memory.

TEST: move to self fails
  Given: Memory A
  When:  `cp memory move A A`
  Then:  Exit code 1, error "Cannot move to itself"

TEST: move to own descendant fails
  Given: Root A with child B, B has child C
  When:  `cp memory move A C`
  Then:  Exit code 1, error "Cannot move to own descendant"

TEST: move preserves summary
  Given: Root A with child B (summary = "If deploy fails...")
  When:  `cp memory move B X`
  Then:  X's block entry for B has summary = "If deploy fails..."
```

### Integration Tests: Promote

```
TEST: promote child to root
  Given: Root A with child B
  When:  `cp memory promote B`
  Then:  B.parent_id = NULL. B is root. A's block no longer has B.

TEST: promote grandchild to child
  Given: Root A → child B → grandchild C
  When:  `cp memory promote C`
  Then:  C.parent_id = A. B's block no longer has C. A's block now has C.

TEST: promote root fails
  Given: Root memory A
  When:  `cp memory promote A`
  Then:  Exit code 1, error "already at root level"
```

### Integration Tests: Show with Depth

```
TEST: show depth 0 (default)
  Given: Root A with children B, C
  When:  `cp memory show A`
  Then:  Shows A content + sub-memories pointer list (B, C as summaries)
         Does NOT show B or C content

TEST: show depth 1
  Given: Root A with children B, C. B has child D.
  When:  `cp memory show A --depth 1`
  Then:  Shows A content + B full content + C full content
         D shown as pointer in B's sub-memories (not expanded)

TEST: show depth 2
  Given: Root A → B → D
  When:  `cp memory show A --depth 2`
  Then:  A, B, and D all expanded with full content

TEST: show increments telemetry
  Given: Memory A with access_count = 5
  When:  `cp memory show A`
  Then:  access_count = 6, last_accessed updated

TEST: show depth 1 increments telemetry for children too
  Given: Root A, child B, both with access_count = 0
  When:  `cp memory show A --depth 1`
  Then:  A.access_count = 1, B.access_count = 1

TEST: show non-existent memory
  Given: No shard pf-nonexistent
  When:  `cp memory show pf-nonexistent`
  Then:  Exit code 1, error "not found"

TEST: show --depth 6 rejected
  Given: Root memory A
  When:  `cp memory show A --depth 6`
  Then:  Exit code 1, error "Maximum depth is 5"
```

### Integration Tests: Tree

```
TEST: tree shows full hierarchy
  Given: Root A (children B, C), Root X (child Y)
  When:  `cp memory tree`
  Then:  Output shows both trees with proper indentation

TEST: tree from specific root
  Given: Root A (children B, C), Root X (child Y)
  When:  `cp memory tree A`
  Then:  Only shows A tree (B, C)

TEST: tree with stats
  Given: Root A (14 reads), child B (28 reads)
  When:  `cp memory tree --stats`
  Then:  Shows read counts. B has ★ marker (more reads than parent).

TEST: tree max-depth
  Given: Root A → B → C → D (4 levels)
  When:  `cp memory tree --max-depth 2`
  Then:  Shows A, B, C only. D not shown.
```

### Integration Tests: Hot

```
TEST: hot finds candidates
  Given: Root A (10 reads), child B (20 reads)
  When:  `cp memory hot`
  Then:  B shown as promotion candidate

TEST: hot no candidates
  Given: All children fewer reads than parents
  When:  `cp memory hot`
  Then:  "No promotion candidates found."

TEST: hot min-depth filter
  Given: Child B (depth 1, 20 reads > parent), grandchild C (depth 2, 30 reads > parent)
  When:  `cp memory hot --min-depth 2`
  Then:  Only C shown
```

### Integration Tests: Sync

```
TEST: sync dry-run detects missing pointer
  Given: Child B with parent_id = A, but A's block doesn't mention B
  When:  `cp memory sync --dry-run`
  Then:  Reports "Missing pointer: A should reference child B"

TEST: sync dry-run detects stale pointer
  Given: A's block mentions deleted shard pf-99
  When:  `cp memory sync --dry-run`
  Then:  Reports "Stale pointer: A references non-existent pf-99"

TEST: sync fixes missing pointer
  Given: Child B missing from A's block
  When:  `cp memory sync`
  Then:  A's block now includes B with placeholder summary

TEST: sync fixes stale pointer
  Given: A's block mentions deleted shard
  When:  `cp memory sync`
  Then:  Stale entry removed from A's block

TEST: sync all consistent
  Given: All pointer blocks match reality
  When:  `cp memory sync --dry-run`
  Then:  "All pointer blocks are in sync."
```

### Integration Tests: Semantic Search with Hierarchy

```
TEST: recall finds sub-memories
  Given: Root A about "deployment overview", child B about "deployment troubleshooting"
  When:  `cp recall "deployment issues"`
  Then:  Both A and B appear in results (embeddings work at all levels)

TEST: recall finds deeply nested memory
  Given: Root A → child B → grandchild C about "Nomad allocation stuck"
  When:  `cp recall "nomad allocation"`
  Then:  C appears in results regardless of tree depth
```

### Integration Tests: Depth Warning

```
TEST: add-sub deep nesting warning
  Given: Memory chain A → B → C → D → E → F (F is at depth 5)
  When:  `cp memory add-sub F --title "G" --body "..." --no-ai --summary "S"`
  Then:  Warning: "This memory will be at depth 6. Deep hierarchies increase access latency. Continue? (y/n)"
```

---

## Cross-Spec Interactions

| Spec | Interaction |
|------|-------------|
| **SPEC-0** (CLI skeleton) | Uses `create_shard()` for memory creation. Inherits `--project`, `-o`, `--debug` flags. |
| **SPEC-1** (semantic search) | Sub-memories get embeddings on create. `cp recall` and `cp memory recall` find sub-memories regardless of tree position. |
| **SPEC-2** (metadata) | Access telemetry stored in `metadata JSONB` (access_count, last_accessed, access_log). GIN index available for metadata queries but access telemetry is read via direct key access, not filtered. |
| **SPEC-3** (requirements) | No direct interaction. Requirements can reference memory shards via edges but this is not managed by SPEC-6. |
| **SPEC-4** (knowledge docs) | No direct interaction. Knowledge documents are a different pattern (versioning vs hierarchy). Memory pointers can reference knowledge doc IDs but this is not managed by SPEC-6. |
| **SPEC-5** (unified search) | Extends `cp memory` from SPEC-5 with `add-sub`, `delete`, `move`, `promote`, `tree`, `hot`, `sync`, enhanced `show`. Uses `create_edge()` (SPEC-5) for `child-of` edges. `cp memory add` (SPEC-5) creates root memories. `cp memory list` (SPEC-5) amended with `--roots` flag to filter to root-only. `parent_id` column is existing infrastructure used by SPEC-6 for the first time. |

## Design Decisions (Resolved)

1. **Concurrent add-sub to same parent:** `SELECT ... FOR UPDATE` on the parent shard.
   Same pattern as SPEC-4. Serializes concurrent writes, safe and simple.
2. **Parent edit suggestions:** Suggestion only — displayed in the interactive approval prompt
   but never auto-applied. Skipped silently in `--auto-approve` mode. No diff/patch logic needed.
3. **`cp memory list` default:** Shows all memories (root and child) in flat list. `--roots`
   flag added to SPEC-5 to filter to root-level only. `cp memory tree` for hierarchical view.
4. **AI provider for summary generation:** Google Gemini (gemini-2.0-flash), reusing the
   existing `GOOGLE_API_KEY`. New `Generator` interface in `internal/generation/` — separate
   from embedding (different shapes). 30s timeout, no retry on failure.
5. **Transaction scope:** Embedding and AI summary generation happen *before* the transaction.
   The transaction only contains SQL operations (FOR UPDATE, create_shard, store embedding,
   create edge, update pointer block). Minimizes lock hold time.
6. **Transaction-aware shard creation:** Call `create_shard()` SQL function directly on the
   `pgx.Tx` handle via `tx.QueryRow()`. No Go-side `CreateShardInTx()` wrapper needed.
7. **`--roots` filter:** New `p_parent_id_null BOOLEAN DEFAULT FALSE` parameter on `list_shards()`.
   Keeps filtering in SQL alongside existing pagination/label/type filters.
8. **Summary storage (dual write):** Summaries stored in both the parent's pointer block (for
   agent in-context navigation) and the `child-of` edge metadata (for SQL-queryable tree
   display). `memory_tree()` LEFT JOINs edges to return summaries. `cp memory sync` reconciles both.
9. **CASCADE delete:** Confirmed — `edges.from_id` and `edges.to_id` both have `ON DELETE CASCADE`.
   Deleting a shard automatically removes its edges. No explicit edge deletion needed.

---

## Pre-Submission Checklist

- [x] Every item in "What to Build" has: CLI section + SQL + success criterion + tests
- [x] Every data flow answers all 7 questions (who writes/when/where/who reads/how/what for/staleness)
- [x] Every command has: syntax + example + output + atomic steps + JSON schema
- [x] Every workflow has: flowchart + all branches + error recovery + non-interactive mode
- [x] Every success criterion has at least one test case
- [x] Concurrency is addressed (FOR UPDATE chosen — see Design Decisions)
- [x] No feature is "mentioned but not specced" (pointer block format, telemetry, all commands)
- [x] Edge cases cover: invalid input, empty state, conflicts, boundaries, cross-feature, failure recovery
- [x] Existing spec interactions documented (Cross-Spec Interactions table)
- [x] Open design questions resolved (9 items — see Design Decisions)
- [x] Sub-agent review completed (25 items: 3 High, 10 Medium, 12 Low — all fixed)
- [x] Implementation review completed (5 questions resolved: AI provider, transaction design, --roots SQL, tree summaries, CASCADE)
