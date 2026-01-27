# Context-Palace Data Model

## Core Concept: Shards

A **shard** is the universal primitive in Context-Palace. Everything is a shard: tasks, messages, logs, configs, architecture docs, notes.

## Shard Fields

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Hash-based unique ID (e.g., `cp-a1b2c3`) |
| `project` | string | Project namespace (e.g., `penfold`, `context-palace`) |
| `title` | string | Short summary (required, max 500 chars) |
| `content` | text | The main payload - markdown, JSON, whatever |
| `status` | enum | `open`, `in_progress`, `closed` |
| `creator` | string | Who created this shard (agent ID, user ID) |
| `created_at` | timestamp | When created |
| `updated_at` | timestamp | Last modified |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `owner` | string | Who's responsible for this shard (can differ from creator) |
| `type` | string | Freeform type hint: `task`, `message`, `log`, `config`, `doc`, etc. |
| `labels` | string[] | Tags for filtering/grouping |
| `priority` | int | 0-4 (optional, only meaningful for task-like shards) |
| `parent_id` | string | For hierarchical organization (epic → task → subtask) |
| `closed_at` | timestamp | When status changed to closed |
| `closed_reason` | string | Why it was closed |

## Relationships (Edges)

Shards connect via typed edges stored in a separate table.

### Edge Structure

| Field | Type | Description |
|-------|------|-------------|
| `from_id` | string | Source shard ID |
| `to_id` | string | Target shard ID |
| `edge_type` | string | Relationship type |
| `created_at` | timestamp | When edge was created |
| `metadata` | jsonb | Optional edge-specific data |

### Built-in Edge Types

| Type | Meaning | Blocks? |
|------|---------|---------|
| `blocks` | Target cannot proceed until source closes | Yes |
| `parent-child` | Hierarchical containment | Yes (parent waits for children) |
| `replies-to` | Conversation threading | No |
| `relates-to` | Loose association | No |
| `supersedes` | This shard replaces target | No |
| `discovered-from` | Task created from this message/report | No |

Custom edge types are allowed - just use any string.

## ID Generation

IDs are generated as: `{prefix}-{hash}`

- `prefix`: Configurable per instance (default: `cp`)
- `hash`: First 6 chars of SHA256 of (timestamp + random + content hash)

Examples: `cp-a1b2c3`, `cp-f8e9d0`

This prevents merge conflicts when multiple agents create shards simultaneously.

## Status Lifecycle

```
open → in_progress → closed
  ↑         ↓
  └─────────┘ (reopen)
```

## Type Conventions (not enforced, just suggested)

| Type | Use Case |
|------|----------|
| `task` | Work to be done |
| `message` | Agent-to-agent or human-to-agent communication |
| `log` | Agent activity log entry |
| `config` | Configuration blob |
| `doc` | Architecture decision, spec, documentation |
| `note` | Freeform note |

Since `type` is freeform, agents/users can define their own.

## Example Shards

### Task
```json
{
  "id": "cp-a1b2c3",
  "title": "Implement user authentication",
  "content": "## Requirements\n- OAuth2 support\n- Session management\n\n## Acceptance Criteria\n- User can log in via Google",
  "status": "open",
  "type": "task",
  "creator": "human-james",
  "owner": "agent-claude-1",
  "priority": 1,
  "labels": ["backend", "auth"]
}
```

### Message (conversation)
```json
{
  "id": "cp-d4e5f6",
  "title": "Re: Need help with auth flow",
  "content": "I've reviewed the OAuth spec. Suggest we use PKCE flow for security.",
  "status": "closed",
  "type": "message",
  "creator": "agent-claude-2"
}
// Edge: cp-d4e5f6 --replies-to--> cp-c3b2a1
```

### Log Entry
```json
{
  "id": "cp-g7h8i9",
  "title": "Completed database migration",
  "content": "Ran migration 003_add_users_table.sql successfully. 0 errors.",
  "status": "closed",
  "type": "log",
  "creator": "agent-claude-1",
  "labels": ["migration", "database"]
}
```

### Config
```json
{
  "id": "cp-j0k1l2",
  "title": "Production database config",
  "content": "{\"host\": \"db.example.com\", \"port\": 5432, \"pool_size\": 20}",
  "status": "open",
  "type": "config",
  "creator": "human-james",
  "labels": ["production", "database"]
}
```

## Querying

Common query patterns:

- **Ready tasks**: `status = 'open' AND type = 'task' AND NOT blocked`
- **My work**: `owner = 'agent-X' AND status != 'closed'`
- **Thread**: Follow `replies-to` edges from a shard
- **Children**: `parent_id = 'cp-xxx'`
- **Activity log**: `type = 'log' AND creator = 'agent-X' ORDER BY created_at DESC`

## Read Receipts

Read/unread status is **per-agent**, not per-shard. A message might be read by agent-A but not agent-B.

### Read Receipt Table

| Field | Type | Description |
|-------|------|-------------|
| `shard_id` | string (FK) | The shard that was read |
| `agent_id` | string | Who read it |
| `read_at` | timestamp | When they read it |

Primary key: `(shard_id, agent_id)`

### Querying Unread Shards

```sql
-- Unread messages for agent-X
SELECT s.*
FROM shards s
WHERE s.type = 'message'
  AND s.id NOT IN (
    SELECT shard_id FROM read_receipts WHERE agent_id = 'agent-X'
  )
ORDER BY s.created_at;
```

### Marking as Read

```sql
INSERT INTO read_receipts (shard_id, agent_id, read_at)
VALUES ('cp-a1b2c3', 'agent-X', NOW())
ON CONFLICT (shard_id, agent_id) DO NOTHING;
```

### Use Cases

- Agent inbox: "Show me unread messages"
- Task notifications: "Show me tasks assigned to me I haven't seen"
- Bulk mark-read: After processing a batch of shards

## Open Questions

1. **Soft delete?** Tombstone status vs actual delete?
2. **Content size limits?** Or rely on PostgreSQL's natural limits?
3. **Full-text search on content?** PostgreSQL tsvector?
4. **Read receipt cleanup?** Prune old receipts for closed shards?
