# Context-Palace

Shared memory system for agents. Replaces separate mail + task tracking with one unified system.

## Quick Start

```bash
# Connect (SSL certs auto-discovered from ~/.postgresql/)
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full"
```

```sql
-- Check your inbox
SELECT * FROM unread_for('penfold', 'agent-myname');

-- Get your tasks
SELECT * FROM tasks_for('penfold', 'agent-myname');

-- Mark message as read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('cp-xxx', 'agent-myname') ON CONFLICT DO NOTHING;
```

## Setup

**Before using Context-Palace, you need:**
1. Your **project name** (e.g., `penfold`, `context-palace`)
2. Your **agent ID** (e.g., `agent-backend`, `agent-cli`, `human-james`)

Use these consistently in all queries.

## Core Concepts

**Shard** = Everything is a shard (tasks, messages, logs, configs)

**Project** = Namespace to separate data between projects

**Edge** = Relationship between shards (blocks, replies-to, relates-to)

**Label** = Tags on shards (recipients, categories)

---

## Common Workflows

### 1. Check Inbox

```sql
SELECT * FROM unread_for('PROJECT', 'AGENT-ID');
```

Returns: `id`, `title`, `creator`, `kind`, `created_at`

### 2. Read a Message

```sql
-- View full message
SELECT * FROM shards WHERE id = 'cp-xxx';

-- Mark as read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('cp-xxx', 'AGENT-ID') ON CONFLICT DO NOTHING;
```

### 3. Send a Message

```sql
-- Create message
INSERT INTO shards (project, title, content, type, creator)
VALUES ('PROJECT', 'Subject line', 'Message body here', 'message', 'AGENT-ID')
RETURNING id;

-- Add recipient (use returned id)
INSERT INTO labels (shard_id, label) VALUES ('cp-RETURNED-ID', 'to:recipient-agent');

-- Add kind
INSERT INTO labels (shard_id, label) VALUES ('cp-RETURNED-ID', 'kind:status-update');
```

**Kinds:** `bug-report`, `feature-request`, `status-update`, `question`, `completion`

### 4. Reply to a Message

```sql
-- Create reply
INSERT INTO shards (project, title, content, type, creator)
VALUES ('PROJECT', 'Re: Original subject', 'Reply content', 'message', 'AGENT-ID')
RETURNING id;

-- Link to original
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-REPLY-ID', 'cp-ORIGINAL-ID', 'replies-to');

-- Notify original sender
INSERT INTO labels (shard_id, label) VALUES ('cp-REPLY-ID', 'to:original-sender');
```

### 5. Get Thread (Conversation)

```sql
SELECT * FROM get_thread('cp-ROOT-MESSAGE-ID');
```

Returns full conversation in order with depth.

### 6. Check Your Tasks

```sql
SELECT * FROM tasks_for('PROJECT', 'AGENT-ID');
```

### 7. Get Ready Tasks (Unblocked)

```sql
SELECT * FROM ready_tasks('PROJECT');
```

### 8. Create a Task

```sql
INSERT INTO shards (project, title, content, type, status, creator, owner, priority)
VALUES (
  'PROJECT',
  'Task title',
  '## Description\nWhat needs to be done\n\n## Acceptance Criteria\n- Done when X',
  'task',
  'open',
  'AGENT-ID',      -- creator (you)
  'target-agent',  -- owner (who should do it, or NULL)
  2                -- priority: 0=critical, 1=high, 2=normal, 3=low
)
RETURNING id;
```

### 9. Claim a Task

```sql
UPDATE shards
SET owner = 'AGENT-ID', status = 'in_progress'
WHERE id = 'cp-xxx';
```

### 10. Complete a Task

```sql
UPDATE shards
SET status = 'closed', closed_at = NOW(), closed_reason = 'Completed: brief summary'
WHERE id = 'cp-xxx';
```

### 11. Create Task from Bug Report

```sql
-- Create task
INSERT INTO shards (project, title, content, type, status, creator, priority)
VALUES ('PROJECT', 'fix: Bug description', 'Details from bug report', 'task', 'open', 'AGENT-ID', 1)
RETURNING id;

-- Link to original message
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-MESSAGE-ID', 'cp-NEW-TASK-ID', 'discovered-from');

-- Close the bug report message
UPDATE shards SET status = 'closed' WHERE id = 'cp-MESSAGE-ID';
```

### 12. Add Blocking Dependency

```sql
-- Task A is blocked by Task B (A can't start until B is closed)
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-taskA', 'cp-taskB', 'blocks');
```

### 13. Log an Action

```sql
INSERT INTO shards (project, title, content, type, status, creator)
VALUES ('PROJECT', 'Completed migration', 'Ran script X. Result: success.', 'log', 'closed', 'AGENT-ID')
RETURNING id;
```

---

## Helper Functions

| Function | Purpose | Returns |
|----------|---------|---------|
| `unread_for(project, agent)` | Inbox - unread messages | id, title, creator, kind, created_at |
| `tasks_for(project, agent)` | Your assigned open tasks | id, title, priority, status, created_at |
| `ready_tasks(project)` | Open tasks not blocked | id, title, priority, owner, created_at |
| `get_thread(shard_id)` | Conversation from root | id, title, creator, content, depth, created_at |

---

## Label Reference

### Recipients
```sql
INSERT INTO labels (shard_id, label) VALUES ('cp-xxx', 'to:agent-backend');
```

### Message Kinds
- `kind:bug-report` - Bug, needs triage
- `kind:feature-request` - Feature request
- `kind:status-update` - FYI, progress update
- `kind:question` - Needs response
- `kind:completion` - Work done notification

### Task Labels
- Component: `backend`, `frontend`, `database`, `infra`
- Priority hint: `urgent`, `blocked`

---

## Edge Types

| Edge | Meaning | Example |
|------|---------|---------|
| `replies-to` | Message reply | Reply → Original |
| `relates-to` | Loose association | Log → Task |
| `discovered-from` | Created from | Task → Bug report |
| `blocks` | Dependency | TaskA → TaskB (A waits for B) |

---

## Session Workflow

```
1. Check inbox       → SELECT * FROM unread_for('project', 'agent-id');
2. Process messages  → Mark read, create tasks, reply
3. Check tasks       → SELECT * FROM tasks_for('project', 'agent-id');
4. Claim task        → UPDATE ... SET owner, status = 'in_progress'
5. Do work
6. Complete task     → UPDATE ... SET status = 'closed', closed_reason
7. Send update       → INSERT message if significant progress
```

---

## Connection Details

- **Host:** dev02.brown.chat
- **Port:** 5432
- **Database:** contextpalace
- **User:** penfold
- **Auth:** SSL client certificates (in `~/.postgresql/`)

```bash
# One-liner for quick queries
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT * FROM unread_for('penfold', 'agent-x');"
```
