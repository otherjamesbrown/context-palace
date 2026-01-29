# Context-Palace Setup for Claude Code

You are a Claude Code instance being asked to set up or update Context-Palace for this project. Follow these steps.

---

## Step 0: Check if Already Installed

First, check if Context-Palace is already set up:

```bash
ls -la context-palace.md *-rules.md 2>/dev/null
```

**If `context-palace.md` exists:** Skip to [Updating an Existing Installation](#updating-an-existing-installation)

**If not:** Continue with [Fresh Installation](#fresh-installation)

---

# Fresh Installation

## Step 1: Gather Information

Ask the user for the following information. Do not proceed until you have all three:

1. **Agent name** - Your identity (e.g., `agent-cli`, `agent-backend`, `agent-myproject`)
2. **Project name** - The project namespace (e.g., `penfold`, `myproject`)
3. **Project prefix** - 2-3 character ID prefix (e.g., `pf` for penfold, `mp` for myproject)

Example prompt:
> To set up Context-Palace, I need three pieces of information:
> 1. What should my agent name be? (e.g., `agent-cli`, `agent-backend`)
> 2. What is the project name? (e.g., `penfold`)
> 3. What is the project's ID prefix? (2-3 chars, e.g., `pf`)

---

## Step 2: Download context-palace.md

Download the full guide to this project directory:

```bash
curl -sL https://raw.githubusercontent.com/otherjamesbrown/context-palace/main/context-palace.md -o context-palace.md
```

Verify the download succeeded by checking the file exists and has content.

---

## Step 3: Replace Placeholders in context-palace.md

Using the Edit tool, replace ALL occurrences in `context-palace.md`:

| Find | Replace with |
|------|--------------|
| `[agent-YOURNAME]` | The agent name from Step 1 |
| `[YOURPROJECT]` | The project name from Step 1 |
| `[PREFIX]` | The project prefix from Step 1 |

Use `replace_all: true` for each replacement.

---

## Step 4: Create Project Rules File

Create a file named `{PREFIX}-rules.md` (e.g., `pf-rules.md`) with project-specific rules for how Context-Palace should be used.

**This file is project-specific and will NOT be overwritten during updates.**

Write this content (replace PREFIX, PROJECT_NAME, AGENT_NAME):

```markdown
# PREFIX-rules.md - Context-Palace Rules for PROJECT_NAME

## Project Identity

- **Project:** PROJECT_NAME
- **Prefix:** PREFIX-
- **Primary Agent:** AGENT_NAME

## Message Routing

### Who to contact for what

| Topic | Send to | Labels |
|-------|---------|--------|
| Bugs in this project | AGENT_NAME | `kind:bug-report` |
| Questions about this project | AGENT_NAME | `kind:question` |
| Context-Palace issues | agent-cxp | `kind:bug-report` |

## Task Conventions

### Priority Guidelines

| Priority | Use for |
|----------|---------|
| 0 (Critical) | Production down, security issues |
| 1 (High) | Blocking other work, user-facing bugs |
| 2 (Normal) | Standard features and fixes |
| 3 (Low) | Nice-to-haves, cleanup |

### Task Naming

- Bug fixes: `fix: description`
- Features: `feat: description`
- Refactoring: `refactor: description`
- Documentation: `docs: description`

## Label Conventions

### Component Labels
Add labels for the components this project uses:
- `backend` - Server-side code
- `frontend` - Client-side code
- `database` - Schema, migrations
- `infra` - Infrastructure, deployment

### Custom Labels
Define project-specific labels here:
- (Add your own)

## Session Start Checklist

When starting a session:
1. Check inbox: `SELECT * FROM unread_for('PROJECT_NAME', 'AGENT_NAME');`
2. Check tasks: `SELECT * FROM tasks_for('PROJECT_NAME', 'AGENT_NAME');`
3. Process messages before starting new work

## Project-Specific Notes

Add any project-specific Context-Palace usage notes here:
- (Add your own)
```

---

## Step 5: Add Context-Palace Section to CLAUDE.md

If CLAUDE.md doesn't exist, create it. If it exists, append to it.

Add this section (with placeholders already replaced):

```markdown
## Context-Palace

You are **AGENT_NAME** working on project **PROJECT_NAME** (prefix: `PREFIX-`).

Context-Palace is your shared memory system. Use it to:
- Create and track tasks/bugs
- Send messages to other agents and humans
- Log your actions
- Store information that needs to persist

**Guides:**
- `context-palace.md` - Full reference guide
- `PREFIX-rules.md` - Project-specific rules and conventions

### Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

### Start of Session

Always check for messages and tasks at the start of a session:

```sql
-- Check inbox
SELECT * FROM unread_for('PROJECT_NAME', 'AGENT_NAME');

-- Check your tasks
SELECT * FROM tasks_for('PROJECT_NAME', 'AGENT_NAME');

-- See ready tasks anyone can claim
SELECT * FROM ready_tasks('PROJECT_NAME');
```

### Quick Commands

```sql
-- Mark message read
INSERT INTO read_receipts (shard_id, agent_id) VALUES ('PREFIX-xxx', 'AGENT_NAME') ON CONFLICT DO NOTHING;

-- Create task
SELECT create_shard('PROJECT_NAME', 'Title', 'Details', 'task', 'AGENT_NAME');

-- Send message
SELECT send_message('PROJECT_NAME', 'AGENT_NAME', ARRAY['recipient'], 'Subject', 'Body');

-- Claim task
UPDATE shards SET owner = 'AGENT_NAME', status = 'in_progress' WHERE id = 'PREFIX-xxx' AND owner IS NULL;

-- Close task
UPDATE shards SET status = 'closed', closed_at = NOW(), closed_reason = 'Done: summary' WHERE id = 'PREFIX-xxx';
```

### Priorities

| Priority | Meaning |
|----------|---------|
| 0 | Critical - drop everything |
| 1 | High - do today |
| 2 | Normal - this week |
| 3 | Low - when possible |
```

Replace `AGENT_NAME`, `PROJECT_NAME`, and `PREFIX` with the actual values.

---

## Step 6: Verify SSL Certificates

Check if PostgreSQL SSL certificates exist:

```bash
ls -la ~/.postgresql/
```

Expected files:
- `postgresql.crt` - Client certificate
- `postgresql.key` - Client private key
- `root.crt` - CA certificate

If these files don't exist, warn the user:
> SSL certificates are not installed. You need to get them from the secrets repository and install them in `~/.postgresql/`. See `context-palace.md` for details, or ask the administrator for access.

---

## Step 7: Test Connection

Test the database connection:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT 1 AS connection_test;"
```

If successful, you'll see:
```
 connection_test
-----------------
               1
```

If it fails, report the error to the user. Common issues:
- SSL certs not installed (Step 6)
- Network/firewall blocking port 5432
- Database server down

---

## Step 8: Register Project (if new)

Check if the project exists:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT * FROM projects WHERE name = 'PROJECT_NAME';"
```

If no rows returned, register it:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "INSERT INTO projects (name, prefix) VALUES ('PROJECT_NAME', 'PREFIX');"
```

---

## Step 9: Confirm Setup

Report to the user:

> Context-Palace setup complete!
>
> **Files created:**
> - `context-palace.md` - Full reference guide
> - `PREFIX-rules.md` - Project-specific rules (customize this!)
>
> **Configuration:**
> - Agent: **AGENT_NAME**
> - Project: **PROJECT_NAME** (prefix: `PREFIX-`)
> - Connection: Tested successfully
>
> **Next steps:**
> 1. Review and customize `PREFIX-rules.md` for your project
> 2. Check inbox: `SELECT * FROM unread_for('PROJECT_NAME', 'AGENT_NAME');`
> 3. Check tasks: `SELECT * FROM tasks_for('PROJECT_NAME', 'AGENT_NAME');`

---

# Updating an Existing Installation

If Context-Palace is already set up, follow these steps to update.

## Step U1: Identify Current Configuration

Read the existing rules file to get the current configuration:

```bash
ls *-rules.md
```

Read the rules file to extract:
- Project name
- Project prefix
- Agent name

If CLAUDE.md exists, you can also check the Context-Palace section there.

---

## Step U2: Check for Updates

Compare local `context-palace.md` with the latest from GitHub:

```bash
# Download latest to temp file
curl -sL https://raw.githubusercontent.com/otherjamesbrown/context-palace/main/context-palace.md -o context-palace.md.new

# Check if different (ignoring placeholder replacements)
# The local file has placeholders replaced, so we need to check structure
```

Use the Read tool to check both files. Look for:
- New sections added
- New helper functions documented
- Changed SQL examples
- Updated instructions

---

## Step U3: Update context-palace.md

Download the fresh version:

```bash
curl -sL https://raw.githubusercontent.com/otherjamesbrown/context-palace/main/context-palace.md -o context-palace.md
```

Replace the placeholders with the values from Step U1:

| Find | Replace with |
|------|--------------|
| `[agent-YOURNAME]` | The agent name from the rules file |
| `[YOURPROJECT]` | The project name from the rules file |
| `[PREFIX]` | The project prefix from the rules file |

Use `replace_all: true` for each replacement.

---

## Step U4: DO NOT Touch Rules File

**IMPORTANT:** The `{PREFIX}-rules.md` file contains project-specific customizations. Do NOT overwrite or modify it during updates.

If the user wants to reset the rules file to defaults, they must explicitly request it.

---

## Step U5: Check CLAUDE.md

Review the Context-Palace section in CLAUDE.md. If it's missing the reference to the rules file, add it:

```markdown
**Guides:**
- `context-palace.md` - Full reference guide
- `PREFIX-rules.md` - Project-specific rules and conventions
```

---

## Step U6: Report Update

Report to the user:

> Context-Palace updated!
>
> **Updated:**
> - `context-palace.md` - Downloaded latest version
>
> **Preserved (not modified):**
> - `PREFIX-rules.md` - Your project-specific rules
>
> **Configuration:**
> - Agent: **AGENT_NAME**
> - Project: **PROJECT_NAME** (prefix: `PREFIX-`)

---

# Troubleshooting

### "SSL certificate not found"
SSL certs must be in `~/.postgresql/`. Get them from the secrets repo or ask the administrator.

### "Connection refused"
- Check if dev02.brown.chat is reachable: `ping dev02.brown.chat`
- Check if port 5432 is open: `nc -zv dev02.brown.chat 5432`

### "Permission denied for table"
The database user needs grants. Ask administrator to run:
```sql
GRANT ALL ON ALL TABLES IN SCHEMA public TO "penfold";
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO "penfold";
```

### "Project prefix already exists"
Each project needs a unique prefix. Choose a different one.

### "Rules file was accidentally deleted"
Re-run the fresh installation steps, but only create the rules file (Step 4). Don't re-download context-palace.md unless needed.
