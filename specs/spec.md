# Context-Palace Specification

## Overview

Context-Palace is a real-time, shared memory system for AI agents. It provides persistent, structured storage that agents can use for:

- Task tracking and coordination
- Inter-agent messaging
- Activity logging
- Configuration storage
- Documentation and knowledge sharing

## Design Principles

1. **Everything is a shard** - One universal primitive, not separate tables for issues/comments/configs
2. **Real-time over batch** - PostgreSQL backend, not git-synced files
3. **Schema-flexible** - Core fields + freeform content, not rigid typed schemas
4. **Graph-native** - First-class support for relationships between shards
5. **Multi-agent friendly** - Hash IDs prevent conflicts, edges enable coordination

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      Agents / Humans                      │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                    Context-Palace API                     │
│  (REST/gRPC or direct PostgreSQL via MCP/CLI)            │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                      PostgreSQL                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │   shards    │  │    edges    │  │   labels    │      │
│  └─────────────┘  └─────────────┘  └─────────────┘      │
└─────────────────────────────────────────────────────────┘
```

## Component Specs

| Document | Description |
|----------|-------------|
| [data-model.md](./data-model.md) | Shard structure, fields, edges, ID generation, read receipts |
| [api.md](./api.md) | API philosophy (SQL-first) |
| [agent-protocols.md](./agent-protocols.md) | Messaging, task handoffs, logging conventions |
| [postgres-schema.md](./postgres-schema.md) | DDL, indexes, constraints, common queries |
| [auth-setup.md](./auth-setup.md) | SSL certificate authentication (no passwords) |

## Key Differences from Beads

| Aspect | Beads | Context-Palace |
|--------|-------|----------------|
| Storage | JSONL in git | PostgreSQL |
| Sync | Pull/push git | Real-time |
| Schema | Many typed fields | Minimal core + freeform |
| Primitives | Issues, Comments, Dependencies | Shards + Edges |
| Use cases | Task tracking | Tasks, messages, logs, configs, docs |

## Status

- [x] Data model finalized
- [x] PostgreSQL schema
- [x] Core API (SQL-first, no wrapper)
- [x] Edge operations (link, unlink, traverse)
- [x] Agent protocols (messaging, task handoffs)
- [x] Deployment guide (CLAUDE.md + snippets)

## Open Questions

1. ~~**API surface**~~ - Decided: Direct SQL
2. ~~**Auth model**~~ - Decided: SSL client certificates (see [auth-setup.md](./auth-setup.md))
3. **Notifications** - PostgreSQL LISTEN/NOTIFY? Polling? Or just query when needed?
4. **History** - Full audit trail? Just current state?
