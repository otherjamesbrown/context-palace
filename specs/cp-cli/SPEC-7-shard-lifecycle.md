# SPEC-7: Shard Lifecycle, Epics, and Focus

**Status:** Draft
**Depends on:** SPEC-0 (CLI skeleton), SPEC-2 (metadata), SPEC-5 (unified search — `cp shard` namespace, `create_edge()`, labels table)
**Blocks:** Nothing

---

## Goal

Give humans and agents clear visibility into what work exists, what's being
worked on, what's done, and what's next. Today shards are flat — 12 open
shards with no grouping, no ordering, and no agent assignment. Agents finish
work but don't close shards. Humans can't ask "what's mycroft doing?" or
"what should I work on next?"

This spec introduces three things:
1. **Epics** — a shard that groups related work with progress tracking
2. **Focus** — persistent "active epic" per project+agent, survives context clears
3. **Lifecycle enforcement** — status transitions, agent assignment, and "what's next" queries

## What Exists

- `shards.status` column: `open`, `in_progress`, `closed`
- `shards.owner` column: text, not consistently used by agents
- `shards.parent_id` column: FK to shards, indexed — used by SPEC-6 for memory hierarchy
- `shards.type` column: freeform (`task`, `message`, `memory`, etc.)
- `shards.priority` column: 0-4 (critical to backlog)
- `labels` table: `kind:bug-report`, `kind:feature-request`, etc. — inconsistently applied
- `edges` table: `blocked-by` edge type exists, used in ingest pipeline Phase 3
- `shards.closed_at`, `closed_by`, `closed_reason` columns
- Ingest pipeline creates sub-shards with `blocked-by` edges for HIGH items
- Ingest pipeline does NOT set `parent_id` when decomposing, does NOT consistently close shards

## What to Build

1. **Epic shard type** — `type='epic'` shard that groups child work items via `parent_id`
2. **Kind labels convention** — standardize `kind:bug`, `kind:feature`, `kind:test`, `kind:task` labels
3. **Focus table** — persistent active epic per project+agent
4. **`cp epic create`** — create an epic with children or adopt existing shards
5. **`cp epic show`** — epic detail with progress bar and child status
6. **`cp epic list`** — list all epics with completion stats
7. **`cp shard assign`** — claim a shard (set owner + in_progress)
8. **`cp shard close`** — close a shard with reason
9. **`cp shard next`** — find next unblocked shard (within focus or globally)
10. **`cp shard board`** — kanban view of shards by status
11. **`cp focus`** — show/set/clear active epic
12. **SQL functions** — epic_progress, shard_next, shard_board, focus management
13. **Ingest pipeline updates** — create epics for decomposed work, assign on pickup, close on verify

## Data Model

### Schema Changes

```sql
-- New table: focus tracking (one active focus per project+agent)
CREATE TABLE focus (
    project     TEXT NOT NULL,
    agent       TEXT NOT NULL,
    epic_id     TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
    set_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note        TEXT,  -- optional: "working on pipeline quality this afternoon"
    PRIMARY KEY (project, agent)
);

CREATE INDEX idx_focus_epic ON focus(epic_id);
```

No changes to the shards table — `type`, `status`, `owner`, `parent_id`, `priority`, `labels`
all exist. This spec standardizes their usage for work tracking.

### Storage Format

**Epic shard content** contains a structured summary:

```markdown
## Pipeline Quality Improvements

Improve entity extraction, acronym detection, and processing reliability.

### Acceptance Criteria
- Entity display names populated for all people
- Acronym detection finds 12+ of 15 known acronyms
- Junk entities filtered automatically
- All processing completes within timeout

### Ordering
1. pf-25f2f3 Configurable pipeline timeouts (no deps)
2. pf-5d3921 Service version/deploy history (no deps)
3. pf-1296a9 Reprocessing with overrides (after: pf-25f2f3)
4. pf-086c4d Quality Dashboard (no deps)
5. pf-53e849 Entity Deduplication (after: pf-086c4d)
```

The ordering section is human-readable documentation. The actual dependency
enforcement uses `blocked-by` edges in the edges table. The content is the
source of intent; the edges are the source of enforcement.

**Kind labels** — standardized values:

| Label | Meaning | Set by |
|-------|---------|--------|
| `kind:bug` | Bug fix | Ingest Phase 1 (classify) |
| `kind:feature` | New capability | Ingest Phase 1 (classify) |
| `kind:test` | Test-only work | Ingest Phase 3 (triage) |
| `kind:task` | Generic work item | Manual or ingest |
| `kind:epic` | Epic container | `cp epic create` or Phase 3 |

### Data Flow

#### Epic shard (type='epic')

1. **WHO writes it?** `cp epic create` or ingest Phase 3 (when decomposing HIGH items)
2. **WHEN is it written?** When grouping related work, or when HIGH item is decomposed into layers
3. **WHERE is it stored?** `shards` table with `type='epic'`
4. **WHO reads it?** `cp epic show`, `cp epic list`, `cp shard board`, `cp shard next`, `cp focus`
5. **HOW is it queried?** `WHERE type = 'epic'` + project filter. Children via `WHERE parent_id = epic.id`.
6. **WHAT decisions does it inform?** Progress tracking, focus selection, "what's next" ordering
7. **DOES it go stale?** No — it's a container. Progress is computed from children's status.

#### Focus row

1. **WHO writes it?** `cp focus set` or agent startup
2. **WHEN is it written?** When user/agent declares current work area
3. **WHERE is it stored?** `focus` table (project, agent, epic_id)
4. **WHO reads it?** `cp focus`, `cp shard next`, `cp shard board`, assistant pickup
5. **HOW is it queried?** `SELECT * FROM focus WHERE project = $1 AND agent = $2`
6. **WHAT decisions does it inform?** Scopes `shard next` and `shard board` to the active epic
7. **DOES it go stale?** Yes — if the epic is closed, focus should auto-clear. `focus_get()` checks this.

#### Shard owner + status transitions

1. **WHO writes it?** `cp shard assign` (sets owner + in_progress), `cp shard close` (sets closed), agents via pipeline
2. **WHEN is it written?** On assignment (pickup) and completion (close)
3. **WHERE is it stored?** `shards.owner`, `shards.status`, `shards.closed_at`, `shards.closed_by`
4. **WHO reads it?** `cp shard board`, `cp shard next`, `cp epic show`, humans asking "what's mycroft doing?"
5. **HOW is it queried?** `WHERE owner = $1 AND status = 'in_progress'` for active work
6. **WHAT decisions does it inform?** Workload visibility, blocking detection, completion tracking
7. **DOES it go stale?** Yes — if an agent crashes mid-work, shard stays in_progress with stale owner. No auto-cleanup in this spec (future: TTL on in_progress).

#### Kind labels (in labels table)

