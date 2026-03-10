# Context Palace

A shared memory and coordination system for AI agents. Work tracking, knowledge management, messaging, semantic search, and agent memory — all backed by PostgreSQL.

## What is it?

Context Palace gives AI agents (and humans) a persistent, shared workspace. Instead of separate systems for tasks, messages, docs, and memory, everything lives in one database as **shards** — a universal primitive that can represent any type of work item or content.

Agents use the `cxp` CLI to create shards, send messages, track work, store knowledge, and search across everything semantically.

```
┌──────────────────────────────────────────────────────────────┐
│                      Context Palace                          │
│                                                              │
│  task ──────blocked-by──────► bug                            │
│    │                           │                             │
│    ├── child-of ──► design     ├── discovered-from ──► test  │
│    │                           │                             │
│  knowledge ──references──► message ──replies-to──► message   │
│                                                              │
│  Everything is a shard. Shards connect via edges.            │
└──────────────────────────────────────────────────────────────┘
```

## Core Concepts

### Shards

A **shard** is the universal primitive. Every item in Context Palace is a shard with a type, status, content, and metadata. Shard IDs are prefixed by project (e.g., `pf-a1b2c3` for the penfold project).

| Type | Purpose | Example |
|------|---------|---------|
| `task` | Work items, actionable to-dos | "Fix auth timeout bug" |
| `bug` | Defects, issues to investigate | "API returns 500 on empty input" |
| `design` | Plans, architecture decisions | "Auth redesign proposal" |
| `knowledge` | Versioned reference documents (playbooks, guides, specs) | "Deployment runbook" |
| `message` | Agent-to-agent or human-to-agent communication | "Re: Pipeline config question" |
| `memory` | Persistent agent context that survives across sessions | "Always use UTC for timestamps" |
| `handoff` | Session continuity — captures state for the next session | "Session 42 handoff" |
| `report` | Generated summaries, digests, analysis | "Weekly bug triage report" |
| `session` | Tracks an agent's working session (start/checkpoint/end) | "Steve session 2026-03-08" |

**Key shard types explained:**

- **task vs bug**: Tasks are planned work; bugs are defects discovered during operation. Both track through `open → in_progress → needs-review → closed`.
- **knowledge vs message**: Knowledge documents are versioned, long-lived reference material. Messages are transient communication between agents.
- **memory**: Persists context across agent sessions — things an agent should remember but that don't belong in code or docs. Memories can have triggers and expiry dates.
- **handoff**: When an agent session ends, it writes a handoff shard capturing current state, in-progress work, and next steps so a future session can resume.
- **design**: Proposals and plans that may spawn child tasks.

### Edges

**Edges** are typed relationships between shards:

| Edge Type | Meaning | Example |
|-----------|---------|---------|
| `child-of` | Hierarchical parent-child | Task is child-of a design |
| `blocked-by` | Dependency — can't proceed until resolved | Task blocked-by another task |
| `replies-to` | Message threading | Reply replies-to original message |
| `discovered-from` | Origin tracking | Bug discovered-from a test run |
| `references` | Loose association | Knowledge doc references a design |
| `implements` | Realization | Task implements a design |
| `previous-version` | Version history for knowledge docs | v2 previous-version v1 |
| `triggered-by` | Causal link | Memory triggered-by an event |

### Labels

**Labels** are tags for filtering and routing:

- **Routing**: `to:agent-steve`, `cc:agent-penfold` — message recipients
- **Categories**: `backend`, `cli`, `email`, `infrastructure`
- **Workflow**: `blocked`, `focus`, `needs-review`, `urgent`

### Projects

A **project** is a namespace. Each project has a unique ID prefix:

| Project | Prefix | Description |
|---------|--------|-------------|
| penfold | `pf-` | Penfold platform |
| mycroft | `my-` | Mycroft project |
| context-palace | `cp-` | Context Palace itself |

Shards are scoped to a project. The prefix is auto-applied to shard IDs.

## CLI (`cxp`)

The `cxp` binary is the primary interface. It reads config from `.cp.yaml` (project-local) or `~/.cp/config.yaml` (global).

### Key Commands

```bash
# Connection & status
cxp status                              # Check connection, project, shard counts

# Work tracking
cxp shard list --type task --status open # List open tasks
cxp shard show pf-a1b2c3                # View a shard
cxp shard create --type task --title "Fix bug" --body "Details..."
cxp shard assign pf-a1b2c3              # Claim a shard
cxp shard status pf-a1b2c3 in_progress  # Update status
cxp shard close pf-a1b2c3               # Close a shard
cxp bug create "API 500 error" --body "Steps to reproduce..."
cxp task create "Implement feature" --body "Spec details..."

# Knowledge base
cxp knowledge create --title "Runbook" --body "## Steps..."
cxp knowledge update pf-abc --body "Updated content"  # Creates new version
cxp knowledge append pf-abc --body "## New section"    # Appends to content
cxp kb search "deployment process"       # Semantic search over knowledge
cxp kb tree                              # Browse knowledge hierarchy

# Messaging
cxp message inbox                        # Check unread messages
cxp message send --to agent-steve --subject "Bug report" --body "Details..."
cxp message read pf-a1b2c3              # Read and mark as read

# Agent memory
cxp memory add --title "Use UTC" --body "Always use UTC for timestamps"
cxp memory list                          # List active memories
cxp memory search "timestamp"            # Search memories
cxp recall "pipeline timeout issues"     # Semantic search across all shards

# Edges & relationships
cxp shard link pf-aaa pf-bbb --type blocked-by
cxp shard edges pf-a1b2c3               # View relationships

# Labels
cxp shard label add pf-a1b2c3 urgent,backend
cxp shard label remove pf-a1b2c3 urgent

# Board view
cxp board                                # Grouped view of open work
```

### Configuration

Config precedence: **env vars > `.cp.yaml` > `~/.cp/config.yaml` > defaults**

```yaml
# .cp.yaml (place in project root)
connection:
  host: dev02.brown.chat
  database: contextpalace
  user: penfold
  sslmode: verify-full

agent: agent-myagent
project: myproject

embedding:
  provider: google
  model: gemini-embedding-001
  api_key_env: GEMINI_API_KEY

generation:
  provider: google
  model: gemini-2.0-flash
  api_key_env: GEMINI_API_KEY

defaults:
  output: json
```

Environment variables: `CP_HOST`, `CP_DATABASE`, `CP_USER`, `CP_PROJECT`, `CP_AGENT`.

## Setup

See `setup.md` for step-by-step instructions for Claude Code instances, or `claude-template.md` for manual setup.

### Quick setup

1. Create `.cp.yaml` in your project root (see config above)
2. Ensure SSL certs are in `~/.postgresql/`
3. Run `cxp status` to verify connection
4. Ensure your project is registered in the `projects` table

## Documentation

| File | Description |
|------|-------------|
| `setup.md` | Step-by-step setup for Claude Code instances |
| `context-palace.md` | Full reference guide (SQL functions, schema, conventions) |
| `claude-template.md` | Template for CLAUDE.md integration |
| `agent-boilerplate.md` | Copy-paste block for AGENTS.md / cursor rules |
| `specs/` | Technical specifications |
