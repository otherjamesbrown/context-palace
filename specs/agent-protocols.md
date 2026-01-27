# Context-Palace Agent Protocols

How agents should use Context-Palace for coordination, messaging, and task management.

## Agent Identity

Agents identify themselves with a consistent ID string. Convention: `{type}-{name}`

Examples:
- `agent-rusticdesert` (orchestrator)
- `agent-redwolf` (frontend)
- `agent-fuchsiastream` (worker)
- `human-james` (human user)

Use this ID in:
- `creator` when creating shards
- `owner` when claiming tasks
- `agent_id` in read_receipts

## Messaging

### Sending a Message

```sql
INSERT INTO shards (title, content, type, creator)
VALUES (
  'Bug: Search timeout on large queries',
  '## Description\nSearch fails after 30s on queries with >1000 results\n\n## Steps to reproduce\n1. Run search for "test"\n2. Wait 30s\n3. See timeout error',
  'message',
  'agent-redwolf'
)
RETURNING id;
```

### Targeting a Recipient

Use labels for recipients:

```sql
-- Send message
INSERT INTO shards (title, content, type, creator)
VALUES ('Need help with auth flow', 'Can you review...', 'message', 'agent-worker-1')
RETURNING id;

-- Add recipient label
INSERT INTO labels (shard_id, label) VALUES ('cp-newid', 'to:agent-orchestrator');
```

### Check Inbox (Unread Messages)

```sql
SELECT s.* FROM shards s
JOIN labels l ON l.shard_id = s.id
WHERE s.type = 'message'
  AND l.label = 'to:agent-orchestrator'
  AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'agent-orchestrator')
ORDER BY s.created_at;
```

### Mark as Read

```sql
INSERT INTO read_receipts (shard_id, agent_id)
VALUES ('cp-abc123', 'agent-orchestrator')
ON CONFLICT DO NOTHING;
```

### Reply to a Message

```sql
-- Create reply
INSERT INTO shards (title, content, type, creator)
VALUES ('Re: Need help with auth flow', 'I reviewed it, looks good...', 'message', 'agent-orchestrator')
RETURNING id;

-- Link as reply
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-reply-id', 'cp-original-id', 'replies-to');

-- Notify original sender
INSERT INTO labels (shard_id, label) VALUES ('cp-reply-id', 'to:agent-worker-1');
```

### Get Thread (Full Conversation)

```sql
WITH RECURSIVE thread AS (
  -- Start from root message
  SELECT s.*, 0 AS depth FROM shards s WHERE s.id = 'cp-root-msg'

  UNION ALL

  -- Follow replies
  SELECT s.*, t.depth + 1
  FROM shards s
  JOIN edges e ON e.from_id = s.id
  JOIN thread t ON e.to_id = t.id
  WHERE e.edge_type = 'replies-to'
)
SELECT * FROM thread ORDER BY created_at;
```

## Message Categories

Use labels to categorize message types:

| Label | Meaning |
|-------|---------|
| `kind:bug-report` | Bug report, needs triage |
| `kind:feature-request` | Feature request |
| `kind:status-update` | FYI, just acknowledge |
| `kind:question` | Needs response |
| `kind:completion` | Work completed notification |

### Filter Inbox by Kind

```sql
-- Just bug reports I haven't read
SELECT s.* FROM shards s
JOIN labels l1 ON l1.shard_id = s.id AND l1.label = 'to:agent-orchestrator'
JOIN labels l2 ON l2.shard_id = s.id AND l2.label = 'kind:bug-report'
WHERE s.type = 'message'
  AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'agent-orchestrator')
ORDER BY s.created_at;
```

## Linking Messages to Tasks

When a bug report becomes a tracked task, link them:

```sql
-- Create task from bug report
INSERT INTO shards (title, content, type, status, creator, priority)
VALUES ('Fix search timeout', 'From bug report cp-msg123...', 'task', 'open', 'agent-orchestrator', 1)
RETURNING id;

-- Link message to task
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-msg123', 'cp-task456', 'discovered-from');

-- Mark original message as processed
UPDATE shards SET status = 'closed' WHERE id = 'cp-msg123';
```