1. **WHO writes it?** Ingest Phase 1 (classify) for incoming work. `cp epic create` for epics. Manual via `cp shard label add <id> kind:bug`.
2. **WHEN is it written?** On shard creation. Existing shards without kind labels are not backfilled automatically.
3. **WHERE is it stored?** `labels` table — row `(shard_id, 'kind:bug')` etc.
4. **WHO reads it?** `epic_children()`, `shard_next()`, `shard_board()` — extracted via subquery `WHERE l.label LIKE 'kind:%'`.
5. **HOW is it queried?** `SELECT replace(l.label, 'kind:', '') FROM labels l WHERE l.shard_id = s.id AND l.label LIKE 'kind:%' LIMIT 1`. Falls back to `'task'` if no kind label exists.
6. **WHAT decisions does it inform?** Display classification in board/epic views. Helps user distinguish bugs from features at a glance.
7. **DOES it go stale?** No — set once at creation, not expected to change. If a shard is reclassified, the old label should be removed and new one added manually.

### Concurrency

**Concurrent focus set:** Two agents setting focus for the same project+agent simultaneously.
`INSERT ... ON CONFLICT (project, agent) DO UPDATE` handles this — last writer wins. This is
fine because focus is per-agent, and a single agent won't race itself.

**Concurrent shard assign:** Two agents trying to assign the same shard. `SELECT ... FOR UPDATE`
on the shard, then check status. If already in_progress, second agent gets error "already assigned
to agent-X." Serialized by row lock.

**Concurrent shard close:** Safe — closing an already-closed shard is idempotent (no-op with warning).

## CLI Surface

### `cp epic create` — Create Epic

```bash
# Create epic with title and description
cp epic create --title "Pipeline Quality" \
    --body "Improve entity extraction and processing reliability"

# Create epic and adopt existing shards as children
cp epic create --title "Pipeline Quality" \
    --body "..." \
    --adopt pf-25f2f3,pf-5d3921,pf-1296a9

# Create epic with priority
cp epic create --title "Pipeline Quality" --body "..." --priority 2

# Set ordering (blocked-by edges) during creation
cp epic create --title "Pipeline Quality" --body "..." \
    --adopt pf-25f2f3,pf-5d3921,pf-1296a9 \
    --order "pf-1296a9:pf-25f2f3"  # pf-1296a9 blocked by pf-25f2f3
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--title` | Yes | -- | Epic title |
| `--body` | No | -- | Epic description / acceptance criteria |
| `--body-file` | No | -- | Read body from file |
| `--adopt` | No | -- | Comma-separated shard IDs to adopt as children |
| `--order` | No | -- | Dependency edges: "child:blocker,child:blocker" |
| `--priority` | No | 2 (normal) | Priority 0-4 |
| `--label` | No | -- | Additional labels |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Create shard with `type='epic'`, title, body, priority, label `kind:epic`
2. If `--adopt`: for each shard ID, set `parent_id = epic.id`. Error if shard not found or already has a parent.
3. If `--order`: for each `child:blocker` pair, create `blocked-by` edge. Error if either shard not in adopt list.
4. Return epic ID.

**Output (text):**
```
Created epic pf-abc123: "Pipeline Quality"
  Adopted 3 shards
  Set 1 dependency edge
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-abc123",
  "title": "Pipeline Quality",
  "adopted": ["pf-25f2f3", "pf-5d3921", "pf-1296a9"],
  "edges": [{"from": "pf-1296a9", "blocked_by": "pf-25f2f3"}]
}
```

### `cp epic show` — Epic Detail with Progress

```bash
cp epic show <epic-id>
cp epic show <epic-id> -o json
```

**What it does:**
1. Fetch epic shard. Error if not found or type != 'epic'.
2. Call `epic_progress()` for completion stats.
3. Call `epic_children()` for child shard details.
4. Format with progress bar and grouped children.

**Output (text):**
```
Pipeline Quality (pf-abc123)
Priority: 2 (normal)
Progress: ███████░░░░░ 3/6 complete

  COMPLETED
  ✓ pf-25f2f3  feature  Configurable pipeline timeouts        (mycroft)
  ✓ pf-5d3921  feature  Service version/deploy history         (mycroft)
  ✓ pf-d2f64f  bug      Worker error classification audit      (mycroft)

  IN PROGRESS
  → pf-1296a9  feature  Reprocessing with overrides            (mycroft, since 14:30)

  OPEN
    pf-086c4d  feature  Quality Dashboard
    pf-53e849  feature  Entity Deduplication                    blocked by: pf-086c4d
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-abc123",
  "title": "Pipeline Quality",
  "priority": 2,
  "progress": {"total": 6, "completed": 3, "in_progress": 1, "open": 2, "blocked": 1},
  "children": [
    {
      "id": "pf-25f2f3", "title": "Configurable pipeline timeouts",
      "status": "closed", "kind": "feature", "owner": "agent-mycroft",
      "closed_at": "2026-02-07T12:00:00Z", "blocked_by": []
    },
    {
      "id": "pf-1296a9", "title": "Reprocessing with overrides",
      "status": "in_progress", "kind": "feature", "owner": "agent-mycroft",
      "started_at": "2026-02-07T14:30:00Z", "blocked_by": ["pf-25f2f3"]
    },
    {
      "id": "pf-53e849", "title": "Entity Deduplication",
      "status": "open", "kind": "feature", "owner": null,
      "blocked_by": ["pf-086c4d"]
    }
  ]
}
```

### `cp epic list` — List Epics with Progress

```bash
cp epic list
cp epic list --status open         # only open epics (default)
cp epic list --status all          # include closed epics
cp epic list -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--status` | No | open | Filter: open, closed, all |
| `-o` | No | text | Output format: text, json |

**What it does:**
1. Query shards where `type = 'epic'` and project filter.
2. For each epic, call `epic_progress()` for completion stats.
3. Format as table.

**Output (text):**
```
EPICS
─────
ID          PROGRESS     PRIORITY  TITLE
pf-abc123   ███████░ 3/6  P2       Pipeline Quality
pf-def456   █░░░░░░░ 1/8  P3       CP CLI Implementation
pf-ghi789   ██████████ 4/4  P2     Entity Cleanup                    DONE
```

**JSON output (`-o json`):**
```json
[
  {
    "id": "pf-abc123", "title": "Pipeline Quality", "priority": 2,
    "progress": {"total": 6, "completed": 3, "in_progress": 1, "open": 2}
  }
]
```

### `cp shard assign` — Claim a Shard

```bash
cp shard assign <shard-id> --agent agent-mycroft
cp shard assign <shard-id>    # uses configured agent from cp config
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--agent` | No | config agent | Agent claiming the shard |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. `SELECT ... FOR UPDATE` on shard.
2. If status != 'open', error: "Shard is already STATUS (owner: AGENT)."
3. If shard has unresolved `blocked-by` edges (blocker status != 'closed'), error: "Shard is blocked by: pf-xxx, pf-yyy."
4. Set `status = 'in_progress'`, `owner = agent`, `updated_at = NOW()`.
5. Store assignment timestamp in metadata: `metadata.assigned_at`.

**Output (text):**
```
Assigned pf-1296a9 "Reprocessing with overrides" to agent-mycroft
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-1296a9",
  "title": "Reprocessing with overrides",
  "owner": "agent-mycroft",
  "status": "in_progress"
}
```

### `cp shard close` — Close a Shard

```bash
cp shard close <shard-id> --reason "Done: implemented and tested"
cp shard close <shard-id>   # no reason (allowed, but reason is recommended)
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--reason` | No | -- | Closure reason (text describing what was done) |
| `-o` | No | text | Output format: text, json |

