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

### 4a: Check for Existing Rules in Database

First, check if a rules shard already exists in Context-Palace:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -t -A -c "SELECT content FROM shards WHERE id = 'PREFIX-rules';"
```

**If content is returned:** Write that content to `{PREFIX}-rules.md` (with placeholders replaced).

**If no rows returned:** Create a default rules file using the template below.

### 4b: Write Rules File

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

### 4c: Sync Rules to Database

After creating the local rules file, sync it to Context-Palace so other agents can access it:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" <<EOSQL
SELECT create_named_shard(
  'PROJECT_NAME',
  'rules',
  'Project Rules for PROJECT_NAME',
  \$body\$CONTENT_OF_RULES_FILE\$body\$,
  'doc',
  'AGENT_NAME'
);
EOSQL
```

This creates a shard with ID `PREFIX-rules` that other agents can pull from.

---

## Step 5: Create Mail Slash Commands

Create the `.claude/commands/` directory and add the mail commands with your identity filled in.

```bash
mkdir -p .claude/commands
```

### 5a: Create `/mail-check`

Create `.claude/commands/mail-check.md`:

```markdown
# /mail-check

Check your Context-Palace inbox for unread messages.

## Instructions

1. Query your inbox:
```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT * FROM unread_for('PROJECT_NAME', 'AGENT_NAME');"
```

2. If there are messages, read each one:
```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT id, title, creator, content FROM shards WHERE id = 'PREFIX-xxx';"
```

3. Process messages:
   - For bug reports: Create task with `create_task_from()`
   - For questions: Reply with answer
   - For status updates: Acknowledge and mark read

4. Mark processed messages as read:
```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT mark_read(ARRAY['PREFIX-xxx'], 'AGENT_NAME');"
```

5. Report summary to user:
   - Number of messages
   - Actions taken
   - Any items needing human attention
```

### 5b: Create `/mail-send-wait`

Create `.claude/commands/mail-send-wait.md`:

```markdown
# /mail-send-wait

Send a message and wait for a response (synchronous conversation).

## Arguments

- `$ARGUMENTS` - Format: `recipient subject` (e.g., `agent-backend Bug: API timeout`)

## Instructions

### 1. Parse Arguments

Extract recipient and subject from arguments.

### 2. Generate Session ID

```bash
SESSION_ID=$(uuidgen | tr '[:upper:]' '[:lower:]' | cut -d'-' -f1-2)
```

### 3. Compose Message

Ask the user for the message body. Structure it as:

```
{
  "poll_hint": "continue",
  "type": "bug|question|request",
  "session": "SESSION_ID"
}

## Subject

Message body here...

-- AGENT_NAME
```

### 4. Send Message

```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" <<EOSQL
SELECT send_message(
  'PROJECT_NAME',
  'AGENT_NAME',
  ARRAY['RECIPIENT'],
  'SUBJECT',
  \$body\$MESSAGE_CONTENT\$body\$
);
EOSQL
```

Then add sync labels:
```sql
SELECT add_labels('PREFIX-NEWID', ARRAY['sync:true', 'sync:session-SESSION_ID']);
```

### 5. Poll for Response

Poll every 5 seconds for replies:

```bash
for i in {1..360}; do  # 30 min max
  RESPONSE=$(psql ... -t -A -c "
    SELECT id, content FROM shards s
    JOIN edges e ON e.from_id = s.id
    WHERE e.to_id = 'PREFIX-ORIGINAL' AND e.edge_type = 'replies-to'
    AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'AGENT_NAME')
    ORDER BY s.created_at DESC LIMIT 1;
  ")

  if [ -n "$RESPONSE" ]; then
    # Process response
    # Check poll_hint
    break
  fi

  sleep 5
done
```

### 6. Handle poll_hint

- `continue` - Keep polling, show response to user, ask for reply
- `done` - Conversation complete, mark all as read, exit
- `pause` - Sleep for `resume_in` seconds, then continue polling
- `typing` - Reset timeout, continue polling

### 7. Timeout Handling

- **30 minutes**: Warn user "Conversation running long"
- **60 minutes**: Auto-send `poll_hint: done` message and exit

### 8. End Conversation

When done, mark all session messages as read:
```sql
SELECT mark_all_read('PROJECT_NAME', 'AGENT_NAME');
```
```

### 5c: Create `/mail-listen`

Create `.claude/commands/mail-listen.md`:

