# Context-Palace

Context-Palace is the shared memory system for all agents. Use it for:
- **Tasks**: Work to be done
- **Messages**: Communication between agents/humans
- **Logs**: Activity tracking

## Connection

Uses shared SSL certificates from `~/.postgresql/` (see `~/infrastructure.md`).

```bash
# Interactive
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full"

# Single command
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "YOUR SQL HERE"
```

No password needed - SSL client certificate provides authentication.

## Your Agent ID

Use a consistent ID format: `agent-{your-name}` (e.g., `agent-backend`, `agent-frontend`, `agent-rusticdesert`)

## Project

Always include `project` when creating shards. This separates data between projects.
- `penfold` - Penfold backend project
- `context-palace` - Context Palace itself
- etc.

Always filter by project in queries to avoid seeing other projects' data.

## Quick Reference

### Check your inbox (unread messages)
```sql
SELECT id, title, creator, created_at FROM shards
WHERE project = 'YOUR-PROJECT'
  AND type = 'message'
  AND id IN (SELECT shard_id FROM labels WHERE label = 'to:YOUR-AGENT-ID')
  AND id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'YOUR-AGENT-ID')
ORDER BY created_at;
```

### Mark message as read
```sql
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('cp-xxxxx', 'YOUR-AGENT-ID') ON CONFLICT DO NOTHING;
```

### Get your assigned tasks
```sql
SELECT id, title, priority, status FROM shards
WHERE project = 'YOUR-PROJECT'
  AND type = 'task' AND owner = 'YOUR-AGENT-ID' AND status != 'closed'
ORDER BY priority, created_at;
```

### Create a task
```sql
INSERT INTO shards (project, title, content, type, status, creator, owner, priority)
VALUES ('YOUR-PROJECT', 'Task title', 'Details...', 'task', 'open', 'YOUR-AGENT-ID', 'target-agent', 1)
RETURNING id;
```

### Send a message
```sql
INSERT INTO shards (project, title, content, type, creator)
VALUES ('YOUR-PROJECT', 'Subject', 'Message body...', 'message', 'YOUR-AGENT-ID')
RETURNING id;

-- Add recipient
INSERT INTO labels (shard_id, label) VALUES ('cp-newid', 'to:recipient-agent-id');

-- Add kind label
INSERT INTO labels (shard_id, label) VALUES ('cp-newid', 'kind:status-update');
```

### Close a task
```sql
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Completed: brief description'
WHERE id = 'cp-xxxxx';
```

### Claim a task
```sql
UPDATE shards SET owner = 'YOUR-AGENT-ID', status = 'in_progress'
WHERE id = 'cp-xxxxx' AND (owner IS NULL OR owner = 'YOUR-AGENT-ID');
```

## Label Conventions

### Recipients
- `to:agent-backend` - Message intended for agent-backend
- `to:human-james` - Message intended for James

### Message kinds
- `kind:bug-report` - Bug report, needs triage
- `kind:feature-request` - Feature request
- `kind:status-update` - Progress update (FYI)
- `kind:question` - Needs response
- `kind:completion` - Work completed notification

### Task labels
- `backend`, `frontend`, `database` - Component
- `priority:high`, `priority:low` - Additional priority signal
- `blocked` - Waiting on something

## Linking Shards

```sql
-- Message relates to a task
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-message', 'cp-task', 'relates-to');

-- Bug report became a task
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-bugreport', 'cp-task', 'discovered-from');

-- Task B blocks Task A (A can't start until B is done)
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-taskA', 'cp-taskB', 'blocks');

-- Reply to a message
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-reply', 'cp-original', 'replies-to');
```

## Workflow

1. **Start of session**: Check inbox for unread messages
2. **Process messages**: Mark as read, create tasks if needed, reply if needed
3. **Check tasks**: Get your assigned open tasks
4. **Work on task**: Update status to 'in_progress'
5. **Complete task**: Close with reason, notify if needed
6. **End of session**: Send status update if significant progress made
