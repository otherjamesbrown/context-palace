# Context-Palace

## My Identity

You are **agent-cxp** working on project **penfold**.

Use Context-Palace to create tasks, send messages, log actions, and store information.

**Full guide:** `~/github/otherjamesbrown/context-palace/context-palace.md`

### Quick Commands

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

```sql
-- Check inbox
SELECT * FROM unread_for('penfold', 'agent-cxp');

-- Check tasks
SELECT * FROM tasks_for('penfold', 'agent-cxp');

-- Mark read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('cpx-xxx', 'agent-cxp') ON CONFLICT DO NOTHING;

-- Send message
INSERT INTO shards (project, title, content, type, creator)
VALUES ('penfold', 'Subject', 'Body', 'message', 'agent-cxp') RETURNING id;
INSERT INTO labels (shard_id, label) VALUES ('cpx-NEWID', 'to:recipient');

-- Create task
INSERT INTO shards (project, title, content, type, status, creator, owner, priority)
VALUES ('penfold', 'Title', 'Details', 'task', 'open', 'agent-cxp', NULL, 2) RETURNING id;

-- Close task
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done: summary' WHERE id = 'cpx-xxx';
```

---

# Template for Other Agents

Copy the section below into your project's CLAUDE.md file. Update the agent name and project.

---

## Template (copy from here)

```markdown
## Context-Palace

You are **agent-YOURNAME** working on project **YOURPROJECT**.

Context-Palace is your shared memory system. Use it to:
- Create and track tasks/bugs
- Send messages to other agents and humans
- Log your actions
- Store information that needs to persist

**Read the full guide:** `~/github/otherjamesbrown/context-palace/context-palace.md`

### Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

### Start of Session

Check for messages and tasks:

```sql
-- Check inbox
SELECT * FROM unread_for('YOURPROJECT', 'agent-YOURNAME');

-- Check your tasks
SELECT * FROM tasks_for('YOURPROJECT', 'agent-YOURNAME');
```

### Quick Commands

```sql
-- Mark message read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('cp-xxx', 'agent-YOURNAME') ON CONFLICT DO NOTHING;

-- Send message
INSERT INTO shards (project, title, content, type, creator)
VALUES ('YOURPROJECT', 'Subject', 'Body', 'message', 'agent-YOURNAME') RETURNING id;
INSERT INTO labels (shard_id, label) VALUES ('cp-NEWID', 'to:recipient');

-- Create task
INSERT INTO shards (project, title, content, type, status, creator, owner, priority)
VALUES ('YOURPROJECT', 'Title', 'Details', 'task', 'open', 'agent-YOURNAME', NULL, 2) RETURNING id;

-- Claim task
UPDATE shards SET owner = 'agent-YOURNAME', status = 'in_progress' WHERE id = 'cp-xxx' AND owner IS NULL;

-- Close task
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done: summary' WHERE id = 'cp-xxx';
```
```

---

## Example: Penfold CLI Agent

```markdown
## Context-Palace

You are **agent-cli** working on project **penfold**.

Context-Palace is your shared memory system. Use it to:
- Create and track tasks/bugs
- Send messages to other agents and humans
- Log your actions
- Store information that needs to persist

**Read the full guide:** `~/github/otherjamesbrown/context-palace/context-palace.md`

### Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

### Start of Session

Check for messages and tasks:

```sql
-- Check inbox
SELECT * FROM unread_for('penfold', 'agent-cli');

-- Check your tasks
SELECT * FROM tasks_for('penfold', 'agent-cli');
```

### Quick Commands

```sql
-- Mark message read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('cp-xxx', 'agent-cli') ON CONFLICT DO NOTHING;

-- Send message
INSERT INTO shards (project, title, content, type, creator)
VALUES ('penfold', 'Subject', 'Body', 'message', 'agent-cli') RETURNING id;
INSERT INTO labels (shard_id, label) VALUES ('cp-NEWID', 'to:recipient');

-- Create task
INSERT INTO shards (project, title, content, type, status, creator, owner, priority)
VALUES ('penfold', 'Title', 'Details', 'task', 'open', 'agent-cli', NULL, 2) RETURNING id;

-- Claim task
UPDATE shards SET owner = 'agent-cli', status = 'in_progress' WHERE id = 'cp-xxx' AND owner IS NULL;

-- Close task
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done: summary' WHERE id = 'cp-xxx';
```
```

---

## Checklist

When adding Context-Palace to a new agent:

1. [ ] Copy the template section into the project's CLAUDE.md
2. [ ] Replace `YOURNAME` with the agent name (e.g., `cli`, `backend`, `frontend`)
3. [ ] Replace `YOURPROJECT` with the project name (e.g., `penfold`)
4. [ ] Ensure SSL certs are installed on the machine (`~/.postgresql/`)
5. [ ] Test connection: `psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT 1;"`
