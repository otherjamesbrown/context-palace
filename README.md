# Context Palace

A shared memory and knowledge system for AI agents. Institutional knowledge, work tracking, messaging, and semantic search — all backed by PostgreSQL.

## What is it?

Context Palace gives AI agents a persistent, shared brain. Everything lives in the database as **shards** — a universal primitive that can represent knowledge articles, work items, messages, or memories. Shards connect to each other via typed **edges**, forming trees, dependency graphs, and conversation threads.

There are two distinct systems built on this primitive:

1. **Knowledge Base** — durable institutional memory. Versioned, tree-structured, designed to be navigated and searched across sessions. This is what agents *think* with.
2. **Work Tracking** — ephemeral workflow. Designs, tasks, bugs that get created, worked, and closed. This is how agents *coordinate*.

In real engineering workflows, these systems often form a lifecycle:

`Spec -> Design/Work Shards -> Knowledge Shards`

- **Specs** define intended architecture and constraints while a system is being built
- **Design/task/bug shards** translate that intent into executable work and track drift during implementation
- **Knowledge shards** capture implemented reality after the code is built, tested, and verified

This keeps agents from depending forever on stale build-time specs while still preserving the original design intent.

```
┌─────────────────────────────────────────────────────────────┐
│                      Context Palace                         │
│                                                             │
│  KNOWLEDGE BASE              WORK TRACKING                  │
│  ─────────────               ─────────────                  │
│  playbook (root)             design                         │
│    ├── branch                  ├── task (child)             │
│    │   ├── article               ├── task (child)           │
│    │   └── article             bug                          │
│    └── branch                    └── task (child)           │
│                                                             │
│  Navigated via tree.         Tracked via board & status.    │
│  Searched via kb search.     Lifecycle: open → closed.      │
│                                                             │
│  + memories, messages, handoffs, sessions                   │
└─────────────────────────────────────────────────────────────┘
```

## Knowledge Base — How Agents Think

The knowledge base is a **tree of versioned knowledge shards** that agents navigate to find information. The root of the tree is the **playbook** — a knowledge shard that's always loaded into the agent's context at session start.

### The Retrieval Hierarchy

Agents follow a strict retrieval order. Each tier is progressively more expensive. Don't skip tiers.

#### 1. Hot — Already in Context (free)

Your playbook, session hook output, and any shards loaded earlier in the session. **Check here first** — you probably already know it.

The playbook is designed to be the agent's "table of contents." It lists its children — each with a **trigger** (when to load it) and a **description** (what it covers). If a trigger matches what you're looking for, you already know where to go without any tool call.

Example playbook children (loaded at session start):

```
Branch: Infrastructure
  trigger: "Need server details, ports, deployment, mTLS, or observability"
  description: "Servers, services, deployment scripts, process management"

Branch: Ingest Pipeline
  trigger: "Working on content ingestion, pipeline stages, or classification"
  description: "6 pipelines, stage definitions, context engine, routing"
```

#### 2. Warm — Navigate the KB Tree (cheap)

Follow the playbook's triggers to load the right branch. Each branch lists **its own children** with their own triggers. Follow the tree recursively until you reach the leaf article you need.

```bash
# Load a branch — see its content AND its children with triggers
cxp shard show pf-abc123

# Browse the full tree structure
cxp kb tree
```

This is **structured navigation, not search**. You're following a curated path, not guessing at keywords. It's cheap because each step loads exactly one shard.

The tree is self-navigating because every parent-child link carries routing metadata. **When you add a child to the tree, always set a trigger and description** — without them, the child is invisible to tree navigation:

```bash
# Link a child with routing metadata
cxp knowledge children add pf-parent pf-child \
  --trigger "When working on deployment procedures" \
  --description "Step-by-step deployment runbook for production"
```

Access telemetry is tracked automatically — every `shard show` increments an access counter and logs the read. This data is used to identify which articles are most used and which are going stale.

#### 3. Cold — Search (moderate)

When the tree doesn't cover your question, search:

```bash
# Hybrid BM25 + vector search scoped to the knowledge tree
cxp kb search "pipeline routing configuration"

# Broader semantic search across ALL shard types
cxp recall "how does model selection work"
```

#### 4. Last Resort — Codebase Scan (expensive)

Only use `grep`, `glob`, or sub-agent exploration when the KB genuinely doesn't have the answer. This is for implementation details not captured in knowledge articles — specific function signatures, recent uncommitted changes, etc.

**If you find yourself scanning the codebase for something that should be in the KB, note the gap.** That's a signal a knowledge article needs to be created or updated.

### Building the KB Tree

Knowledge shards are organized into a tree using `child-of` edges. Each edge carries metadata that makes the tree self-navigating:

