# Context-Palace Agent Template

## Setup Instructions

### Step 1: Get Your Identity

Ask the user for:
- **Agent name** - e.g., `agent-cli`, `agent-backend`
- **Project name** - e.g., `penfold`
- **Project prefix** - e.g., `pf` for penfold, `cp` for context-palace

### Step 2: Copy Template to CLAUDE.md

Copy the template section below into your project's `CLAUDE.md` file.

### Step 3: Download the Full Guide

```bash
curl -o context-palace.md https://raw.githubusercontent.com/otherjamesbrown/context-palace/main/context-palace.md
```

Save it to the same folder as your `CLAUDE.md`.

### Step 4: Replace All Placeholders

In **both files** (`CLAUDE.md` and `context-palace.md`), find and replace:

| Find | Replace with | Example |
|------|--------------|---------|
| `[agent-YOURNAME]` | Your agent name | `agent-cli` |
| `[YOURPROJECT]` | Your project name | `penfold` |
| `[PREFIX]` | Your project prefix | `pf` |

### Step 5: Verify SSL Certs

Ensure SSL certificates are installed in `~/.postgresql/` (see secrets repo).

### Step 6: Test Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT 1;"
```

---

## Template (copy from here)

```markdown
## Context-Palace

You are **[agent-YOURNAME]** working on project **[YOURPROJECT]** (prefix: `[PREFIX]-`).

Context-Palace is your shared memory system. Use it to:
- Create and track tasks/bugs
- Send messages to other agents and humans
- Log your actions
- Store information that needs to persist

**Full guide:** Read `context-palace.md` in this folder.

### Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

### Start of Session

Always check for messages and tasks at the start of a session:

```sql
-- Check inbox
SELECT * FROM unread_for('[YOURPROJECT]', '[agent-YOURNAME]');

-- Check your tasks
SELECT * FROM tasks_for('[YOURPROJECT]', '[agent-YOURNAME]');

-- See ready tasks anyone can claim
SELECT * FROM ready_tasks('[YOURPROJECT]');
```

### Quick Commands

```sql
-- Mark message read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('[PREFIX]-xxx', '[agent-YOURNAME]') ON CONFLICT DO NOTHING;

-- Create task (simple)
SELECT create_shard('[YOURPROJECT]', 'Title', 'Details', 'task', '[agent-YOURNAME]');
-- Returns: [PREFIX]-a1b2c3

-- Create task with owner and priority
SELECT create_shard('[YOURPROJECT]', 'Title', 'Details', 'task', '[agent-YOURNAME]', 'target-agent', 2);

-- Send message
SELECT create_shard('[YOURPROJECT]', 'Subject', 'Body', 'message', '[agent-YOURNAME]');
INSERT INTO labels (shard_id, label) VALUES ('[PREFIX]-NEWID', 'to:recipient');

-- Reply to message
SELECT create_shard('[YOURPROJECT]', 'Re: Subject', 'Reply text', 'message', '[agent-YOURNAME]');
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('[PREFIX]-REPLY', '[PREFIX]-ORIGINAL', 'replies-to');
INSERT INTO labels (shard_id, label) VALUES ('[PREFIX]-REPLY', 'to:original-sender');

-- Claim task
UPDATE shards SET owner = '[agent-YOURNAME]', status = 'in_progress' WHERE id = '[PREFIX]-xxx' AND owner IS NULL;

-- Close task
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done: summary' WHERE id = '[PREFIX]-xxx';

-- Log an action
SELECT create_shard('[YOURPROJECT]', 'Did something', 'Details of action', 'log', '[agent-YOURNAME]');

-- Get conversation thread
SELECT * FROM get_thread('[PREFIX]-xxx');

-- Search
SELECT id, title, status FROM shards, to_tsquery('english', 'keyword') query
WHERE project = '[YOURPROJECT]' AND search_vector @@ query ORDER BY ts_rank(search_vector, query) DESC LIMIT 10;
```

### Priorities

| Priority | Meaning |
|----------|---------|
| 0 | Critical - drop everything |
| 1 | High - do today |
| 2 | Normal - this week |
| 3 | Low - when possible |

### Message Labels

- `to:agent-xxx` - Send to agent
- `to:human-xxx` - Send to human
- `kind:bug-report` - Bug report
- `kind:status-update` - FYI / progress
- `kind:question` - Needs response
```

---

## Example: Penfold CLI Agent

After replacement, CLAUDE.md would look like:

```markdown
## Context-Palace

You are **agent-cli** working on project **penfold** (prefix: `pf-`).

Context-Palace is your shared memory system. Use it to:
- Create and track tasks/bugs
- Send messages to other agents and humans
- Log your actions
- Store information that needs to persist

**Full guide:** Read `context-palace.md` in this folder.

### Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

### Start of Session

Always check for messages and tasks at the start of a session:

```sql
-- Check inbox
SELECT * FROM unread_for('penfold', 'agent-cli');

-- Check your tasks
SELECT * FROM tasks_for('penfold', 'agent-cli');

-- See ready tasks anyone can claim
SELECT * FROM ready_tasks('penfold');
```

### Quick Commands

```sql
-- Mark message read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('pf-xxx', 'agent-cli') ON CONFLICT DO NOTHING;

-- Create task (simple)
SELECT create_shard('penfold', 'Title', 'Details', 'task', 'agent-cli');
-- Returns: pf-a1b2c3

-- Create task with owner and priority
SELECT create_shard('penfold', 'Title', 'Details', 'task', 'agent-cli', 'target-agent', 2);

-- Send message
SELECT create_shard('penfold', 'Subject', 'Body', 'message', 'agent-cli');
INSERT INTO labels (shard_id, label) VALUES ('pf-NEWID', 'to:recipient');

-- Claim task
UPDATE shards SET owner = 'agent-cli', status = 'in_progress' WHERE id = 'pf-xxx' AND owner IS NULL;

-- Close task
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done: summary' WHERE id = 'pf-xxx';
```
```

---

## Known Projects

| Project | Prefix | Example ID |
|---------|--------|------------|
| penfold | `pf` | `pf-a1b2c3` |
| context-palace | `cp` | `cp-d4e5f6` |

Register new projects:
```sql
INSERT INTO projects (name, prefix) VALUES ('my-project', 'mp');
```
