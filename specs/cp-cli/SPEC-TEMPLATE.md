# SPEC-N: [Title]

**Status:** Draft
**Depends on:** [SPEC-X (what), SPEC-Y (what)]
**Blocks:** [SPEC-Z or Nothing]

---

## Goal

<!-- One paragraph. Why does this exist? What problem does it solve? Who benefits? -->

## What Exists

<!-- Bullet list. What's already built that this builds on?
     Include: DB tables/columns, existing CLI commands, existing SQL functions.
     Be specific — "parent_id column on shards (indexed)" not "existing schema". -->

## What to Build

<!-- Numbered list of deliverables. Each item here MUST have a corresponding:
     - CLI Surface section
     - SQL function (if it touches the DB)
     - Success criterion
     - Test cases
     If an item doesn't have all four, it's not specced — it's a wish. -->

## Data Model

### Schema Changes

<!-- New tables, columns, indexes. Full DDL.
     If no schema changes, state explicitly: "No schema changes." -->

### Storage Format

<!-- If data is stored in content/metadata, define the exact format.
     Include: delimiters, JSON schema, field descriptions, max sizes.
     Show a complete example of stored data. -->

### Data Flow

<!-- For EVERY piece of data this spec introduces, answer ALL of these:
     1. WHO writes it? (which command, which agent, automatic?)
     2. WHEN is it written? (on create, on read, on schedule?)
     3. WHERE is it stored? (which table, which column, which field in JSONB?)
     4. WHO reads it? (which command, which query?)
     5. HOW is it queried? (SQL function, metadata lookup, text search?)
     6. WHAT decisions does it inform? (promotion, archival, display?)
     7. DOES it go stale? (what invalidates it, how is it refreshed?)

     If you can't answer all 7 for a data item, the spec has a gap. -->

### Concurrency

<!-- What happens when two agents/users hit the same data simultaneously?
     Specify: isolation level, locking strategy (SELECT FOR UPDATE, optimistic),
     retry behavior. If N/A, state why. -->

## CLI Surface

<!-- For EACH command:
     1. Full syntax with all flags
     2. Example invocations (happy path)
     3. Example output (exact format — text and JSON)
     4. What it does (numbered atomic steps)
     5. Interactive vs non-interactive modes
     6. JSON output schema (for -o json)
-->

### `cp <noun> <verb>` — [Short Description]

```bash
# Example invocations
```

**What it does (atomic):**
1. Step one
2. Step two

**JSON output (`-o json`):**
```json
{
  "field": "type and description"
}
```

<!-- Repeat for each command -->

## Workflows

<!-- For any multi-step process (approval flows, AI-assisted generation, etc.):
     1. ASCII flowchart showing the full path
     2. Every decision point with all branches
     3. Error recovery at each step
     4. What happens on cancel/timeout/failure
     5. Non-interactive alternatives
-->

## SQL Functions

<!-- Complete SQL — not pseudocode. Include:
     - Function signature with all parameters and defaults
     - Return type
     - Full body
     - LANGUAGE and volatility (STABLE/VOLATILE)
     - Indexes it relies on
     - Guard clauses (depth limits on recursive CTEs, etc.)
-->

```sql
CREATE OR REPLACE FUNCTION ...
```

## Go Implementation Notes

### Package Structure

```
cp/
├── cmd/
│   └── ...
└── internal/
    └── ...
```

### Key Types

```go
// Define structs, interfaces, constants
```

### Key Flows

```go
// Show the main function flow with real types, not pseudocode.
// Every external call (DB, AI, embedding) should be visible.
// Error handling paths included.
```

## Success Criteria

<!-- Numbered list. Each criterion must be:
     - Testable (can write a pass/fail test for it)
     - Specific (no "handles errors gracefully" — which errors? what response?)
     - Traceable (maps to a test case below)

     Cross-check: every item in "What to Build" needs at least one criterion here.
     Cross-check: every criterion here needs at least one test case below. -->

## Edge Cases

<!-- Table format. For EACH command, consider:
     - Invalid input (wrong type, missing required, out of range)
     - Empty state (no data, first use)
     - Conflict state (duplicate, already exists, concurrent modification)
     - Boundary conditions (max depth, max size, overflow)
     - Cross-feature interactions (how does this affect other specs?)
     - Failure recovery (what state is left after a partial failure?)
-->

| Case | Expected Behavior |
|------|-------------------|
| ... | ... |

---

## Test Cases

<!-- Three categories, each maps back to success criteria.
     EVERY success criterion must have at least one test.
     After writing tests, re-read success criteria and confirm coverage. -->

### SQL Tests

```
TEST: [descriptive name]
  Given: [setup state]
  When:  [action]
  Then:  [expected result]
```

### Go Unit Tests

```
TEST: [descriptive name]
  Given: [input]
  When:  [function call]
  Then:  [expected output]
```

### Integration Tests

```
TEST: [descriptive name]
  Given: [system state]
  When:  [CLI command]
  Then:  [observable result]
```

---

## Pre-Submission Checklist

<!-- Fill this in BEFORE presenting the spec. Every box must be ticked. -->

- [ ] Every item in "What to Build" has: CLI section + SQL + success criterion + tests
- [ ] Every data flow answers all 7 questions (who writes/when/where/who reads/how/what for/staleness)
- [ ] Every command has: syntax + example + output + atomic steps + JSON schema
- [ ] Every workflow has: flowchart + all branches + error recovery + non-interactive mode
- [ ] Every success criterion has at least one test case
- [ ] Concurrency is addressed (locking, isolation, retry)
- [ ] No feature is "mentioned but not specced" (grep for TODO, TBD, "handles", "manages", "tracks")
- [ ] Edge cases cover: invalid input, empty state, conflicts, boundaries, cross-feature, failure recovery
- [ ] Existing spec interactions documented (does this change behavior defined in other specs?)
- [ ] Sub-agent review completed (for specs >300 lines)
