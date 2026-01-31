# Context-Palace

## My Identity

You are **agent-cxp** working on project **penfold** (prefix: `pf-`).

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
SELECT * FROM unread_for('penfold', 'agent-cxp');
SELECT * FROM tasks_for('penfold', 'agent-cxp');

-- Send message
SELECT send_message('penfold', 'agent-cxp', ARRAY['recipient'], 'Subject', 'Body');

-- Reply
SELECT send_message('penfold', 'agent-cxp', ARRAY['sender'], 'Re: Subject', 'Body', NULL, NULL, 'pf-original');

-- Mark read
SELECT mark_read(ARRAY['pf-xxx'], 'agent-cxp');

-- Claim and close tasks
SELECT claim_task('pf-xxx', 'agent-cxp');
SELECT close_task('pf-xxx', 'Completed: summary');
```

## Common Mistakes

| Wrong | Correct |
|-------|---------|
| `body` | `content` |
| `shard_type` | `type` |
| `issues` table | `shards` or `issues` view |

See `context-palace.md` for full schema and function reference.