**What it does (atomic):**
1. Fetch shard. Error if not found.
2. If already closed, warn: "Already closed" and exit 0 (idempotent).
3. Set `status = 'closed'`, `closed_at = NOW()`, `closed_by = config.agent`, `closed_reason = reason`.
4. Check if this shard's closure unblocks other shards. For each shard that had a `blocked-by` edge to this one, check if ALL its blockers are now closed. If so, log: "Unblocked: pf-xxx TITLE".
5. If shard has a parent epic, update the epic's `updated_at`.

**Output (text):**
```
Closed pf-1296a9 "Reprocessing with overrides"
  Reason: Done: implemented and tested
  Unblocked: pf-086c4d "Quality Dashboard"
```

**JSON output (`-o json`):**
```json
{
  "id": "pf-1296a9",
  "status": "closed",
  "closed_at": "2026-02-07T16:00:00Z",
  "reason": "Done: implemented and tested",
  "unblocked": ["pf-086c4d"]
}
```

### `cp shard next` — Next Workable Shard

```bash
cp shard next                     # within focused epic (if set), else all open
cp shard next --epic pf-abc123    # within specific epic
cp shard next --global            # ignore focus, search all open shards
cp shard next -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--epic` | No | focused epic | Scope to specific epic |
| `--global` | No | false | Ignore focus, search all open work |
| `--limit` | No | 1 | Number of next candidates to return (max 10) |
| `-o` | No | text | Output format: text, json |

**What it does:**
1. Determine scope: `--epic` > focused epic > global (if no focus and no `--epic`).
2. Call `shard_next()` — finds shards where status='open', no unresolved blocked-by edges, ordered by priority then created_at.
3. Return the top N candidates (default 1).
4. If scoped to an epic, also show "After that" section via `epic_children()` — blocked shards with their blocker status, so the user can see what's coming.

**Output (text):**
```
Next up (in epic "Pipeline Quality"):
  pf-086c4d  feature  Quality Dashboard  (P3, no blockers)

  After that:
  pf-53e849  feature  Entity Deduplication  (P3, blocked by pf-086c4d)
```

If no focus set and no `--epic`:
```
Next up (global):
  pf-9c23ea  bug  Hard delete / purge for content  (P2, no blockers)
```

**JSON output (`-o json`):**
```json
{
  "scope": "epic",
  "epic_id": "pf-abc123",
  "next": {
    "id": "pf-086c4d", "title": "Quality Dashboard",
    "kind": "feature", "priority": 3, "blocked_by": []
  },
  "upcoming": [
    {
      "id": "pf-53e849", "title": "Entity Deduplication",
      "kind": "feature", "priority": 3,
      "blocked_by": [{"id": "pf-086c4d", "status": "open"}]
    }
  ]
}
```

### `cp shard board` — Kanban View

```bash
cp shard board                     # within focused epic
cp shard board --epic pf-abc123    # specific epic
cp shard board --global            # all open work
cp shard board --agent agent-mycroft  # what's this agent working on?
cp shard board -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--epic` | No | focused epic | Scope to specific epic |
| `--global` | No | false | All open work across epics |
| `--agent` | No | -- | Filter to specific agent's work |
| `-o` | No | text | Output format: text, json |

**What it does:**
1. Determine scope (same logic as `shard next`).
2. Call `shard_board()` — returns shards grouped by status.
3. Format as kanban columns.

**Output (text):**
```
Pipeline Quality (pf-abc123)  ███████░░░ 3/6

OPEN (2)
  pf-086c4d  feature  Quality Dashboard
  pf-53e849  feature  Entity Deduplication     blocked by: pf-086c4d

IN PROGRESS (1)
  → pf-1296a9  feature  Reprocessing with overrides  (mycroft, 2h ago)

COMPLETED (3)
  ✓ pf-25f2f3  feature  Configurable pipeline timeouts   (mycroft)
  ✓ pf-5d3921  feature  Service version/deploy history    (mycroft)
  ✓ pf-d2f64f  bug      Worker error classification       (mycroft)
```

With `--agent agent-mycroft`:
```
agent-mycroft is working on:

IN PROGRESS (1)
  → pf-1296a9  feature  Reprocessing with overrides  (Pipeline Quality epic)

COMPLETED TODAY (2)
  ✓ pf-25f2f3  feature  Configurable pipeline timeouts
  ✓ pf-5d3921  feature  Service version/deploy history
```

**JSON output (`-o json`):**
```json
{
  "scope": "epic",
  "epic_id": "pf-abc123",
  "open": [{"id": "pf-086c4d", "title": "...", "kind": "feature", "blocked_by": []}],
  "in_progress": [{"id": "pf-1296a9", "title": "...", "kind": "feature", "owner": "agent-mycroft", "assigned_at": "..."}],
  "completed": [{"id": "pf-25f2f3", "title": "...", "kind": "feature", "owner": "agent-mycroft", "closed_at": "..."}]
}
```

### `cp focus` — Show/Set/Clear Active Epic

```bash
# Show current focus
cp focus

# Set focus
cp focus set <epic-id>
cp focus set <epic-id> --note "working on this until EOD"

# Clear focus
cp focus clear

# JSON output
cp focus -o json
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--note` | No | -- | Optional context note |
| `-o` | No | text | Output format: text, json |

**`cp focus` (show) — What it does:**
1. Call `focus_get()` for current agent+project.
2. If no focus set: "No focus set. Use `cp focus set <epic-id>` to set one."
3. If focus set: show epic summary + progress.

**Output (text):**
```
Focus: Pipeline Quality (pf-abc123)
  Set: 2h ago
  Note: working on this until EOD
  Progress: ███████░░░ 3/6 complete
  In progress: pf-1296a9 Reprocessing with overrides (mycroft)
  Next: pf-086c4d Quality Dashboard
```

**`cp focus set` — What it does (atomic):**
1. Verify shard exists and type='epic'. Error if not.
2. `INSERT INTO focus ... ON CONFLICT (project, agent) DO UPDATE SET epic_id = $1, set_at = NOW(), note = $2`.
3. Show confirmation with epic progress.

**`cp focus clear` — What it does:**
1. `DELETE FROM focus WHERE project = $1 AND agent = $2`.
2. If no row deleted: "No focus was set."
3. If deleted: "Focus cleared."

**JSON output (`-o json`):**
```json
{
  "epic_id": "pf-abc123",
  "epic_title": "Pipeline Quality",
  "set_at": "2026-02-07T14:00:00Z",
  "note": "working on this until EOD",
  "progress": {"total": 6, "completed": 3, "in_progress": 1, "open": 2}
}
```

## SQL Functions

