# pf-rules.md - Context-Palace Rules for penfold

## Project Identity

- **Project:** penfold
- **Prefix:** pf-
- **Agents:**
  - **agent-mycroft** - Primary development agent
  - **agent-penfold** - Development agent
  - **agent-cxp** - Context-Palace maintainer

## Message Routing

### Who to contact for what

| Topic | Send to | Labels |
|-------|---------|--------|
| Penfold feature requests | mycroft | `kind:feature` |
| Penfold bugs | mycroft | `kind:bug-report` |
| Context-Palace issues | cxp | `kind:bug-report` |
| Infrastructure issues | cxp | `kind:bug-report`, `infra` |

Note: Agent names auto-expand (`mycroft` → `agent-mycroft`)

## Message → Task Workflow

Messages are for communication. Tasks are for trackable work.

### When you receive a message that requires work:

1. **Read and acknowledge** the message
2. **Discuss with user** if clarification needed
3. **Create task(s)** linked to the message:
   ```sql
   SELECT create_shard('penfold', 'feat: description', 'Details...',
     'task', 'mycroft', NULL, 2);
   -- Or with parent_id for linking:
   INSERT INTO shards (project, type, title, content, parent_id, creator, priority)
   VALUES ('penfold', 'task', 'feat: description', 'Details...',
           'pf-original-msg-id', 'agent-mycroft', 2)
   RETURNING id;
   ```
4. **Claim the task** with claim_task()
5. **Work the task** and close when done
6. **Reply to original message** to confirm completion

### Key Points

- `claim_task()` only works on type='task', not type='message'
- Task's `parent_id` links back to originating message
- Multiple tasks can reference same parent message

## File Claims (Multi-Agent Coordination)

When multiple Claude sessions work in parallel, use file_claims to prevent conflicts.

### Before starting work on a shard:

```sql
SELECT claim_files('pf-xxx', 'session-id', 'mycroft',
  ARRAY['file1.go', 'file2.go']);
```

### Check for conflicts before planning work:

```sql
SELECT * FROM check_conflicts(ARRAY['file1.go'], 'my-shard-id');
```

### When shard is closed:

Claims are automatically released by close_task().

### Rules

- Claim files BEFORE reading or writing
- If claim fails, STOP and report conflict
- Claims expire after 1 hour (extend if needed)
- Sub-agents inherit parent shard's claims

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

- `cli` - CLI commands (cmd/penf/)
- `gateway` - Gateway service
- `worker` - Worker service
- `proto` - Protobuf definitions
- `database` - Schema, migrations, queries
- `infra` - Infrastructure, deployment
- `docs` - Documentation

### Custom Labels

- `context-palace` - Related to Context-Palace itself
- `agent-comms` - Agent communication features

## Session Start Checklist

When starting a session:
1. Check inbox: `SELECT * FROM unread_for('penfold', 'YOURNAME');`
2. Check tasks: `SELECT * FROM tasks_for('penfold', 'YOURNAME');`
3. Process messages before starting new work
4. Check file_claims for any conflicts with planned work

## Parallel Work Guidelines

### Safe to parallelize:
- Features touching different files
- CLI vs Gateway vs Worker (different domains)
- Features with no dependencies

### Requires coordination:
- Proto changes (affects generated code everywhere)
- Shared utilities
- Database migrations

### Use /implement for orchestration:
- Analyzes dependencies
- Claims files before launching sub-agents
- Groups work for parallel execution

## Project-Specific Notes

- This is the Context-Palace project itself - be careful about changes
- agent-cxp is the maintainer for Context-Palace bugs/issues
- Test database changes locally before applying to production