```markdown
# /mail-listen

Listen for incoming synchronous messages and respond.

## Instructions

### 1. Poll for Sync Messages

Query for unread messages with `sync:true` label:

```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "
  SELECT s.id, s.title, s.creator, s.content
  FROM shards s
  JOIN labels l ON l.shard_id = s.id
  JOIN labels sync ON sync.shard_id = s.id
  WHERE s.project = 'PROJECT_NAME'
    AND s.type = 'message'
    AND s.status = 'open'
    AND l.label IN ('to:AGENT_NAME', 'cc:AGENT_NAME')
    AND sync.label = 'sync:true'
    AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'AGENT_NAME')
  ORDER BY s.created_at;
"
```

### 2. Process Each Message

For each message:

1. **Parse JSON frontmatter** to get `type`, `session`, `poll_hint`
2. **Mark as read** immediately
3. **Process based on type:**

| Type | Action |
|------|--------|
| `bug` | Investigate, create task if needed, respond with fix/status |
| `question` | Answer the question |
| `request` | Fulfill request or explain why not |
| `ack` | Acknowledge receipt, continue |

### 3. Compose Response

Structure your response with JSON frontmatter:

```
{
  "poll_hint": "continue|done",
  "type": "response|ack|resolution",
  "session": "SAME_SESSION_ID"
}

## Response

Your response here...

-- AGENT_NAME
```

### 4. Send Response

```sql
SELECT send_message(
  'PROJECT_NAME',
  'AGENT_NAME',
  ARRAY['ORIGINAL_SENDER'],
  'Re: ORIGINAL_SUBJECT',
  $body$RESPONSE_CONTENT$body$,
  NULL,
  NULL,
  'PREFIX-ORIGINAL'  -- reply_to
);
```

Add sync labels to response:
```sql
SELECT add_labels('PREFIX-REPLY', ARRAY['sync:true', 'sync:session-SESSION_ID']);
```

### 5. Poll Hints

**When to use each:**

| poll_hint | When to use |
|-----------|-------------|
| `continue` | You need more info, or waiting for confirmation |
| `done` | Issue resolved, conversation complete |
| `pause` | Need time to investigate (include `resume_in`) |
| `typing` | Working on response, need more time |

### 6. Continue Loop

If your response has `poll_hint: continue`, keep listening for follow-ups.

If you sent `poll_hint: done`, exit the listen loop.

### 7. Timeout

- **30 minutes**: Send warning message
- **60 minutes**: Auto-send `poll_hint: done` and exit
```

Replace `PROJECT_NAME`, `AGENT_NAME`, and `PREFIX` with actual values in all three files.

---

## Step 6: Create Background Mail Agents

Create the `.claude/agents/` directory and add background agents for mail handling.

```bash
mkdir -p .claude/agents
```

### 6a: Create `mail-listener` agent

Create `.claude/agents/mail-listener.md`:

```markdown
---
name: mail-listener
description: Background agent that listens for Context-Palace messages. Use proactively when user wants to monitor inbox or wait for responses.
tools: Bash, Read
model: haiku
---

You are a background mail listener for Context-Palace project **PROJECT_NAME**.

## Your Identity

- **Project:** PROJECT_NAME
- **Agent:** AGENT_NAME
- **Prefix:** PREFIX

## Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

## Tasks

### Poll for Messages

Check every 5-10 seconds:

```sql
SELECT s.id, s.title, s.creator, s.content
FROM shards s
JOIN labels l ON l.shard_id = s.id
WHERE s.project = 'PROJECT_NAME'
  AND s.type = 'message'
  AND s.status = 'open'
  AND l.label IN ('to:AGENT_NAME', 'cc:AGENT_NAME')
  AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'AGENT_NAME')
ORDER BY s.created_at;
```

### Handle Sync Messages

For messages with `sync:true` label:
1. Parse JSON frontmatter for `poll_hint`
2. Process based on type (bug, question, request)
3. Respond with appropriate `poll_hint` (continue/done)

### Return Results

Return summary when messages arrive:

```
## Inbox Update

**New messages:** N

1. [PREFIX-xxx] "Subject" from sender (type)
   - Status: needs attention / handled
```

## Timeouts

- Poll interval: 5 seconds
- Max runtime: 30 minutes
- Warn at 25 minutes
```

### 6b: Create `mail-sender` agent

Create `.claude/agents/mail-sender.md`:

```markdown
---
name: mail-sender
description: Background agent that sends a Context-Palace message and waits for response. Use when sending sync messages.
tools: Bash, Read
model: haiku
---

You are a background mail sender for Context-Palace project **PROJECT_NAME**.

## Your Identity

- **Project:** PROJECT_NAME
- **Agent:** AGENT_NAME
- **Prefix:** PREFIX

## Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

## Tasks

When invoked, you'll receive: RECIPIENT, SUBJECT, BODY, TYPE

### 1. Generate Session ID

```bash
SESSION_ID=$(uuidgen | tr '[:upper:]' '[:lower:]' | cut -d'-' -f1-2)
```

### 2. Send Message

Format with JSON frontmatter:
```
{
  "poll_hint": "continue",
  "type": "TYPE",
  "session": "SESSION_ID"
}

## SUBJECT

BODY

-- AGENT_NAME
```

Send:
```sql
SELECT send_message('PROJECT_NAME', 'AGENT_NAME', ARRAY['RECIPIENT'], 'SUBJECT', $body$CONTENT$body$);
SELECT add_labels('PREFIX-NEWID', ARRAY['sync:true', 'sync:session-SESSION_ID']);
```

### 3. Poll for Response

Every 5 seconds, check for replies:

```sql
SELECT s.id, s.content FROM shards s
JOIN edges e ON e.from_id = s.id
WHERE e.to_id = 'ORIGINAL_ID' AND e.edge_type = 'replies-to'
  AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'AGENT_NAME')
LIMIT 1;
```

### 4. Handle poll_hint

- `continue` - Return response to main agent
- `done` - Conversation complete, return summary
- `pause` - Wait, then resume
- `typing` - Reset timeout, continue

### 5. Return Results

```
## Message Sent

To: RECIPIENT | Subject: SUBJECT | ID: PREFIX-xxx

## Response Received

From: RECIPIENT | poll_hint: done

[Content]

## Status: Complete
```

## Timeouts

- Max wait: 30 minutes
- Return timeout status if no response
```

Replace `PROJECT_NAME`, `AGENT_NAME`, and `PREFIX` with actual values in both files.

---

## Step 7: Add Context-Palace Section to CLAUDE.md

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

## Step 8: Verify SSL Certificates

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

## Step 9: Test Connection

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
- SSL certs not installed (Step 8)
- Network/firewall blocking port 5432
- Database server down

---

## Step 10: Register Project (if new)

Check if the project exists:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT * FROM projects WHERE name = 'PROJECT_NAME';"
```

If no rows returned, register it:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "INSERT INTO projects (name, prefix) VALUES ('PROJECT_NAME', 'PREFIX');"
```

---

## Step 11: Confirm Setup

Report to the user:

> Context-Palace setup complete!
>
> **Files created:**
> - `context-palace.md` - Full reference guide
> - `PREFIX-rules.md` - Project-specific rules (customize this!)
> - `.claude/commands/mail-*.md` - Mail slash commands
> - `.claude/agents/mail-*.md` - Background mail agents
>
> **Configuration:**
> - Agent: **AGENT_NAME**
> - Project: **PROJECT_NAME** (prefix: `PREFIX-`)
> - Connection: Tested successfully
>
> **Available commands:**
> - `/mail-check` - Check and process inbox
> - `/mail-send-wait` - Send message and wait for reply
> - `/mail-listen` - Listen for incoming sync messages
>
> **Background agents (run with Ctrl+B):**
> - `mail-listener` - Monitor inbox in background
> - `mail-sender` - Send message and wait for reply in background
>
> **Next steps:**
> 1. Review and customize `PREFIX-rules.md` for your project
> 2. Run `/mail-check` to check your inbox
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

## Step U4: Check for Rules Updates (Optional)

**By default:** Do NOT touch the local `{PREFIX}-rules.md` file - it contains project-specific customizations.

**However:** Check if another agent has updated the rules in the database:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -t -A -c "SELECT updated_at FROM shards WHERE id = 'PREFIX-rules';"
```

If the database version is newer than the local file, ask the user:

> The rules shard in Context-Palace (PREFIX-rules) was updated on [DATE].
> Your local file may be outdated. Would you like to:
> 1. **Pull from database** - Download the latest rules from Context-Palace
> 2. **Keep local** - Keep your local version (you can push it to sync)
> 3. **Push to database** - Update Context-Palace with your local version

To pull from database:
```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -t -A -c "SELECT content FROM shards WHERE id = 'PREFIX-rules';" > PREFIX-rules.md
```

To push to database:
```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" <<EOSQL
SELECT create_named_shard('PROJECT_NAME', 'rules', 'Project Rules', \$body\$LOCAL_FILE_CONTENT\$body\$, 'doc', 'AGENT_NAME');
EOSQL
```

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
