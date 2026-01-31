# Palace CLI

A simplified CLI for sub-agents to interact with context-palace tasks and artifacts.

---

## Quick Start

```bash
# Set required environment variables
export PALACE_USER=penfold
export PALACE_AGENT=[agent-YOURNAME]

# Or use config file ~/.palace.yaml
```

---

## Commands

### Get Task Details

```bash
palace task get [PREFIX]-xxx
```

Output includes: id, title, content, status, owner, priority, artifacts.

### Claim a Task

```bash
palace task claim [PREFIX]-xxx
```

Sets you as owner and status to `in_progress`. Idempotent if you already own it.

### Log Progress

```bash
palace task progress [PREFIX]-xxx "Found the bug in auth.go line 45"
```

Appends a timestamped note to the task content.

### Close a Task

```bash
palace task close [PREFIX]-xxx "Fixed OAuth token refresh"
```

Sets status to `closed` with completion summary.

### Add Artifact

```bash
palace artifact add [PREFIX]-xxx <type> <reference> "description"
```

Types: `commit`, `file`, `pr`, `url`, `deploy`, or any custom type.

Examples:
```bash
palace artifact add [PREFIX]-xxx commit abc123 "Fixed null pointer bug"
palace artifact add [PREFIX]-xxx file services/oauth.go "Modified refresh logic"
palace artifact add [PREFIX]-xxx pr https://github.com/org/repo/pull/42 "PR link"
```

---

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PALACE_USER` | Yes | - | Database user |
| `PALACE_AGENT` | Yes | - | Your agent name (e.g., `[agent-YOURNAME]`) |
| `PALACE_HOST` | No | `dev02.brown.chat` | Database host |
| `PALACE_DB` | No | `contextpalace` | Database name |
| `PALACE_PROJECT` | No | `penfold` | Project name |

### Config File

Create `~/.palace.yaml`:

```yaml
host: dev02.brown.chat
database: contextpalace
user: penfold
project: [YOURPROJECT]
agent: [agent-YOURNAME]
```

Environment variables override config file values.

---

## Output Formats

### Human-Readable (default)

```bash
palace task get [PREFIX]-xxx
```

```
ID:       [PREFIX]-xxx
Title:    Fix OAuth token refresh
Status:   in_progress
Owner:    [agent-YOURNAME]
Priority: 1
Created:  2026-01-31 10:30:00

--- Content ---
The OAuth tokens are not persisting...

--- Artifacts ---
  [commit] abc123: Fixed null pointer bug
```

### JSON

```bash
palace --json task get [PREFIX]-xxx
```

```json
{
  "id": "[PREFIX]-xxx",
  "title": "Fix OAuth token refresh",
  "status": "in_progress",
  "owner": "[agent-YOURNAME]",
  "artifacts": [...]
}
```

---

## Typical Workflow

```bash
# 1. Get task details
palace task get [PREFIX]-123

# 2. Claim the task
palace task claim [PREFIX]-123

# 3. Work on it, log progress
palace task progress [PREFIX]-123 "Found bug in oauth.go line 45"

# 4. Add artifacts as you work
palace artifact add [PREFIX]-123 file services/oauth.go "Modified refresh logic"
palace artifact add [PREFIX]-123 commit abc123 "Fixed token refresh"

# 5. Close when done
palace task close [PREFIX]-123 "Fixed OAuth token refresh - tokens now persist correctly"
```

---

## Installation

```bash
cd palace
go build -o palace .

# Optional: install globally
sudo cp palace /usr/local/bin/
```

---

## Error Handling

The CLI provides clear error messages without exposing SQL:

```
Error: PALACE_USER is required (set via environment or ~/.palace.yaml)
Error: task not found: pf-999
Error: failed to connect to database: check your configuration and network
```

---

## When to Use Palace vs psql

| Use Palace | Use psql |
|------------|----------|
| Get task details | Complex queries |
| Claim tasks | Create shards |
| Log progress | Send messages |
| Close tasks | Relate shards |
| Add artifacts | Inbox management |

Palace is for **sub-agents** doing focused task work. Orchestrators use **psql** for full access.

---

## Syncing Documentation

Use `palace-sync-docs` to fetch the latest context-palace.md with your values filled in:

```bash
# Create config in your working directory
cat > .palace.yaml << 'EOF'
project: penfold
agent: agent-yourname
prefix: pf
EOF

# Fetch and personalize docs
palace-sync-docs

# Preview changes without writing
palace-sync-docs --check
```

This script:
1. Reads `.palace.yaml` from current directory
2. Fetches latest `context-palace.md` from the repo
3. Replaces template values (PROJECT, agent-NAME, PREFIX-) with your values
4. Writes personalized `context-palace.md` to current directory
