# Context-Palace Agent Template

## For Claude Code Instances

**If you are a Claude Code instance** setting up Context-Palace for a project, read and follow `setup.md` instead. It has step-by-step instructions designed for you to execute.

```bash
curl -sL https://raw.githubusercontent.com/otherjamesbrown/context-palace/main/setup.md | head -100
```

---

## Manual Setup Instructions (for humans)

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
# Context-Palace

## My Identity

You are **[agent-YOURNAME]** working on project **[YOURPROJECT]** (prefix: `[PREFIX]-`).

## Context-Palace (Support System)

Context-Palace is your **support system** for:
- Raising issues and reporting bugs
- Creating and tracking work items
- Sending messages to other agents
- Logging actions and storing information

It assists your work - it is not your primary task.

**Reference docs:**
- `context-palace.md` - Full usage guide (Quick Reference at top, Common Mistakes section)
- `[PREFIX]-rules` - Project rules: `SELECT content FROM shards WHERE id = '[PREFIX]-rules';`

**Connection:**
```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

## Quick Commands

```sql
-- Check inbox and tasks
SELECT * FROM unread_for('[YOURPROJECT]', '[agent-YOURNAME]');
SELECT * FROM inbox_summary('[YOURPROJECT]', '[agent-YOURNAME]');
SELECT * FROM tasks_for('[YOURPROJECT]', '[agent-YOURNAME]');

-- Send message
SELECT send_message('[YOURPROJECT]', '[agent-YOURNAME]', ARRAY['recipient'], 'Subject', 'Body');

-- Reply to message
SELECT send_message('[YOURPROJECT]', '[agent-YOURNAME]', ARRAY['sender'], 'Re: Subject', 'Body', NULL, NULL, '[PREFIX]-original');

-- Mark read
SELECT mark_read(ARRAY['[PREFIX]-xxx'], '[agent-YOURNAME]');

-- Create task
SELECT create_shard('[YOURPROJECT]', 'Title', 'Description', 'task', '[agent-YOURNAME]');

-- Claim and close tasks
SELECT claim_task('[PREFIX]-xxx', '[agent-YOURNAME]');
SELECT close_task('[PREFIX]-xxx', 'Completed: summary');

-- Add artifact to task
SELECT add_artifact('[PREFIX]-xxx', 'commit', 'abc123', 'Fixed the bug');
```

## Common Mistakes

| Wrong | Correct |
|-------|---------|
| `body` | `content` |
| `shard_type` | `type` |
| `issues` table | `shards` or `issues` view |

See `context-palace.md` for full schema and function reference.
```

---

## Example: Penfold CLI Agent

After replacement, CLAUDE.md would look like:

```markdown
# Context-Palace

## My Identity

You are **agent-cli** working on project **penfold** (prefix: `pf-`).

## Context-Palace (Support System)

Context-Palace is your **support system** for:
- Raising issues and reporting bugs
- Creating and tracking work items
- Sending messages to other agents
- Logging actions and storing information

It assists your work - it is not your primary task.

**Reference docs:**
- `context-palace.md` - Full usage guide (Quick Reference at top, Common Mistakes section)
- `pf-rules` - Project rules: `SELECT content FROM shards WHERE id = 'pf-rules';`

**Connection:**
```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

## Quick Commands

```sql
-- Check inbox and tasks
SELECT * FROM unread_for('penfold', 'agent-cli');
SELECT * FROM inbox_summary('penfold', 'agent-cli');
SELECT * FROM tasks_for('penfold', 'agent-cli');

-- Send message
SELECT send_message('penfold', 'agent-cli', ARRAY['recipient'], 'Subject', 'Body');

-- Reply to message
SELECT send_message('penfold', 'agent-cli', ARRAY['sender'], 'Re: Subject', 'Body', NULL, NULL, 'pf-original');

-- Mark read
SELECT mark_read(ARRAY['pf-xxx'], 'agent-cli');

-- Create task
SELECT create_shard('penfold', 'Title', 'Description', 'task', 'agent-cli');

-- Claim and close tasks
SELECT claim_task('pf-xxx', 'agent-cli');
SELECT close_task('pf-xxx', 'Completed: summary');

-- Add artifact to task
SELECT add_artifact('pf-xxx', 'commit', 'abc123', 'Fixed the bug');
```

## Common Mistakes

| Wrong | Correct |
|-------|---------|
| `body` | `content` |
| `shard_type` | `type` |
| `issues` table | `shards` or `issues` view |

See `context-palace.md` for full schema and function reference.
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
