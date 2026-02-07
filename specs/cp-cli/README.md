# `cp` CLI — Context Palace Developer Tooling

## Overview

`cp` is a standalone CLI for Context Palace — project-agnostic developer tooling for
requirements management, knowledge documents, semantic search, agent memory, and work
tracking. It replaces and extends the existing `palace` CLI.

**Separate from `penf`:** `penf` handles Penfold-specific operations (search, pipeline,
entities). `cp` handles Context Palace operations (shards, requirements, knowledge,
memory, messaging). Reusable across any project.

## Specs

| Spec | File | Depends On | Summary |
|------|------|------------|---------|
| SPEC-0 | [SPEC-0-cli.md](SPEC-0-cli.md) | — | CLI skeleton, config, connection, migrated commands |
| SPEC-1 | [SPEC-1-semantic-search.md](SPEC-1-semantic-search.md) | SPEC-0 | pgvector, embedding pipeline, `cp recall` |
| SPEC-2 | [SPEC-2-metadata.md](SPEC-2-metadata.md) | SPEC-0 | JSONB metadata column, conventions |
| SPEC-3 | [SPEC-3-requirements.md](SPEC-3-requirements.md) | SPEC-2 | Requirement lifecycle, dashboard |
| SPEC-4 | [SPEC-4-knowledge-docs.md](SPEC-4-knowledge-docs.md) | SPEC-2 | Versioned knowledge documents |
| SPEC-5 | [SPEC-5-unified-search.md](SPEC-5-unified-search.md) | SPEC-1, SPEC-2 | `cp recall`, shard ops, graph navigation |
| Tests | [test-infrastructure.md](test-infrastructure.md) | — | Test framework, patterns, CI |

## Implementation Order

```
Phase 1 (foundation):
  SPEC-0 (cp CLI skeleton) ──────────────┐
  SPEC-2 (metadata column — DB only) ────┤
                                          │
Phase 2 (parallel features):              │
  SPEC-1 (semantic search) ──────────────┤
  SPEC-3 (requirements) ─────────────────┤
  SPEC-4 (knowledge docs) ───────────────┤
                                          │
Phase 3 (integration):                    │
  SPEC-5 (unified CLI + recall) ─────────┘
```

## Dependency Graph

```
SPEC-0 (CLI) ──────────────────────────────┐
    │                                       │
    ├── SPEC-1 (pgvector + embeddings) ────┤
    │                                       ├──▶ SPEC-5 (recall + shard ops)
    ├── SPEC-2 (metadata) ────────────────┤
    │       │                              │
    │       ├── SPEC-3 (requirements)      │
    │       └── SPEC-4 (knowledge docs)    │
    │                                       │
    └──────────────────────────────────────┘
```

## Existing System Reference

| File | What it covers |
|------|---------------|
| [../spec.md](../spec.md) | System architecture overview |
| [../data-model.md](../data-model.md) | Shard data model |
| [../postgres-schema.md](../postgres-schema.md) | Current DDL, indexes, functions |
| [../api.md](../api.md) | SQL-first API philosophy |
| [../agent-protocols.md](../agent-protocols.md) | Multi-agent messaging and tasks |

## Current State

- **Database:** 7 tables, 15+ SQL functions, full-text search (tsvector)
- **Existing CLI (`palace`):** 5 commands (task get/claim/progress/close, artifact add)
- **Test coverage:** Zero
- **Shard types in use:** 13 (task, message, memory, backlog, bug, config, design, doc, epic, issue, log, proposal, session)
- **Edge types in use:** 12 (blocked-by, blocks, child-of, discovered-from, extends, has-artifact, implements, parent, references, relates-to, replies-to, triggered-by)