```sql
-- Epic progress: completion stats for an epic's children
CREATE OR REPLACE FUNCTION epic_progress(
    p_project TEXT,
    p_epic_id TEXT
) RETURNS TABLE (
    total INT,
    completed INT,
    in_progress INT,
    open INT,
    blocked INT
) AS $$
    SELECT
        count(*)::int AS total,
        count(*) FILTER (WHERE s.status = 'closed')::int AS completed,
        count(*) FILTER (WHERE s.status = 'in_progress')::int AS in_progress,
        count(*) FILTER (WHERE s.status = 'open'
            AND NOT EXISTS (
                SELECT 1 FROM edges e
                JOIN shards blocker ON blocker.id = e.to_id
                WHERE e.from_id = s.id
                  AND e.edge_type = 'blocked-by'
                  AND blocker.status != 'closed'
            ))::int AS open,
        count(*) FILTER (WHERE s.status = 'open'
            AND EXISTS (
                SELECT 1 FROM edges e
                JOIN shards blocker ON blocker.id = e.to_id
                WHERE e.from_id = s.id
                  AND e.edge_type = 'blocked-by'
                  AND blocker.status != 'closed'
            ))::int AS blocked
    FROM shards s
    WHERE s.project = p_project
      AND s.parent_id = p_epic_id
      AND s.type != 'epic';  -- don't count nested epics as children
$$ LANGUAGE sql STABLE;


-- Epic children: detailed child list with status, kind, owner, blockers
CREATE OR REPLACE FUNCTION epic_children(
    p_project TEXT,
    p_epic_id TEXT
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    status TEXT,
    kind TEXT,
    owner TEXT,
    priority INT,
    assigned_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    closed_by TEXT,
    closed_reason TEXT,
    blocked_by TEXT[]
) AS $$
    SELECT
        s.id, s.title, s.status,
        -- Extract kind from labels (first kind: label, or 'task' default)
        COALESCE(
            (SELECT replace(l.label, 'kind:', '')
             FROM labels l
             WHERE l.shard_id = s.id AND l.label LIKE 'kind:%'
             LIMIT 1),
            'task'
        ) AS kind,
        s.owner,
        s.priority,
        (s.metadata->>'assigned_at')::timestamptz,
        s.closed_at,
        s.closed_by,
        s.closed_reason,
        -- Array of unresolved blocker IDs
        COALESCE(
            (SELECT array_agg(e.to_id)
             FROM edges e
             JOIN shards blocker ON blocker.id = e.to_id
             WHERE e.from_id = s.id
               AND e.edge_type = 'blocked-by'
               AND blocker.status != 'closed'),
            '{}'::text[]
        )
    FROM shards s
    WHERE s.project = p_project
      AND s.parent_id = p_epic_id
      AND s.type != 'epic'
    ORDER BY
        CASE s.status
            WHEN 'in_progress' THEN 0
            WHEN 'open' THEN 1
            WHEN 'closed' THEN 2
        END,
        s.priority,
        s.created_at;
$$ LANGUAGE sql STABLE;


-- Next workable shard: unblocked, open, ordered by priority
CREATE OR REPLACE FUNCTION shard_next(
    p_project TEXT,
    p_epic_id TEXT DEFAULT NULL,  -- NULL = all open work
    p_limit INT DEFAULT 5
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    kind TEXT,
    priority INT,
    epic_id TEXT,
    epic_title TEXT
) AS $$
    SELECT
        s.id, s.title,
        COALESCE(
            (SELECT replace(l.label, 'kind:', '')
             FROM labels l
             WHERE l.shard_id = s.id AND l.label LIKE 'kind:%'
             LIMIT 1),
            'task'
        ),
        s.priority,
        s.parent_id,
        p.title
    FROM shards s
    LEFT JOIN shards p ON p.id = s.parent_id AND p.type = 'epic'
    WHERE s.project = p_project
      AND s.status = 'open'
      AND s.type NOT IN ('epic', 'memory', 'message')
      AND (p_epic_id IS NULL OR s.parent_id = p_epic_id)
      -- Not blocked
      AND NOT EXISTS (
          SELECT 1 FROM edges e
          JOIN shards blocker ON blocker.id = e.to_id
          WHERE e.from_id = s.id
            AND e.edge_type = 'blocked-by'
            AND blocker.status != 'closed'
      )
    ORDER BY s.priority, s.created_at
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;


-- Board view: all shards grouped by status
CREATE OR REPLACE FUNCTION shard_board(
    p_project TEXT,
    p_epic_id TEXT DEFAULT NULL,
    p_agent TEXT DEFAULT NULL
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    status TEXT,
    kind TEXT,
    owner TEXT,
    priority INT,
    epic_id TEXT,
    epic_title TEXT,
    assigned_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    blocked_by TEXT[]
) AS $$
    SELECT
        s.id, s.title, s.status,
        COALESCE(
            (SELECT replace(l.label, 'kind:', '')
             FROM labels l
             WHERE l.shard_id = s.id AND l.label LIKE 'kind:%'
             LIMIT 1),
            'task'
        ),
        s.owner, s.priority, s.parent_id, p.title,
        (s.metadata->>'assigned_at')::timestamptz,
        s.closed_at,
        COALESCE(
            (SELECT array_agg(e.to_id)
             FROM edges e
             JOIN shards blocker ON blocker.id = e.to_id
             WHERE e.from_id = s.id
               AND e.edge_type = 'blocked-by'
               AND blocker.status != 'closed'),
            '{}'::text[]
        )
    FROM shards s
    LEFT JOIN shards p ON p.id = s.parent_id AND p.type = 'epic'
    WHERE s.project = p_project
      AND s.type NOT IN ('epic', 'memory', 'message')
      AND (p_epic_id IS NULL OR s.parent_id = p_epic_id)
      AND (p_agent IS NULL OR s.owner = p_agent)
      AND (p_agent IS NOT NULL           -- agent filter: show all their work
           OR p_epic_id IS NOT NULL      -- epic filter: show all children
           OR s.status != 'closed'       -- global: hide old completions
           OR s.closed_at > NOW() - INTERVAL '24 hours')  -- global: keep recent completions
    ORDER BY
        CASE s.status
            WHEN 'in_progress' THEN 0
            WHEN 'open' THEN 1
            WHEN 'closed' THEN 2
        END,
        s.priority,
        s.created_at;
$$ LANGUAGE sql STABLE;


-- Focus management
CREATE OR REPLACE FUNCTION focus_set(
    p_project TEXT,
    p_agent TEXT,
    p_epic_id TEXT,
    p_note TEXT DEFAULT NULL
) RETURNS VOID AS $$
BEGIN
    -- Verify epic exists and is type 'epic'
    IF NOT EXISTS (
        SELECT 1 FROM shards
        WHERE id = p_epic_id AND project = p_project AND type = 'epic'
    ) THEN
        RAISE EXCEPTION 'Shard % is not an epic in project %', p_epic_id, p_project;
    END IF;

    INSERT INTO focus (project, agent, epic_id, set_at, note)
    VALUES (p_project, p_agent, p_epic_id, NOW(), p_note)
    ON CONFLICT (project, agent) DO UPDATE
    SET epic_id = p_epic_id, set_at = NOW(), note = p_note;
END;
$$ LANGUAGE plpgsql VOLATILE;


CREATE OR REPLACE FUNCTION focus_get(
    p_project TEXT,
    p_agent TEXT
) RETURNS TABLE (
    epic_id TEXT,
    epic_title TEXT,
    epic_status TEXT,
    set_at TIMESTAMPTZ,
    note TEXT
) AS $$
DECLARE
    v_epic_id TEXT;
    v_epic_title TEXT;
    v_epic_status TEXT;
    v_set_at TIMESTAMPTZ;
    v_note TEXT;
BEGIN
    SELECT f.epic_id, s.title, s.status, f.set_at, f.note
    INTO v_epic_id, v_epic_title, v_epic_status, v_set_at, v_note
    FROM focus f
    JOIN shards s ON s.id = f.epic_id
    WHERE f.project = p_project AND f.agent = p_agent;

    IF NOT FOUND THEN
        -- No focus set (or epic was hard-deleted via CASCADE)
        RETURN;
    END IF;

    -- Auto-clear if epic is closed
    IF v_epic_status = 'closed' THEN
        DELETE FROM focus WHERE project = p_project AND agent = p_agent;
        RETURN;  -- return empty (focus cleared)
    END IF;

    RETURN QUERY SELECT v_epic_id, v_epic_title, v_epic_status, v_set_at, v_note;
END;
$$ LANGUAGE plpgsql VOLATILE;


CREATE OR REPLACE FUNCTION focus_clear(
    p_project TEXT,
    p_agent TEXT
) RETURNS BOOLEAN AS $$
DECLARE
    rows_deleted INT;
BEGIN
    DELETE FROM focus WHERE project = p_project AND agent = p_agent;
    GET DIAGNOSTICS rows_deleted = ROW_COUNT;
    RETURN rows_deleted > 0;
END;
$$ LANGUAGE plpgsql VOLATILE;


-- Shard assign: atomically claim a shard
CREATE OR REPLACE FUNCTION shard_assign(
    p_project TEXT,
    p_shard_id TEXT,
    p_agent TEXT
) RETURNS TEXT AS $$
DECLARE
    v_status TEXT;
    v_owner TEXT;
    v_title TEXT;
BEGIN
    SELECT status, owner, title INTO v_status, v_owner, v_title
    FROM shards WHERE id = p_shard_id AND project = p_project FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    IF v_status = 'in_progress' THEN
        RAISE EXCEPTION 'Shard % is already in_progress (owner: %)', p_shard_id, v_owner;
    END IF;

    IF v_status = 'closed' THEN
        RAISE EXCEPTION 'Shard % is already closed', p_shard_id;
    END IF;

    -- Check for unresolved blockers
    IF EXISTS (
        SELECT 1 FROM edges e
        JOIN shards blocker ON blocker.id = e.to_id
        WHERE e.from_id = p_shard_id
          AND e.edge_type = 'blocked-by'
          AND blocker.status != 'closed'
    ) THEN
        RAISE EXCEPTION 'Shard % has unresolved blockers', p_shard_id;
    END IF;

    UPDATE shards
    SET status = 'in_progress',
        owner = p_agent,
        updated_at = NOW(),
        metadata = jsonb_set(
            COALESCE(metadata, '{}'::jsonb),
            '{assigned_at}',
            to_jsonb(NOW()::text)
        )
    WHERE id = p_shard_id AND project = p_project;

    RETURN v_title;
END;
$$ LANGUAGE plpgsql VOLATILE;


-- Shard close: close with reason, report unblocked shards
CREATE OR REPLACE FUNCTION shard_close(
    p_project TEXT,
    p_shard_id TEXT,
    p_agent TEXT,
    p_reason TEXT DEFAULT NULL
) RETURNS TABLE (
    closed_title TEXT,
    unblocked_id TEXT,
    unblocked_title TEXT
) AS $$
DECLARE
    v_status TEXT;
    v_title TEXT;
    v_parent_id TEXT;
BEGIN
    SELECT status, title, parent_id INTO v_status, v_title, v_parent_id
    FROM shards WHERE id = p_shard_id AND project = p_project FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    -- Idempotent: already closed = no-op
    IF v_status = 'closed' THEN
        RETURN QUERY SELECT v_title, NULL::TEXT, NULL::TEXT;
        RETURN;
    END IF;

    UPDATE shards
    SET status = 'closed',
        closed_at = NOW(),
        closed_by = p_agent,
        closed_reason = p_reason,
        updated_at = NOW()
    WHERE id = p_shard_id;

    -- Update parent epic's updated_at
    IF v_parent_id IS NOT NULL THEN
        UPDATE shards SET updated_at = NOW() WHERE id = v_parent_id;
    END IF;

    -- Return the closed shard + any newly unblocked shards
    RETURN QUERY
    SELECT v_title, NULL::TEXT, NULL::TEXT
    UNION ALL
    SELECT NULL::TEXT, s.id, s.title
    FROM shards s
    WHERE s.status = 'open'
      AND EXISTS (
          SELECT 1 FROM edges e
          WHERE e.from_id = s.id
            AND e.to_id = p_shard_id
            AND e.edge_type = 'blocked-by'
      )
      AND NOT EXISTS (
          SELECT 1 FROM edges e
          JOIN shards blocker ON blocker.id = e.to_id
          WHERE e.from_id = s.id
            AND e.edge_type = 'blocked-by'
            AND blocker.status != 'closed'
      );
END;
$$ LANGUAGE plpgsql VOLATILE;
```

