# SPEC-3: Requirement Management

**Status:** Draft
**Depends on:** SPEC-2 (metadata column)
**Blocks:** Nothing (SPEC-5 uses it but isn't strictly blocked)

---

## Goal

Structured workflow for managing requirements in Context Palace. Requirements are shards
of type `requirement` with lifecycle tracking, linked to implementation tasks and test
coverage. Agents create requirements. James approves. Implementers build via linked
task shards. Agents verify against success criteria.

## What Exists

- Shard type system — `requirement` is just a new type value
- Edge types: `implements`, `blocked-by`, `references`, `has-artifact`
- `cp backlog` — basic work items without structured criteria or lifecycle
- SPEC-2 metadata — structured fields on shards

## What to Build

1. **`requirement` shard type** with metadata conventions
2. **Lifecycle management** — draft → approved → in_progress → implemented → verified
3. **Linking** — requirements ↔ tasks, requirements ↔ requirements
4. **Test coverage tracking** — test shards linked to requirements
5. **Dashboard** — status overview
6. **`cp requirement` commands** — full CRUD + lifecycle + linking

## Requirement Lifecycle

```
                    James
                   approves
    ┌───────┐     ┌──────────┐     ┌─────────────┐     ┌───────────────┐     ┌──────────┐
    │ draft │ ──▶ │ approved │ ──▶ │ in_progress │ ──▶ │ implemented   │ ──▶ │ verified │
    └───────┘     └──────────┘     └─────────────┘     └───────────────┘     └──────────┘
    Agent          James            Auto: task           Auto: all tasks      Agent
    writes it      signs off        linked               closed               verifies
```

**Explicit transitions:** draft→approved, implemented→verified, any→reopened
**Auto transitions:** approved→in_progress (task linked), in_progress→implemented (all tasks closed)

Lifecycle status stored in `metadata.lifecycle_status`.

## Database Changes

### SQL Functions

```sql
-- Requirement dashboard query
CREATE OR REPLACE FUNCTION requirement_dashboard(p_project TEXT)
RETURNS TABLE (
    id TEXT,
    title TEXT,
    lifecycle_status TEXT,
    priority INT,
    category TEXT,
    task_count_total INT,
    task_count_closed INT,
    test_count INT,
    blocked_by_ids TEXT[],
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
) AS $$
    SELECT
        s.id, s.title,
        COALESCE(s.metadata->>'lifecycle_status', 'draft'),
        COALESCE((s.metadata->>'priority')::int, 3),
        s.metadata->>'category',
        (SELECT count(*) FROM edges e
         WHERE e.to_id = s.id AND e.edge_type = 'implements')::int,
        (SELECT count(*) FROM edges e
         JOIN shards t ON t.id = e.from_id
         WHERE e.to_id = s.id AND e.edge_type = 'implements'
         AND t.status = 'closed')::int,
        (SELECT count(*) FROM edges e
         WHERE e.to_id = s.id AND e.edge_type = 'has-artifact'
         AND EXISTS (SELECT 1 FROM shards a WHERE a.id = e.from_id AND a.type = 'test'))::int,
        (SELECT array_agg(e.to_id) FROM edges e
         WHERE e.from_id = s.id AND e.edge_type = 'blocked-by'
         AND EXISTS (SELECT 1 FROM shards b WHERE b.id = e.to_id
                     AND COALESCE(b.metadata->>'lifecycle_status','draft') != 'verified'))
        FILTER (WHERE TRUE),
        s.created_at,
        s.updated_at
    FROM shards s
    WHERE s.project = p_project
      AND s.type = 'requirement'
      AND s.status != 'closed'
    ORDER BY COALESCE((s.metadata->>'priority')::int, 3), s.created_at;
$$ LANGUAGE sql STABLE;

-- Check for circular dependencies
CREATE OR REPLACE FUNCTION has_circular_dependency(
    p_from TEXT,
    p_to TEXT
) RETURNS BOOLEAN AS $$
    WITH RECURSIVE dep_chain AS (
        SELECT to_id FROM edges WHERE from_id = p_to AND edge_type = 'blocked-by'
        UNION
        SELECT e.to_id FROM edges e
        JOIN dep_chain d ON e.from_id = d.to_id
        WHERE e.edge_type = 'blocked-by'
    )
    SELECT EXISTS (SELECT 1 FROM dep_chain WHERE to_id = p_from);
$$ LANGUAGE sql STABLE;
```

## CLI Surface

```bash
# Create
cp requirement create "Entity Lifecycle Management" \
    --priority 2 \
    --category entity-management \
    --body "## Goal\nLet me reject junk entities..."
# Or from file:
cp requirement create "Entity Lifecycle Management" \
    --priority 2 \
    --category entity-management \
    --body-file req-entity.md

# List
cp requirement list
# Output:
#   ID          STATUS       PRI  CATEGORY             TITLE                          TASKS  TESTS
#   pf-req-01   approved     2    entity-management    Entity Lifecycle Management    0/0    0
#   pf-req-02   draft        1    assertions           Cross-Content Assertion Query  0/0    0

# List with filters
cp requirement list --status approved
cp requirement list --category entity-management
cp requirement list --status draft,approved   # multiple statuses

# Show detail
cp requirement show pf-req-01
# Output:
#   Entity Lifecycle Management
#   ───────────────────────────
#   ID:       pf-req-01
#   Status:   approved
#   Priority: 2
#   Category: entity-management
#   Created:  2026-02-07 by agent-penfold
#
#   Tasks: 0/0 (no implementation tasks linked)
#   Tests: 0
#
#   ## Goal
#   Let me reject junk entities...
#   [full content]
#
#   Edges:
#     blocked-by  pf-req-04  "Structured Error Codes" (in_progress)

# Lifecycle transitions
cp requirement approve pf-req-01
cp requirement verify pf-req-01
cp requirement reopen pf-req-01 --reason "test failures found"

# Link implementation task
cp requirement link pf-req-01 --task pf-task-123
# Creates edge: pf-task-123 --implements--> pf-req-01

# Link test
cp requirement link pf-req-01 --test pf-test-789

# Link dependency
cp requirement link pf-req-05 --depends-on pf-req-03

# Unlink
cp requirement unlink pf-req-01 --task pf-task-123

# Dashboard
cp requirement dashboard
# Output:
#   REQUIREMENTS DASHBOARD
#   ──────────────────────
#   Total: 7    Draft: 3    Approved: 1    In Progress: 2    Implemented: 0    Verified: 1
#
#   BLOCKED:
#     pf-req-05 "Reprocessing Overrides" ← blocked by pf-req-03 (draft), pf-req-04 (in_progress)
#
#   READY FOR IMPLEMENTATION (approved, unblocked):
#     pf-req-01 "Entity Lifecycle Management" (pri 2, entity-management)
#
#   NEEDS VERIFICATION (implemented, untested):
#     (none)
#
#   TEST COVERAGE: 1/7 (14%)
#     pf-req-04 "Structured Error Codes" — 2 tests

# JSON output
cp requirement list -o json
cp requirement dashboard -o json
```

## Success Criteria

1. **Create:** `cp requirement create` creates shard with type=requirement, metadata
   with lifecycle_status=draft. Returns shard ID.
2. **List:** Table shows ID, status, priority, category, title, task counts, test count.
3. **Show:** Full content plus metadata, linked tasks with their status, test count, edges.
4. **Approve:** Only from draft. Sets lifecycle_status=approved.
5. **Verify:** Only from implemented. Sets lifecycle_status=verified.
6. **Reopen:** From any terminal state. Sets to approved. Requires --reason.
7. **Link task:** Creates `implements` edge. If requirement was approved → auto-sets
   lifecycle_status=in_progress.
8. **Link test:** Creates `has-artifact` edge. Test count increments in dashboard.
9. **Link depends-on:** Creates `blocked-by` edge. Circular dependency check rejects loops.
10. **Auto-implemented:** When all linked tasks are closed, lifecycle_status → implemented.
11. **Dashboard:** Shows blocked, ready, needs-verification, and coverage sections.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Approve from non-draft | Error: "Cannot approve: status is 'in_progress', expected 'draft'." |
| Verify from non-implemented | Error: "Cannot verify: status is 'in_progress', expected 'implemented'. N tasks still open." |
| Verify with no tests | Warn: "No tests linked. Use --force to verify without tests." |
| Approve with empty content | Warn: "No success criteria in content. Use --force." |
| Delete with linked tasks | Error: "Has 3 linked tasks. Close or unlink first." |
| Some tasks closed, not all | Status stays in_progress. Dashboard shows "2/5 tasks closed." |
| Circular dependency A→B→A | Error at edge creation: "Circular dependency detected: pf-req-01 → pf-req-03 → pf-req-01." |
| Reopen verified | Allowed → approved. Reason stored in metadata.reopen_reason. |
| Task shard closed triggers auto-status | If that was the last open task → requirement becomes implemented. |
| Link task that doesn't exist | Error: "Shard pf-task-999 not found." |
| Link non-task shard as task | Error: "Shard pf-bug-001 is type 'bug', expected 'task'." |
| Create with invalid priority | Error: "Priority must be 1-7. Got: 9." |
| List with no requirements | Empty table, not error. |

---

## Test Cases

### SQL Tests: requirement_dashboard

```
TEST: dashboard returns all requirements
  Given: 3 requirement shards in project 'test'
  When:  SELECT * FROM requirement_dashboard('test')
  Then:  Returns 3 rows with correct fields

TEST: dashboard calculates task counts
  Given: Requirement R1 with 3 implementing tasks (2 closed, 1 open)
  When:  SELECT * FROM requirement_dashboard('test')
  Then:  R1 has task_count_total=3, task_count_closed=2

TEST: dashboard calculates test counts
  Given: Requirement R1 with 2 test shards linked via has-artifact
  When:  SELECT * FROM requirement_dashboard('test')
  Then:  R1 has test_count=2

TEST: dashboard shows blocked_by
  Given: R2 blocked-by R1 (R1 is draft, not verified)
  When:  SELECT * FROM requirement_dashboard('test')
  Then:  R2 has blocked_by_ids containing R1.id

TEST: dashboard excludes closed requirements
  Given: 3 requirements, 1 with status='closed'
  When:  SELECT * FROM requirement_dashboard('test')
  Then:  Returns 2 rows (closed excluded)

TEST: dashboard sorts by priority then created_at
  Given: R1 (pri 3, created 10am), R2 (pri 1, created 11am), R3 (pri 3, created 9am)
  When:  SELECT * FROM requirement_dashboard('test')
  Then:  Order: R2 (pri 1), R3 (pri 3, earlier), R1 (pri 3, later)
```

### SQL Tests: has_circular_dependency

```
TEST: no cycle returns false
  Given: A blocked-by B, B blocked-by C
  When:  SELECT has_circular_dependency('C', 'A')
  Then:  Returns false (adding A blocked-by C creates no cycle)

TEST: direct cycle returns true
  Given: A blocked-by B
  When:  SELECT has_circular_dependency('A', 'B')
  Then:  Returns true (B blocked-by A would create A→B→A)

TEST: indirect cycle returns true
  Given: A blocked-by B, B blocked-by C
  When:  SELECT has_circular_dependency('A', 'C')
  Then:  Returns true (C blocked-by A would create A→B→C→A)

TEST: no edges returns false
  Given: No blocked-by edges exist
  When:  SELECT has_circular_dependency('A', 'B')
  Then:  Returns false

TEST: self-reference returns true
  Given: No edges
  When:  SELECT has_circular_dependency('A', 'A')
  Then:  Returns true (can't block yourself)
```

### Go Unit Tests: Lifecycle Validation

```
TEST: valid transitions
  Given: Status transition map
  When:  validateTransition("draft", "approved")
  Then:  Returns nil (valid)

TEST: invalid transition draft to verified
  Given: Status transition map
  When:  validateTransition("draft", "verified")
  Then:  Returns error: "Cannot transition from draft to verified"

TEST: valid transition with reopen
  Given: Status transition map
  When:  validateTransition("verified", "approved")  // reopen
  Then:  Returns nil (valid)

TEST: all valid transitions
  Verify these transitions are valid:
    draft → approved
    approved → in_progress
    in_progress → implemented
    implemented → verified
    verified → approved (reopen)
    in_progress → approved (reopen)
    implemented → approved (reopen)

TEST: all invalid transitions
  Verify these transitions are rejected:
    draft → in_progress
    draft → implemented
    draft → verified
    approved → verified
    approved → implemented
```

### Integration Tests: Full Lifecycle

```
TEST: create requirement
  When:  `cp requirement create "Test Req" --priority 2 --category testing`
  Then:  Exit code 0
         Output contains shard ID
         `cp requirement show <id>` shows status=draft, priority=2, category=testing

TEST: approve requirement
  Given: Requirement in draft status
  When:  `cp requirement approve <id>`
  Then:  Exit code 0
         `cp requirement show <id>` shows status=approved

TEST: approve non-draft fails
  Given: Requirement in approved status
  When:  `cp requirement approve <id>`
  Then:  Exit code 1
         Error contains "expected 'draft'"

TEST: link task triggers in_progress
  Given: Requirement in approved status, task shard exists
  When:  `cp requirement link <req-id> --task <task-id>`
  Then:  Edge created
         Requirement status auto-changed to in_progress

TEST: close all tasks triggers implemented
  Given: Requirement in_progress with 2 tasks
  When:  Both tasks closed via `cp task close`
  Then:  Requirement auto-changes to implemented

TEST: verify requirement
  Given: Requirement in implemented status
  When:  `cp requirement verify <id>`
  Then:  Status = verified

TEST: verify without tests warns
  Given: Requirement in implemented status, no test shards linked
  When:  `cp requirement verify <id>`
  Then:  Exit code 1
         Error: "No test coverage. Use --force"

TEST: verify without tests with force
  Given: Same as above
  When:  `cp requirement verify <id> --force`
  Then:  Exit code 0, status = verified

TEST: link circular dependency rejected
  Given: R1 depends-on R2
  When:  `cp requirement link R2 --depends-on R1`
  Then:  Exit code 1
         Error: "Circular dependency detected"

TEST: dashboard output
  Given: 3 requirements in various states
  When:  `cp requirement dashboard`
  Then:  Output contains all sections (blocked, ready, needs verification, coverage)

TEST: reopen verified
  Given: Requirement in verified status
  When:  `cp requirement reopen <id> --reason "found regression"`
  Then:  Status = approved
         metadata.reopen_reason = "found regression"

TEST: reopen without reason fails
  Given: Requirement in verified status
  When:  `cp requirement reopen <id>`
  Then:  Exit code 1
         Error: "--reason is required"
```
