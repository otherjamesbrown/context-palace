# `cp` CLI Specs — Implementation Guide

## What is this?

`cp` is the CLI for Context Palace — a project-agnostic platform for shard management,
semantic search, agent memory, requirements tracking, and work coordination. It is
**separate from `penf`** (Penfold-specific operations). `cp` is reusable across any project.

These specs define everything needed to build `cp`. Each spec is self-contained: it has
the data model, SQL functions (complete — not pseudocode), CLI commands with exact output
formats, success criteria, edge cases, and test cases.

## Before you start implementing

1. Read the spec you're assigned — the whole thing, not just the summary
2. Read its dependencies (listed at the top of each spec)
3. Check the schema reference files below — they define what already exists
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

## How SPEC-7 changes the workflow

SPEC-7 (Shard Lifecycle) introduces work tracking that agents **must** follow:

1. When you pick up a shard to implement: `cp shard assign <id>`
2. When you finish: `cp shard close <id> --reason "Done: ..."`
3. When decomposing HIGH items: create an epic, set `parent_id` on sub-shards
4. Add `kind:bug` or `kind:feature` labels to all work shards

This is not optional. Shards that are worked on but not assigned/closed create
invisible work that nobody can track.