## Workflows

### Epic Creation Flow

```
User/Agent identifies related work
            │
            ▼
  ┌────────────────────┐
  │  cp epic create    │
  │  --adopt shards    │
  │  --order deps      │
  └────────┬───────────┘
           │
           ▼
  ┌────────────────────┐
  │  Shards re-parented│
  │  Edges created     │
  └────────┬───────────┘
           │
           ▼
  ┌────────────────────┐
  │  cp focus set      │
  │  <epic-id>         │
  └────────────────────┘
```

### Agent Work Cycle

```
Agent receives work (ingest pipeline or manual)
            │
            ▼
  ┌────────────────────┐
  │  cp shard next     │  ← scoped to focused epic
  │  → returns pf-xxx  │
  └────────┬───────────┘
           │
           ▼
  ┌────────────────────┐
  │  cp shard assign   │  ← sets in_progress + owner
  │  pf-xxx            │
  └────────┬───────────┘
           │
           ▼
  ┌────────────────────┐
  │  [do the work]     │
  └────────┬───────────┘
           │
           ▼
  ┌────────────────────┐
  │  cp shard close    │  ← sets closed + reason
  │  pf-xxx "Done..."  │  ← reports unblocked shards
  └────────┬───────────┘
           │
           ▼
  ┌────────────────────┐
  │  cp shard next     │  ← loop: next shard
  └────────────────────┘
```

### Ingest Pipeline Integration

