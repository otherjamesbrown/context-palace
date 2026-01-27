# Context-Palace Snippet

Copy this section into any repo's CLAUDE.md to enable Context-Palace.

---

## Context-Palace (Shared Agent Memory)

Shared task and message system. Replace `YOURPROJECT` and `agent-YOURNAME` with your values.

**Connect:**
```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

**Check inbox:**
```sql
SELECT * FROM unread_for('YOURPROJECT', 'agent-YOURNAME');
```

**Mark read:**
```sql
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('cp-xxx', 'agent-YOURNAME') ON CONFLICT DO NOTHING;
```

**Get your tasks:**
```sql
SELECT * FROM tasks_for('YOURPROJECT', 'agent-YOURNAME');
```

**Send message:**
```sql
INSERT INTO shards (project, title, content, type, creator)
VALUES ('YOURPROJECT', 'Subject', 'Body', 'message', 'agent-YOURNAME') RETURNING id;
-- Then add recipient:
INSERT INTO labels (shard_id, label) VALUES ('cp-NEWID', 'to:recipient-agent');
```

**Create task:**
```sql
INSERT INTO shards (project, title, content, type, status, creator, owner, priority)
VALUES ('YOURPROJECT', 'Title', 'Details', 'task', 'open', 'agent-YOURNAME', 'target-agent', 2)
RETURNING id;
```

**Close task:**
```sql
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done' WHERE id = 'cp-xxx';
```

**Full docs:** `~/github/otherjamesbrown/context-palace/CLAUDE.md`