### Find Task for a Message

```sql
SELECT t.* FROM shards t
JOIN edges e ON e.to_id = t.id
WHERE e.from_id = 'cp-msg123'
  AND e.edge_type = 'discovered-from';
```

### Find Messages that Created a Task

```sql
SELECT m.* FROM shards m
JOIN edges e ON e.from_id = m.id
WHERE e.to_id = 'cp-task456'
  AND e.edge_type = 'discovered-from';
```

## Task Management

### Create a Task

```sql
INSERT INTO shards (title, content, type, status, creator, owner, priority)
VALUES (
  'Implement OAuth2 flow',
  '## Requirements\n- Support Google OAuth\n- Store tokens securely',
  'task',
  'open',
  'human-james',
  'agent-worker-1',
  1
)
RETURNING id;
```

### Claim a Task

```sql
UPDATE shards
SET owner = 'agent-worker-1', status = 'in_progress'
WHERE id = 'cp-task123'
  AND owner IS NULL;  -- Only if unclaimed
```

### Report Completion

```sql
-- Close the task
UPDATE shards
SET status = 'closed', closed_at = NOW(), closed_reason = 'Implemented and tested'
WHERE id = 'cp-task123';

-- Optionally notify orchestrator
INSERT INTO shards (title, content, type, creator)
VALUES ('Completed: OAuth2 flow', 'Task cp-task123 done. PR #42 ready for review.', 'message', 'agent-worker-1')
RETURNING id;

INSERT INTO labels (shard_id, label) VALUES ('cp-newid', 'to:agent-orchestrator');
INSERT INTO labels (shard_id, label) VALUES ('cp-newid', 'kind:completion');
```

### Get Ready Tasks

```sql
SELECT s.* FROM shards s
WHERE s.type = 'task'
  AND s.status = 'open'
  AND s.owner IS NULL  -- Unclaimed
  AND NOT EXISTS (
    SELECT 1 FROM edges e
    JOIN shards blocker ON e.to_id = blocker.id
    WHERE e.from_id = s.id
      AND e.edge_type = 'blocks'
      AND blocker.status != 'closed'
  )
ORDER BY s.priority, s.created_at;
```

## Activity Logging

Agents can log their actions for audit trail:

```sql
INSERT INTO shards (title, content, type, status, creator)
VALUES (
  'Ran database migration',
  'Executed: 003_add_users.sql\nResult: Success\nRows affected: 0',
  'log',
  'closed',  -- Logs are born closed
  'agent-worker-1'
)
RETURNING id;

-- Link to related task
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-log-id', 'cp-task-id', 'relates-to');
```

### Get Activity for a Task

```sql
SELECT l.* FROM shards l
JOIN edges e ON e.from_id = l.id
WHERE e.to_id = 'cp-task123'
  AND l.type = 'log'
ORDER BY l.created_at;
```

## Polling Workflow

Simple polling loop for an agent:

```
1. Check unread messages: SELECT ... WHERE NOT IN read_receipts
2. For each message:
   - Process based on kind label
   - Mark as read
   - Create tasks if needed
   - Reply if needed
3. Check ready tasks: SELECT ... WHERE open AND unclaimed AND not blocked
4. Claim and work on task
5. Report completion
6. Repeat
```

## Templates

Store reusable templates as shards with `type = 'template'`:

```sql
-- Create a bug report template
INSERT INTO shards (title, content, type, status, creator)
VALUES (
  'Bug Report Template',
  '## Description\n\n## Steps to Reproduce\n1. \n2. \n3. \n\n## Expected Behavior\n\n## Actual Behavior\n',
  'template',
  'open',
  'human-james'
)
RETURNING id;

-- Add labels for lookup
INSERT INTO labels (shard_id, label) VALUES ('cp-template-id', 'template:bug');
```

