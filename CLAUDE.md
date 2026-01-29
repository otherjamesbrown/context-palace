# Context-Palace

## My Identity

You are **agent-cxp** working on project **penfold** (prefix: `pf-`).

Use Context-Palace to create tasks, send messages, log actions, and store information.

**Guides:**
- `context-palace.md` - Full reference guide
- `pf-rules.md` - Project-specific rules and conventions

### Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

### Start of Session

```sql
-- Check inbox
SELECT * FROM unread_for('penfold', 'agent-cxp');

-- Check tasks
SELECT * FROM tasks_for('penfold', 'agent-cxp');
```

### Quick Commands

```sql
-- Mark read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('pf-xxx', 'agent-cxp') ON CONFLICT DO NOTHING;

-- Create task
SELECT create_shard('penfold', 'Title', 'Details', 'task', 'agent-cxp');
-- With owner and priority
SELECT create_shard('penfold', 'Title', 'Details', 'task', 'agent-cxp', 'target-agent', 2);

-- Send message
SELECT create_shard('penfold', 'Subject', 'Body', 'message', 'agent-cxp');
INSERT INTO labels (shard_id, label) VALUES ('pf-NEWID', 'to:recipient');

-- Claim task
UPDATE shards SET owner = 'agent-cxp', status = 'in_progress' WHERE id = 'pf-xxx' AND owner IS NULL;

-- Close task
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done: summary' WHERE id = 'pf-xxx';

-- Get thread
SELECT * FROM get_thread('pf-xxx');
```
