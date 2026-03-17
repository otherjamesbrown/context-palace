# Context Palace — Agent Boilerplate

Copy the relevant section below into your project's `AGENTS.md`, `CLAUDE.md`,
`.cursorrules`, or equivalent agent configuration file.

Replace these placeholders before pasting:

| Placeholder | Replace with | Example |
|-------------|--------------|---------|
| `AGENT_NAME` | Your agent identity | `agent-mycroft` |
| `PROJECT` | Your project name | `mycroft` |
| `PREFIX` | Your project's shard ID prefix | `my` |

---

## Minimal (for CLAUDE.md / .cursorrules)

```markdown
## Context Palace

You are **AGENT_NAME** on project **PROJECT** (shard prefix: `PREFIX-`).

Context Palace is your shared memory and coordination system. Use the `cxp` CLI
to track work, communicate with other agents, store knowledge, and search.

**Config:** `.cxp.yaml` in the project root.

### Quick Reference

| Action | Command |
|--------|---------|
| Check status | `cxp status` |
| See your work | `cxp shard list --type task,bug --status open --assigned-to AGENT_NAME` |
| View a shard | `cxp shard show PREFIX-xxx` |
| Claim work | `cxp shard assign PREFIX-xxx` |
| Update status | `cxp shard status PREFIX-xxx in_progress` |
| Log progress | `cxp shard append PREFIX-xxx --body "update text"` |
| Mark done | `cxp shard status PREFIX-xxx needs-review` |
| Check inbox | `cxp message inbox` |
| Send message | `cxp message send --to agent-name --subject "Subject" --body "Body"` |
| Search | `cxp recall "search query"` |

### Shard Types

| Type | Use for |
|------|---------|
| `task` | Planned work items |
| `bug` | Defects and issues |
| `design` | Plans, proposals, architecture decisions |
| `knowledge` | Versioned reference docs (guides, runbooks, playbooks) |
| `message` | Communication between agents |
| `memory` | Persistent context that survives across sessions |

### Status Flow

```
open → ready → in_progress → needs-review → closed
```

### Labels

Use labels for routing and categorization:
- `to:agent-name` — route to an agent
- `blocked` — can't proceed, needs input
- `focus` — pinned to board focus section
- `needs-review` — done, awaiting verification

### KB Routing Quality

When creating or updating KB shards, write the parent/child `trigger` and
`description` as retrieval-quality text.

Treat them as the shard's interface:
- `trigger` answers: "Read this when you need to know about X, Y, or Z."
- `description` answers: "This shard covers A, B, and C."

Rules:
- Write triggers from the agent's perspective, as task or question language.
- Prefer concrete problem statements over taxonomy labels.
- Include the terms an agent would naturally search for or recognize in a task.
- Be specific enough to distinguish this shard from siblings.
- Update triggers and descriptions when a shard's scope changes.

Bad:
- `trigger`: `database`
- `description`: `DB stuff`

Good:
- `trigger`: `Need PostgreSQL connection details, migration workflow, or test DB setup`
- `description`: `Database URLs, SSL requirements, Alembic usage, local and test setup`
```

---

## Full (for AGENTS.md or detailed agent config)