| Field | Purpose | Required? |
|-------|---------|-----------|
| `trigger` | When should an agent load this child? A routing hint. | Yes — without it, agents can't navigate to this shard |
| `description` | What does this child cover? A brief summary. | Yes — shown in parent listings |
| `ordering` | Display order among siblings (lower = first) | Optional, auto-assigned |

A shard without a parent is an **orphan** — it exists but can't be found by tree navigation, only by search. Orphans are a knowledge gap.

```bash
# Add a child to a branch with full routing metadata
cxp knowledge children add pf-parent pf-child \
  --trigger "When configuring model routing or LLM selection" \
  --description "Model config tables, routing rules, provider fallbacks"

# Create a new knowledge article linked into the tree
# --parent links it, then set routing metadata separately
cxp knowledge create --title "Deployment Runbook" --body "## Steps..." \
  --parent pf-infrastructure
cxp knowledge children add pf-infrastructure pf-newid \
  --trigger "When deploying services" \
  --description "Step-by-step deployment runbook"
```

### Knowledge Shard Versioning

Knowledge shards are **versioned**. Updating a knowledge shard creates a new version — the previous content is preserved.

```bash
# Create a new knowledge article
cxp knowledge create --title "Deployment Runbook" --body "## Steps..."

# Update (creates a new version, preserves history)
cxp knowledge update pf-abc --body "## Updated steps..."

# Append content without replacing
cxp knowledge append pf-abc --body "## New section"

# View version history
cxp knowledge history pf-abc
cxp knowledge diff pf-abc          # Diff between versions
```

### Agent Memory

**Memories** are a separate mechanism from knowledge shards. They persist agent-specific context across sessions — preferences, learned patterns, things to remember that don't belong in the shared KB.

```bash
cxp memory add --title "Use UTC" --body "Always use UTC for timestamps"
cxp memory list                     # List active memories
cxp memory search "timestamp"       # Search memories
```

Memories can have triggers (conditions that make them relevant) and expiry dates. They're personal to an agent, while knowledge shards are shared across agents.

## Work Tracking — How Agents Coordinate

Work tracking uses three shard types with a simple lifecycle:

| Type | Purpose |
|------|---------|
| `design` | Plans and proposals. Defines *what* to build. Spawns child tasks. |
| `task` | Actionable work items. Assigned to an agent, tracked to completion. |
| `bug` | Defects. Like tasks but discovered during operation, not planned. |

**Status lifecycle:** `open → in_progress → needs-review → closed`

When the last child task of a design or bug closes, the parent auto-transitions to `needs-review`.

```bash
# Create work items
cxp design create "Auth redesign" --body "## Overview..."
cxp task create "Implement token refresh" --body "Details..."
cxp bug create "API 500 on empty input" --body "Steps to reproduce..."

# Link a task as a child of a design
cxp shard link pf-task --child-of pf-design

# Track work
cxp shard assign pf-abc             # Claim a shard
cxp shard status pf-abc in_progress # Update status
cxp shard close pf-abc              # Close when done

# View the board
cxp board                           # Grouped view of all open work
cxp shard list --type task --status open
```

## Messaging

Agents communicate through messages. Messages are routed using labels.

```bash
cxp message inbox                                        # Check unread messages
cxp message send agent-steve "Bug report" --body "..."   # Send a message
cxp message read pf-abc                                  # Read and mark as read
```

## Core Primitives

### Shards

A **shard** is the universal primitive. Every item has a type, status, content, and metadata. Shard IDs are prefixed by project (e.g., `pf-a1b2c3` for the penfold project).

| Type | Purpose |
|------|---------|
| `knowledge` | Versioned reference documents — playbooks, guides, runbooks |
| `design` | Plans and architecture decisions |
| `task` | Actionable work items |
| `bug` | Defects and issues |
| `message` | Agent-to-agent communication |
| `memory` | Persistent agent context across sessions |
| `handoff` | Session state capture for continuity |
| `session` | Tracks an agent's working session |

### Edges

Typed relationships between shards:

| Edge Type | Meaning |
|-----------|---------|
| `child-of` | Tree structure — knowledge branches, task hierarchy |
| `blocked-by` | Dependency — can't proceed until resolved |
| `replies-to` | Message threading |
| `discovered-from` | Origin tracking (bug from test) |
| `references` | Loose association |
| `implements` | Task implements a design |
| `previous-version` | Version history chain |

### Labels

Tags for filtering and routing:

- **Routing**: `to:agent-steve`, `cc:agent-penfold`
- **Categories**: `backend`, `cli`, `infrastructure`
- **Workflow**: `blocked`, `focus`, `urgent`

### Projects

A **project** is a namespace with a unique ID prefix. Shards are scoped to a project; the prefix is auto-applied to shard IDs.

## Configuration

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

1. Create `.cp.yaml` in your project root
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