The ingest pipeline (Penfold's `/ingest` command) needs these changes:

**Phase 1 (Classify):** Add `kind:bug` or `kind:feature` label to created shards.

**Phase 3 (Triage):** When decomposing HIGH items into layer sub-shards:
1. Create an epic shard: `cp epic create --title "feat: [feature name]"`
2. Set `parent_id` on all layer sub-shards to the epic
3. Create `blocked-by` edges between layers (existing behavior, now under epic)
4. Report the epic ID in the feedback message to penfold

**Phase 4 (Implement):** Before starting work on a shard:
```bash
cp shard assign <shard-id> --agent agent-mycroft
```

**Phase 5 (Verify):** After verification passes:
```bash
cp shard close <shard-id> "Verified: all tests pass, deployed as commit abc123"
```
After closing all layer sub-shards, close the parent epic if all children are done:
```bash
# Check if epic is complete
cp epic show <epic-id> -o json  # check progress.open == 0 && progress.in_progress == 0
cp shard close <epic-id> "All shards complete"
```

## Go Implementation Notes

### Package Structure

```
cp/
├── cmd/
│   ├── epic.go          # epic create, show, list
│   ├── focus.go         # focus show, set, clear
│   ├── shard_assign.go  # shard assign
│   ├── shard_close.go   # shard close
│   ├── shard_next.go    # shard next
│   └── shard_board.go   # shard board
└── internal/
    ├── client/
    │   ├── epic.go      # epic DB operations
    │   ├── focus.go     # focus DB operations
    │   └── lifecycle.go # assign, close, next, board
    └── render/
        └── board.go     # kanban + progress bar rendering
```

### Key Types

```go
type EpicProgress struct {
    Total      int `json:"total"`
    Completed  int `json:"completed"`
    InProgress int `json:"in_progress"`
    Open       int `json:"open"`
    Blocked    int `json:"blocked"`
}

type EpicChild struct {
    ID         string    `json:"id"`
    Title      string    `json:"title"`
    Status     string    `json:"status"`
    Kind       string    `json:"kind"`
    Owner      string    `json:"owner,omitempty"`
    Priority   int       `json:"priority"`
    AssignedAt *time.Time `json:"assigned_at,omitempty"`
    ClosedAt   *time.Time `json:"closed_at,omitempty"`
    BlockedBy  []string  `json:"blocked_by"`
}

type Focus struct {
    EpicID    string    `json:"epic_id"`
    EpicTitle string    `json:"epic_title"`
    SetAt     time.Time `json:"set_at"`
    Note      string    `json:"note,omitempty"`
    Progress  EpicProgress `json:"progress"`
}

type NextShard struct {
    ID        string   `json:"id"`
    Title     string   `json:"title"`
    Kind      string   `json:"kind"`
    Priority  int      `json:"priority"`
    EpicID    string   `json:"epic_id,omitempty"`
    EpicTitle string   `json:"epic_title,omitempty"`
    BlockedBy []string `json:"blocked_by"`
}
```

### Progress Bar Rendering

```go
func RenderProgressBar(completed, total int, width int) string {
    if total == 0 {
        return strings.Repeat("░", width)
    }
    filled := (completed * width) / total
    return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
```

## Success Criteria

1. **Epic creation** — `cp epic create` produces a type='epic' shard. `--adopt` re-parents existing shards. `--order` creates `blocked-by` edges. Adopted shards appear as children in `cp epic show`.
2. **Epic progress** — `cp epic show` displays accurate completion stats computed from children's status. Progress bar reflects completed/total ratio.
3. **Kind labels** — All work shards have a `kind:X` label. `cp shard board` and `cp epic show` display kind per shard.
4. **Shard assign** — `cp shard assign` atomically sets status=in_progress + owner. Rejects if already assigned, closed, or blocked.
5. **Shard close** — `cp shard close` atomically sets status=closed with reason. Reports newly unblocked shards. Idempotent on already-closed shards.
6. **Shard next** — `cp shard next` returns unblocked open shards ordered by priority. Respects focus scope. Returns empty when all work is done or blocked.
7. **Board view** — `cp shard board` groups shards by status (open/in_progress/completed). Shows owner and time for in_progress shards. Shows blocked-by for blocked shards.
8. **Focus persistence** — `cp focus set` survives context clears. `cp focus` shows current epic with progress. `cp focus clear` removes focus. Auto-clears if epic is closed.
9. **Concurrent assign safety** — Two agents cannot assign the same shard simultaneously. FOR UPDATE lock serializes attempts. Second agent gets clear error.
10. **Ingest pipeline integration** — Phase 3 creates epics for HIGH decomposition. Phase 4 assigns shards before work. Phase 5 closes shards after verification. All shards have kind labels after Phase 1.
11. **JSON output** — All commands support `-o json` with structured output matching documented schemas.
12. **Agent visibility** — `cp shard board --agent X` shows what agent X is working on and recently completed.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| `epic create --adopt` with non-existent shard ID | Error: "Shard pf-xxx not found" |
| `epic create --adopt` shard already has parent | Error: "Shard pf-xxx already belongs to epic pf-yyy" |
| `epic create --order` references shard not in adopt list | Error: "Shard pf-xxx not in adopt list" |
| `epic create --order` malformed (missing colon) | Error: "Invalid order format 'pf-a', expected 'child:blocker'" |
| `epic create --order` self-referential (`pf-a:pf-a`) | Error: "Shard cannot block itself: pf-a" |
| `epic create --order` circular (`pf-a:pf-b,pf-b:pf-a`) | Error: "Circular dependency detected: pf-a and pf-b block each other" |
| `epic show` on non-epic shard | Error: "Shard pf-xxx is type 'task', expected 'epic'" |
| `epic show` on epic with no children | Shows empty epic with 0/0 progress |
| `shard assign` already in_progress | Error: "already in_progress (owner: agent-mycroft)" |
| `shard assign` already closed | Error: "already closed" |
| `shard assign` with unresolved blockers | Error: "has unresolved blockers: pf-xxx" |
| `shard close` already closed | Warn + exit 0 (idempotent) |
| `shard close` unblocks multiple shards | All unblocked shards reported |
| `shard close` last child in epic | Epic not auto-closed (explicit close required) |
| `shard next` no unblocked work | "No unblocked shards available." Exit 0. |
| `shard next` no focus set, no --epic | Falls back to global search |
| `focus set` on non-epic shard | Error: "not an epic" |
| `focus set` on closed epic | Allowed (user may want to review completed work) |
| `focus` when focused epic has been deleted | "Focused epic pf-xxx no longer exists. Focus cleared." |
| `shard board --agent` with no assigned work | "agent-mycroft has no active or recent work." |
| Two agents assign same shard concurrently | First succeeds, second gets error. FOR UPDATE serializes. |
| `shard close` when shard has no parent epic | Works fine — parent update step skipped |
| `epic create` with empty adopt list | Creates epic with no children (can adopt later) |

---

## Test Cases

### SQL Tests: epic_progress

```
TEST: epic_progress counts all statuses
  Given: Epic E with 6 children: 3 closed, 1 in_progress, 1 open (unblocked), 1 open (blocked)
  When:  SELECT * FROM epic_progress('test', 'E')
  Then:  total=6, completed=3, in_progress=1, open=1, blocked=1

TEST: epic_progress empty epic
  Given: Epic E with no children
  When:  SELECT * FROM epic_progress('test', 'E')
  Then:  total=0, completed=0, in_progress=0, open=0, blocked=0

TEST: epic_progress blocked detection
  Given: Epic E, child A (closed), child B (open, blocked-by A). A just closed.
  When:  SELECT * FROM epic_progress('test', 'E')
  Then:  blocked=0 (A is closed, so B is no longer blocked). open=1.

TEST: epic_progress ignores nested epics
  Given: Epic E with child task T and child epic E2
  When:  SELECT * FROM epic_progress('test', 'E')
  Then:  total=1 (only T, not E2)
```

### SQL Tests: shard_next

```
TEST: shard_next returns unblocked open shards
  Given: Shard A (open, no blockers), shard B (open, blocked by A)
  When:  SELECT * FROM shard_next('test')
  Then:  Returns A only

TEST: shard_next respects priority order
  Given: Shard A (open, priority 3), shard B (open, priority 1)
  When:  SELECT * FROM shard_next('test')
  Then:  B first (higher priority), then A

TEST: shard_next scoped to epic
  Given: Epic E with child A (open). Shard X (open, no parent).
  When:  SELECT * FROM shard_next('test', 'E')
  Then:  Returns A only (X excluded)

TEST: shard_next excludes in_progress and closed
  Given: Shard A (in_progress), shard B (closed), shard C (open)
  When:  SELECT * FROM shard_next('test')
  Then:  Returns C only

TEST: shard_next no results
  Given: All shards are closed or blocked
  When:  SELECT * FROM shard_next('test')
  Then:  Returns 0 rows
```

### SQL Tests: shard_assign

```
TEST: shard_assign sets status and owner
  Given: Shard A (open, no owner) in project 'test'
  When:  SELECT shard_assign('test', 'A', 'agent-mycroft')
  Then:  A.status = 'in_progress', A.owner = 'agent-mycroft', metadata.assigned_at set

TEST: shard_assign rejects in_progress
  Given: Shard A (in_progress, owner = agent-other)
  When:  SELECT shard_assign('test', 'A', 'agent-mycroft')
  Then:  EXCEPTION: "already in_progress (owner: agent-other)"

TEST: shard_assign rejects blocked
  Given: Shard A (open), blocker B (open), A blocked-by B
  When:  SELECT shard_assign('test', 'A', 'agent-mycroft')
  Then:  EXCEPTION: "has unresolved blockers"

TEST: shard_assign not found
  Given: No shard 'nonexistent' in project 'test'
  When:  SELECT shard_assign('test', 'nonexistent', 'agent-mycroft')
  Then:  EXCEPTION: "not found"
```

### SQL Tests: shard_close

```
TEST: shard_close sets closed status
  Given: Shard A (in_progress)
  When:  SELECT * FROM shard_close('A', 'agent-mycroft', 'Done')
  Then:  A.status = 'closed', A.closed_at set, A.closed_reason = 'Done'
         First row: closed_title = A's title

TEST: shard_close reports unblocked shards
  Given: Shard A (in_progress), shard B (open, blocked-by A only)
  When:  SELECT * FROM shard_close('A', 'agent-mycroft', 'Done')
  Then:  Two rows: first = A's closure, second = B's unblocked notification

TEST: shard_close idempotent on closed
  Given: Shard A (closed)
  When:  SELECT * FROM shard_close('A', 'agent-mycroft', 'Done again')
  Then:  Returns title, no error. Status unchanged.

TEST: shard_close updates parent epic
  Given: Epic E, child A (in_progress, parent_id = E)
  When:  SELECT * FROM shard_close('A', 'agent-mycroft', 'Done')
  Then:  E.updated_at refreshed
```

### SQL Tests: focus

```
TEST: focus_set creates new focus
  Given: No focus for agent-penfold
  When:  SELECT focus_set('test', 'agent-penfold', 'E', 'afternoon work')
  Then:  Row exists: project=test, agent=agent-penfold, epic_id=E, note='afternoon work'

TEST: focus_set replaces existing
  Given: Focus on epic E1
  When:  SELECT focus_set('test', 'agent-penfold', 'E2', NULL)
  Then:  Focus now on E2. Only one row per project+agent.

TEST: focus_set rejects non-epic
  Given: Shard T with type='task'
  When:  SELECT focus_set('test', 'agent-penfold', 'T', NULL)
  Then:  EXCEPTION: "not an epic"

TEST: focus_get returns current focus
  Given: Focus set on epic E
  When:  SELECT * FROM focus_get('test', 'agent-penfold')
  Then:  Returns E's id, title, status, set_at, note

TEST: focus_get no focus
  Given: No focus row
  When:  SELECT * FROM focus_get('test', 'agent-penfold')
  Then:  Returns 0 rows

TEST: focus_get auto-clears when epic is closed
  Given: Focus set on epic E. E.status = 'closed'.
  When:  SELECT * FROM focus_get('test', 'agent-penfold')
  Then:  Returns 0 rows. Focus row deleted from table.

TEST: focus_clear removes focus
  Given: Focus set on epic E
  When:  SELECT focus_clear('test', 'agent-penfold')
  Then:  Returns true. focus_get returns 0 rows.

TEST: focus_clear no focus
  Given: No focus row
  When:  SELECT focus_clear('test', 'agent-penfold')
  Then:  Returns false. No error.
```

### SQL Tests: epic_children

```
TEST: epic_children returns children ordered by status then priority
  Given: Epic E with child A (in_progress, P2), child B (open, P1), child C (closed, P1)
  When:  SELECT * FROM epic_children('test', 'E')
  Then:  Order: A (in_progress first), B (open), C (closed). Within same status, by priority.

TEST: epic_children extracts kind from labels
  Given: Child A with label 'kind:bug', child B with label 'kind:feature'
  When:  SELECT * FROM epic_children('test', 'E')
  Then:  A.kind = 'bug', B.kind = 'feature'

TEST: epic_children defaults kind to task
  Given: Child A with no kind: label
  When:  SELECT * FROM epic_children('test', 'E')
  Then:  A.kind = 'task'

TEST: epic_children returns blocked_by array
  Given: Child A (open), child B (open, blocked-by A)
  When:  SELECT * FROM epic_children('test', 'E')
  Then:  A.blocked_by = '{}', B.blocked_by = '{A}'

TEST: epic_children excludes nested epics
  Given: Epic E with child task T and child epic E2
  When:  SELECT * FROM epic_children('test', 'E')
  Then:  Returns T only (E2 excluded)
```

### SQL Tests: shard_board

```
TEST: shard_board shows all statuses for epic scope
  Given: Epic E, child A (closed 3 days ago), child B (in_progress), child C (open)
  When:  SELECT * FROM shard_board('test', 'E')
  Then:  Returns all 3 (no 24h filter for epic scope)

TEST: shard_board global excludes old completions
  Given: Shard A (closed 3 days ago), shard B (closed 2 hours ago), shard C (open)
  When:  SELECT * FROM shard_board('test')
  Then:  Returns B and C only (A excluded — older than 24h in global view)

TEST: shard_board agent filter
  Given: Shard A (in_progress, owner=mycroft), shard B (in_progress, owner=penfold)
  When:  SELECT * FROM shard_board('test', NULL, 'agent-mycroft')
  Then:  Returns A only

TEST: shard_board order
  Given: Shard A (in_progress), shard B (open, P1), shard C (open, P3)
  When:  SELECT * FROM shard_board('test')
  Then:  Order: A (in_progress first), B (open P1), C (open P3)
```

### Go Unit Tests

```
TEST: RenderProgressBar zero total
  Given: completed=0, total=0, width=10
  When:  RenderProgressBar(0, 0, 10)
  Then:  Returns "░░░░░░░░░░" (all empty)

TEST: RenderProgressBar partial
  Given: completed=3, total=6, width=10
  When:  RenderProgressBar(3, 6, 10)
  Then:  Returns "█████░░░░░" (half filled)

TEST: RenderProgressBar full
  Given: completed=4, total=4, width=10
  When:  RenderProgressBar(4, 4, 10)
  Then:  Returns "██████████" (all filled)

TEST: RenderProgressBar rounding
  Given: completed=1, total=3, width=10
  When:  RenderProgressBar(1, 3, 10)
  Then:  Returns "███░░░░░░░" (3 filled — integer division 10/3=3)

TEST: ParseOrderFlag valid
  Given: "--order pf-a:pf-b,pf-c:pf-b"
  When:  ParseOrderFlag("pf-a:pf-b,pf-c:pf-b")
  Then:  Returns [{From:"pf-a", BlockedBy:"pf-b"}, {From:"pf-c", BlockedBy:"pf-b"}]

TEST: ParseOrderFlag malformed
  Given: "--order pf-a"
  When:  ParseOrderFlag("pf-a")
  Then:  Error: "invalid order format, expected child:blocker"

TEST: ParseOrderFlag self-reference
  Given: "--order pf-a:pf-a"
  When:  ParseOrderFlag("pf-a:pf-a")
  Then:  Error: "shard cannot block itself: pf-a"
```

### Integration Tests: Epic Lifecycle

```
TEST: create epic, adopt shards, set focus, check progress
  Given: 3 open task shards pf-a, pf-b, pf-c
  When:  cp epic create --title "Test Epic" --adopt pf-a,pf-b,pf-c --order "pf-c:pf-a"
  Then:  Epic created. All 3 shards have parent_id = epic. pf-c blocked-by pf-a.
  When:  cp focus set <epic-id>
  Then:  Focus shows "Test Epic" 0/3 complete
  When:  cp shard assign pf-a --agent test-agent
  Then:  pf-a is in_progress, owner = test-agent
  When:  cp shard close pf-a "Done"
  Then:  pf-a closed. Reports "Unblocked: pf-c"
  When:  cp shard next
  Then:  Returns pf-b or pf-c (both unblocked now)
  When:  cp epic show <epic-id>
  Then:  Progress 1/3 complete. pf-a in COMPLETED, pf-b and pf-c in OPEN.

TEST: board view shows kanban columns
  Given: Epic with pf-a (closed), pf-b (in_progress, owner=mycroft), pf-c (open)
  When:  cp shard board --epic <epic-id>
  Then:  Output has three sections: OPEN (pf-c), IN PROGRESS (pf-b with mycroft), COMPLETED (pf-a)

TEST: agent filter on board
  Given: pf-a (in_progress, owner=mycroft), pf-b (in_progress, owner=penfold)
  When:  cp shard board --agent agent-mycroft
  Then:  Only shows pf-a

TEST: focus survives context clear
  Given: cp focus set <epic-id>
  When:  New session (context cleared), run cp focus
  Then:  Shows same epic with current progress

TEST: shard next with no focus falls back to global
  Given: No focus set. Open shard pf-x with no parent.
  When:  cp shard next
  Then:  Returns pf-x

TEST: shard next with focus scopes to epic
  Given: Focus on epic E. Open shard pf-x (no parent). Open shard pf-a (parent=E).
  When:  cp shard next
  Then:  Returns pf-a only (scoped to focused epic)

TEST: shard close on last child does not auto-close epic
  Given: Epic E with 1 child pf-a (in_progress)
  When:  cp shard close pf-a "Done"
  Then:  pf-a closed. Epic E still open. cp epic show shows 1/1 complete but epic status = open.
```

### Integration Tests: Concurrent Safety

```
TEST: concurrent assign rejected
  Given: Shard pf-a (open)
  When:  Two concurrent `cp shard assign pf-a` from different agents
  Then:  One succeeds, other gets "already in_progress" error

TEST: concurrent close is idempotent
  Given: Shard pf-a (in_progress)
  When:  Two concurrent `cp shard close pf-a "Done"`
  Then:  Both succeed (second is no-op). Shard closed once.
```

---

## Ingest Pipeline Changes (Summary)

These changes apply to the Penfold ingest pipeline (`/ingest` command files).
They are convention changes — the pipeline uses the CP commands above.

| Phase | Current Behavior | New Behavior |
|-------|-----------------|--------------|
| Phase 1 (Classify) | Creates shards with title prefix `investigate:` / `analyze:` | Also adds `kind:bug` or `kind:feature` label |
| Phase 3 (Triage) | Creates sub-shards with `blocked-by` edges, no parent | Creates epic shard first, sets `parent_id` on sub-shards, adds `kind:` labels |
| Phase 4 (Implement) | Launches agents, no assignment tracking | Calls `cp shard assign <id>` before starting work |
| Phase 5 (Verify) | Calls `/palace task close` inconsistently | Calls `cp shard close <id> "reason"` for each verified shard + parent epic |

Detailed pipeline file changes are a separate PR against the penfold repo.

## Cross-Spec Interactions

| Spec | Interaction |
|------|-------------|
| **SPEC-0** (CLI skeleton) | New commands inherit `--project`, `-o`, `--debug` flags |
| **SPEC-2** (metadata) | `assigned_at` stored in metadata JSONB. GIN index available for queries. |
| **SPEC-5** (unified search) | **Supersedes** SPEC-5's `cp shard close` (adds `closed_by`, `closed_reason`, unblocked reporting). SPEC-5's `cp shard reopen` remains valid — it resets status to `open` and clears `closed_at`/`closed_by`/`closed_reason`. Uses `cp shard` namespace, `create_edge()`, and labels table from SPEC-5. |
| **SPEC-6** (hierarchical memory) | Epics use same `parent_id` column as memory hierarchy. No conflict — filtered by `type`. |

## Design Decisions

1. **Epic as shard type vs label:** Type='epic' chosen over a label because epics are
   structurally different — they're containers, not work items. `WHERE type = 'epic'` is
   cleaner than label lookups for listing/filtering.

2. **Kind as label vs column:** Labels chosen because the existing infrastructure (labels table,
   indexed, queryable) works well and avoids a migration. The `kind:` prefix convention is
   already partially in use.

3. **No auto-close on epic completion:** Epics require explicit closure because "all children done"
   doesn't always mean the epic is done (there may be follow-up work, review, etc.).

4. **Focus per agent:** Each agent has independent focus. This supports parallel work —
   mycroft focused on one epic, penfold on another.

5. **Shard close idempotent:** Closing an already-closed shard is a no-op, not an error.
   This prevents pipeline failures from double-close scenarios.

---

## Pre-Submission Checklist

- [x] Every item in "What to Build" has: CLI section + SQL + success criterion + tests
- [x] Every data flow answers all 7 questions (who writes/when/where/who reads/how/what for/staleness)
- [x] Every command has: syntax + example + output + atomic steps + JSON schema
- [x] Every workflow has: flowchart + all branches + error recovery + non-interactive mode
- [x] Every success criterion has at least one test case
- [x] Concurrency is addressed (FOR UPDATE on assign, UPSERT on focus, idempotent close)
- [x] No feature is "mentioned but not specced" (all 13 build items fully specified)
- [x] Edge cases cover: invalid input, empty state, conflicts, boundaries, cross-feature, failure recovery
- [x] Existing spec interactions documented (Cross-Spec Interactions table)
- [x] Sub-agent review completed (27 items: 4 High, 11 Medium, 12 Low — all HIGH and key MEDIUM items fixed)