```markdown
## Context Palace

You are **AGENT_NAME** on project **PROJECT** (shard prefix: `PREFIX-`).

Context Palace is your shared memory and coordination system backed by PostgreSQL.
Use the `cxp` CLI to track work, communicate, store knowledge, and search semantically.

**Config:** `.cxp.yaml` in the project root. Run `cxp status` to verify.

---

### What is a Shard?

A shard is the universal primitive in Context Palace. Every item — task, bug, message,
document — is a shard with a type, status, content, labels, and edges to other shards.

Shard IDs are prefixed by project: `PREFIX-a1b2c3`.

### Shard Types

| Type | Purpose | When to create |
|------|---------|----------------|
| `task` | Planned work items | You have concrete work to do |
| `bug` | Defects, issues | You found something broken |
| `design` | Plans, proposals | You need to document an approach before building |
| `knowledge` | Versioned reference docs | Guides, runbooks, playbooks — long-lived content |
| `message` | Agent communication | You need to tell another agent something |
| `memory` | Persistent agent context | Something you need to remember across sessions |
| `handoff` | Session state transfer | Capturing state so your next session can resume |
| `report` | Summaries, digests | Generated analysis or status reports |

### Status Flow

Shards progress through statuses:

```
open → ready → in_progress → needs-review → closed
```

| Status | Meaning |
|--------|---------|
| `open` | Created, not yet scoped or ready |
| `ready` | Scoped and available for someone to pick up |
| `in_progress` | Claimed and actively being worked on |
| `needs-review` | Work complete, waiting for verification |
| `closed` | Verified and accepted |

### Edges (Relationships)

Shards connect to each other via typed edges:

| Edge | Meaning |
|------|---------|
| `child-of` | Hierarchical — task is child of a design |
| `blocked-by` | Can't proceed until the other shard is resolved |
| `replies-to` | Message threading |
| `discovered-from` | Where this bug/issue was found |
| `references` | Loose association |
| `implements` | This task implements that design |
| `previous-version` | Version chain for knowledge docs |

### Labels

Tags for filtering and routing:

| Pattern | Purpose |
|---------|---------|
| `to:agent-name` | Route shard to an agent |
| `blocked` | Can't proceed, needs input |
| `focus` | Pinned to board focus section |
| `needs-review` | Ready for verification |
| `urgent` | High priority |

---

### CLI Quick Reference

```bash
# Status
cxp status

# Work tracking
cxp shard list --type task,bug --status open --assigned-to AGENT_NAME
cxp shard show PREFIX-xxx
cxp shard create --type task --title "Title" --body "Details"
cxp shard assign PREFIX-xxx
cxp shard status PREFIX-xxx in_progress
cxp shard append PREFIX-xxx --body "Progress update"
cxp shard status PREFIX-xxx needs-review
cxp shard close PREFIX-xxx
cxp bug create "Description" --body "Repro steps"
cxp board

# Knowledge
cxp knowledge create --title "Doc title" --body "Content"
cxp knowledge update PREFIX-xxx --body "New content"   # versioned
cxp knowledge append PREFIX-xxx --body "## New section" # versioned
cxp kb search "query"
cxp kb tree

# Messaging
cxp message inbox
cxp message send --to agent-name --subject "Subject" --body "Body"
cxp message read PREFIX-xxx

# Memory
cxp memory add --title "Remember this" --body "Details"
cxp memory list
cxp recall "semantic search query"

# Edges & labels
cxp shard link PREFIX-aaa PREFIX-bbb --type blocked-by
cxp shard edges PREFIX-xxx
cxp shard label add PREFIX-xxx urgent,backend
```

### Session Start Checklist

At the start of every session:

1. `cxp status` — verify connection
2. `cxp message inbox` — check for messages
3. `cxp shard list --type task,bug --status open --assigned-to AGENT_NAME` — check your work queue
4. `cxp board` — see the full picture

### Workflow

1. **Pick up work**: `cxp shard assign PREFIX-xxx`
2. **Mark in progress**: `cxp shard status PREFIX-xxx in_progress`
3. **Log progress**: `cxp shard append PREFIX-xxx --body "update"`
4. **Complete**: `cxp shard status PREFIX-xxx needs-review`
5. **After approval**: `cxp shard close PREFIX-xxx`

### KB Routing Quality

When creating or updating KB shards, write the parent/child `trigger` and
`description` as retrieval-quality text.

Treat them as the shard's interface:
- `trigger` answers: "Read this when you need to know about X, Y, or Z."
- `description` answers: "This shard covers A, B, and C."

Rules:
- Write triggers from the agent's perspective, as task or question language.
- Prefer concrete problem statements over taxonomy labels.
- Include the terms an agent would naturally search for or recognize in a task.
- Be specific enough to distinguish this shard from siblings.
- Update triggers and descriptions when a shard's scope changes.

Bad:
- `trigger`: `database`
- `description`: `DB stuff`

Good:
- `trigger`: `Need PostgreSQL connection details, migration workflow, or test DB setup`
- `description`: `Database URLs, SSL requirements, Alembic usage, local and test setup`
```

---

## Notes

- The `cxp` binary must be on PATH (typically installed to `~/bin/cxp`).
- Config is read from `.cxp.yaml` in the working directory, then `~/.cxp/config.yaml`.
- SSL certificates for PostgreSQL must be in `~/.postgresql/`.
- See `context-palace.md` in the Context Palace repo for the full reference including
  SQL functions, schema details, and advanced usage.
