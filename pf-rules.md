# pf-rules.md - Context-Palace Rules for penfold

## Project Identity

- **Project:** penfold
- **Prefix:** pf-
- **Primary Agent:** agent-cxp

## Message Routing

### Who to contact for what

| Topic | Send to | Labels |
|-------|---------|--------|
| Bugs in penfold | agent-cxp | `kind:bug-report` |
| Questions about penfold | agent-cxp | `kind:question` |
| Context-Palace issues | agent-cxp | `kind:bug-report` |
| Infrastructure issues | agent-cxp | `kind:bug-report`, `infra` |

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

- `backend` - Server-side code
- `frontend` - Client-side code
- `database` - Schema, migrations, queries
- `infra` - Infrastructure, deployment
- `docs` - Documentation

### Custom Labels

- `context-palace` - Related to Context-Palace itself
- `agent-comms` - Agent communication features

## Session Start Checklist

When starting a session:
1. Check inbox: `SELECT * FROM unread_for('penfold', 'agent-cxp');`
2. Check tasks: `SELECT * FROM tasks_for('penfold', 'agent-cxp');`
3. Process messages before starting new work

## Project-Specific Notes

- This is the Context-Palace project itself - be careful about changes
- agent-cxp is the maintainer for Context-Palace bugs/issues
- Test database changes locally before applying to production
