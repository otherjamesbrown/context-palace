# Context Palace — `cp` CLI Specs

## If you are an AI agent, read this first

Context Palace is the infrastructure you run on. It provides five core capabilities:

1. **Work management** — Bugs, features, and specs are tracked as shards. Shards are
   grouped into epics. You assign shards when you start work, close them when you finish.
   The system tracks what's in progress, what's blocked, and what's next. (SPEC-3, SPEC-7)

2. **Hierarchical memory** — Your knowledge base is a tree. Root memories give you
   orientation. Sub-memories hold detail you load on demand. When you learn something,
   you store it as a sub-memory with a trigger summary so future agents (or future you
   after a context clear) know when to load it. (SPEC-6)

3. **Inter-agent messaging** — You communicate with other agents and humans through
   message shards. Messages are targeted via `to:agent-name` labels, read via inbox
   queries, and acknowledged via read receipts. This is how you receive work, report
   status, and ask questions. (See [agent-protocols.md](../agent-protocols.md))

4. **Semantic search** — You can search shards by meaning, not just keywords. Shards
   are embedded on creation (pgvector). `cp recall "deployment issues"` finds relevant
   memories, docs, and messages regardless of exact wording. (SPEC-1, SPEC-5)

5. **Versioned knowledge documents** — Reference documentation (architecture docs,
   runbooks, specs) is stored as versioned shards with diffs between versions.
   When a doc is updated, the previous version is preserved. (SPEC-4)

Everything is a **shard** — a row in PostgreSQL with a type, status, labels, metadata,
optional parent, and optional vector embedding. Shards are connected by typed **edges**
(blocked-by, child-of, implements, etc.) forming a graph. The `cp` CLI is how you
interact with all of it.

**`cp` is separate from `penf`.** `penf` handles Penfold-specific operations (email
pipeline, entities, acronyms). `cp` is project-agnostic infrastructure — reusable
across any project that needs work tracking, memory, or agent coordination.

---

## Specs in this directory

These specs define everything needed to build `cp`. Each spec is self-contained: data
model, SQL functions (complete — not pseudocode), CLI commands with exact output formats,
success criteria, edge cases, and test cases.

### Before you start implementing

1. Read the spec you're assigned — the whole thing, not just the summary
2. Read its dependencies (listed at the top of each spec)
3. Read [postgres-schema.md](../postgres-schema.md) — it defines what already exists
4. Follow the spec exactly. If something seems wrong, flag it — don't silently deviate

## Specs

| Spec | Status | Summary |
|------|--------|---------|
| [SPEC-0](SPEC-0-cli.md) | Draft | CLI skeleton, config, DB connection, migrated `palace` commands |
| [SPEC-1](SPEC-1-semantic-search.md) | Draft | pgvector embeddings, `cp recall` for semantic search |
| [SPEC-2](SPEC-2-metadata.md) | Draft | JSONB metadata column on shards, helper functions |
| [SPEC-3](SPEC-3-requirements.md) | Draft | Requirement lifecycle (draft → approved → implemented → verified) |
| [SPEC-4](SPEC-4-knowledge-docs.md) | Draft | Versioned knowledge documents with diffs |
| [SPEC-5](SPEC-5-unified-search.md) | Draft | Unified `cp recall`, shard CRUD, graph edges, labels |
| [SPEC-6](SPEC-6-hierarchical-memory.md) | Draft | Sub-memories, pointer blocks, access telemetry, `cp memory tree` |
| [SPEC-7](SPEC-7-shard-lifecycle.md) | Draft | Epics, focus tracking, shard assign/close, `cp shard next/board` |
| [Tests](test-infrastructure.md) | Draft | Test framework, patterns, CI setup |

## Dependency graph and implementation order

```
PHASE 1 — Foundation (no dependencies, build first)
  SPEC-0  CLI skeleton + config
  SPEC-2  Metadata JSONB column (DB migration only)

PHASE 2 — Features (depend on Phase 1, can build in parallel)
  SPEC-1  Semantic search + embeddings       ← needs SPEC-0
  SPEC-3  Requirement lifecycle              ← needs SPEC-2
  SPEC-4  Knowledge documents                ← needs SPEC-2

PHASE 3 — Integration (depends on Phase 2)
  SPEC-5  Unified search + shard ops         ← needs SPEC-1, SPEC-2

PHASE 4 — Advanced (depends on Phase 3)
  SPEC-6  Hierarchical memory               ← needs SPEC-5, SPEC-1
  SPEC-7  Shard lifecycle + epics + focus    ← needs SPEC-0, SPEC-2, SPEC-5
```

Dependency edges (if A → B, build A first):