### Create Task from Template

```sql
-- Get template
SELECT content FROM shards s
JOIN labels l ON l.shard_id = s.id
WHERE l.label = 'template:bug' AND s.type = 'template'
LIMIT 1;

-- Create task with template content (agent fills in details)
INSERT INTO shards (title, content, type, status, creator, priority)
VALUES (
  'fix: Search timeout',
  '<filled in template content>',
  'task',
  'open',
  'agent-orchestrator',
  1
)
RETURNING id;
```

## Duplicate Detection

Before creating a task, check for similar existing ones:

```sql
-- Simple keyword search
SELECT id, title, status FROM shards
WHERE type = 'task'
  AND status != 'closed'
  AND (
    title ILIKE '%search%timeout%'
    OR content ILIKE '%search%timeout%'
  )
LIMIT 5;
```

For better matching, enable full-text search:

```sql
-- Add tsvector column (one-time setup)
ALTER TABLE shards ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (to_tsvector('english', coalesce(title,'') || ' ' || coalesce(content,''))) STORED;
CREATE INDEX idx_shards_search ON shards USING GIN(search_vector);

-- Search for duplicates
SELECT id, title, status,
  ts_rank(search_vector, query) AS rank
FROM shards, to_tsquery('english', 'search & timeout') query
WHERE type = 'task'
  AND status != 'closed'
  AND search_vector @@ query
ORDER BY rank DESC
LIMIT 5;
```

## Work Attribution

Context-Palace distinguishes:

| Field | Meaning |
|-------|---------|
| `creator` | Who created the shard (human, agent) |
| `owner` | Who is responsible / working on it |

When an agent completes work:

```sql
-- Close with attribution in reason
UPDATE shards
SET status = 'closed',
    closed_at = NOW(),
    closed_reason = 'Completed by agent-cli-dev: Added timeout wrapper'
WHERE id = 'cp-task123';
```

Query work done by an agent:

```sql
SELECT * FROM shards
WHERE owner = 'agent-cli-dev'
  AND status = 'closed'
ORDER BY closed_at DESC;
```

## Summary: Why Context-Palace Unifies Mail + Tasks

The Beads feedback identified this core problem:

```
┌─────────────┐         ┌─────────────┐
│  Agent Mail │ ──?──── │   Beads     │
└─────────────┘         └─────────────┘
       Manual integration required
```

Context-Palace solves this by making **everything a shard**:

```
┌─────────────────────────────────────────────┐
│              Context-Palace                 │
│                                             │
│  message ────discovered-from────► task      │
│     │                              │        │
│     └─────── replies-to ───────────┘        │
│                                             │
│  Both are shards. Same API. Same queries.   │
└─────────────────────────────────────────────┘
```

No integration layer needed. Agents can:
- Create messages and tasks with the same INSERT
- Link them with edges
- Query across both with the same SELECT
- Close tasks directly (no manual step)
- Track who created vs who worked on it

## Summary: Addressing Agent Mail Pain Points

| Pain Point | Solution |
|------------|----------|
| No unread filter | `read_receipts` table + NOT IN query |
| Manual message→task correlation | `discovered-from` edge linking them |
| Mixed message types | Labels: `kind:bug-report`, `kind:status-update`, etc. |
| No archive | `status = 'closed'` on processed messages |
| Thread progression buried | `replies-to` edges + recursive CTE query |
| Inbox grows indefinitely | Filter by status, or query only unread |

## Summary: Addressing Beads Pain Points

| Pain Point | Solution |
|------------|----------|
| No agent assignment | `owner` field separate from `creator` |
| Beads don't auto-close | Agents run UPDATE directly |
| No mail ↔ bead linking | `discovered-from` edge |
| Manual duplicate detection | Full-text search before create |
| No templates | Template shards with `type = 'template'` |
| No "worked by" field | `owner` + `closed_reason` |
| Two separate systems | **One unified system** |
