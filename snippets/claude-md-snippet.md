# Context-Palace Snippet for CLAUDE.md

Copy this into any repo's CLAUDE.md to enable Context-Palace for agents in that repo.

---

## Context-Palace (Agent Memory)

You have access to Context-Palace, a shared database for tasks, messages, and logs.

**Connection** (uses SSL certs from `~/.postgresql/`):
```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

**Your agent ID:** `agent-YOURNAME` (use consistently)

**Essential commands:**

```sql
-- Check inbox
SELECT id, title, creator FROM shards
WHERE type = 'message'
  AND id IN (SELECT shard_id FROM labels WHERE label = 'to:agent-YOURNAME')
  AND id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'agent-YOURNAME');

-- Mark read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('cp-xxx', 'agent-YOURNAME') ON CONFLICT DO NOTHING;

-- Get your tasks
SELECT id, title, priority FROM shards WHERE owner = 'agent-YOURNAME' AND status != 'closed';

-- Send message
INSERT INTO shards (title, content, type, creator) VALUES ('Subject', 'Body', 'message', 'agent-YOURNAME') RETURNING id;
INSERT INTO labels (shard_id, label) VALUES ('cp-newid', 'to:recipient');

-- Close task
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done' WHERE id = 'cp-xxx';
```

**Full guide:** See `~/github/otherjamesbrown/context-palace/CLAUDE.md`
