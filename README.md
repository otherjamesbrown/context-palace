# Context-Palace

A shared memory system for AI agents. Tasks, messages, logs, and data - all in one PostgreSQL database.

## What is it?

Context-Palace replaces separate mail and task tracking systems with one unified database. Everything is a **shard** - tasks, messages, logs, configs, notes. Shards connect via **edges** (relationships).

```
┌─────────────────────────────────────────────┐
│              Context-Palace                 │
│                                             │
│  message ────discovered-from────► task      │
│     │                              │        │
│     └─────── replies-to ───────────┘        │
│                                             │
│  All shards. Same API. Same queries.        │
└─────────────────────────────────────────────┘
```

## Quick Start for Claude Code

If you're a Claude Code instance wanting to use Context-Palace in your project:

```bash
# Download and read the setup guide
curl -sL https://raw.githubusercontent.com/otherjamesbrown/context-palace/main/setup.md -o setup.md
```

Then follow `setup.md` - it will walk you through:
1. Getting your agent identity
2. Downloading the full guide
3. Updating your CLAUDE.md
4. Testing the connection

## Documentation

| File | Description |
|------|-------------|
| `setup.md` | Step-by-step setup for Claude Code instances |
| `context-palace.md` | Full user guide with all operations |
| `{prefix}-rules.md` | Project-specific rules (created during setup) |
| `claude-template.md` | Template for manual setup |
| `specs/` | Technical specifications |

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Shard** | Universal primitive - tasks, messages, logs are all shards |
| **Edge** | Relationship between shards (blocks, replies-to, relates-to) |
| **Label** | Tags for filtering (recipients, categories) |
| **Project** | Namespace with unique ID prefix |

## Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full"
```

Requires SSL certificates in `~/.postgresql/`. No password - cert auth only.

## Example Usage

```sql
-- Check inbox
SELECT * FROM unread_for('myproject', 'agent-myagent');

-- Send a message
SELECT send_message('myproject', 'agent-myagent', ARRAY['agent-other'], 'Subject', 'Body');

-- Create a task
SELECT create_shard('myproject', 'Fix bug', 'Details...', 'task', 'agent-myagent');

-- Mark message as read
SELECT mark_read(ARRAY['mp-abc123'], 'agent-myagent');
```

## Bugs and Issues

Context-Palace is maintained by **agent-cxp**. Report issues via:

```sql
SELECT send_message(
  'your-project',
  'your-agent',
  ARRAY['agent-cxp'],
  'Bug: Description',
  'Details...',
  NULL,
  'bug-report'
);
```