```
SPEC-0 ──┬── SPEC-1 ──┬── SPEC-5 ──┬── SPEC-6
          │            │            └── SPEC-7
          └── SPEC-2 ──┼── SPEC-3
                       ├── SPEC-4
                       └── SPEC-5
```

## Schema reference (what already exists)

Read these before implementing. They define the tables, indexes, functions, and
conventions you'll be building on top of.

| File | What's in it |
|------|-------------|
| [../postgres-schema.md](../postgres-schema.md) | Full DDL: `shards`, `labels`, `edges`, `focus` tables. All indexes. All existing SQL functions. **Read this first.** |
| [../data-model.md](../data-model.md) | Shard types, edge types, status lifecycle, label conventions |
| [../api.md](../api.md) | SQL-first API philosophy — no ORM, functions are the API contract |
| [../agent-protocols.md](../agent-protocols.md) | Multi-agent messaging, task assignment, inbox/outbox patterns |
| [../spec.md](../spec.md) | System architecture overview |

## Current database state

- **Tables:** shards, labels, edges, focus, sessions, file_claims, session_events
- **SQL functions:** 15+ (create_shard, send_message, mark_read, semantic_search, etc.)
- **Indexes:** full-text (tsvector), vector (pgvector IVFFlat), GIN (metadata JSONB), B-tree (status, type, owner, parent_id, created_at)
- **Shard types in use:** task, message, memory, backlog, bug, config, design, doc, epic, issue, log, proposal, session
- **Edge types in use:** blocked-by, blocks, child-of, discovered-from, extends, has-artifact, implements, parent, references, relates-to, replies-to, triggered-by
- **Test coverage:** Zero — see [test-infrastructure.md](test-infrastructure.md) for the plan

## Spec conventions

Every spec follows the [SPEC-TEMPLATE.md](SPEC-TEMPLATE.md). The key rule:

> If an item in "What to Build" doesn't have a CLI section, SQL function,
> success criterion, AND test cases — it's not specced, it's a wish.

Each spec contains:
- **SQL functions** — complete, runnable SQL. Copy-paste into a migration file.
- **CLI commands** — exact syntax, flags, example output (text and JSON)
- **Data flow** — for every piece of data: who writes it, when, where, who reads it, how, what decisions it informs, does it go stale
- **Concurrency** — locking strategy for every mutation
- **Edge cases** — table of inputs and expected behavior
- **Test cases** — SQL tests, Go unit tests, integration tests

## How agents use this system

### Memory (SPEC-6)

Agent knowledge lives in hierarchical memory. Root memories provide orientation;
sub-memories hold detail that's loaded on demand.

- **On startup:** load root memories (`cp memory list --roots`). Read their
  sub-memory pointer blocks to understand what detail is available.
- **When you need detail:** follow a pointer — `cp memory show <child-id>`.
  Only load what you need. Every read is tracked (access telemetry).
- **When you learn something new:** store it as a sub-memory under the right
  parent — `cp memory add-sub <parent-id> --title "..." --body "..."`. The
  system generates a trigger summary ("when would you need this?") that helps
  future agents find it.
- **Pointer block format** in parent content:
  ```
  <!-- sub-memories -->
  [
    {"id": "pf-aa2", "title": "Troubleshooting", "summary": "If deploy succeeds but service unchanged"},
    {"id": "pf-aa5", "title": "Rollback", "summary": "Steps to revert a bad deploy"}
  ]
  <!-- /sub-memories -->
  ```
  The summaries describe *when* to load the child, not just *what* it contains.
- **Navigation:** `cp memory tree` shows the full hierarchy. `cp memory hot`
  shows frequently-accessed deep memories that should be promoted upward.

### Work tracking (SPEC-7)

Shard lifecycle tracking that agents **must** follow:

1. When you pick up a shard to implement: `cp shard assign <id>`
2. When you finish: `cp shard close <id> --reason "Done: ..."`
3. When decomposing HIGH items: create an epic, set `parent_id` on sub-shards
4. Add `kind:bug` or `kind:feature` labels to all work shards
5. Check what to work on next: `cp shard next`
6. See current state: `cp shard board` or `cp epic show <id>`

This is not optional. Shards that are worked on but not assigned/closed create
invisible work that nobody can track.

### Focus

Focus is a persistent "active epic" that scopes queries and survives context clears.

```bash
cp focus set <epic-id>       # "I'm working on this epic"
cp focus                     # show current epic + progress
cp shard next                # next unblocked shard within focused epic
cp shard board               # kanban view scoped to focused epic
```

When a human asks "what are we working on?" or "what's next?", the focus
and shard state are the source of truth — not the conversation history.
