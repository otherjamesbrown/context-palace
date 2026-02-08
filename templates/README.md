# Context Palace — Project Templates

Templates that `cp init` scaffolds into new projects. Each template has placeholders
that get replaced with project-specific values during initialization.

## Templates

| File | Destination | Purpose |
|------|-------------|---------|
| `ways-of-working.md` | `docs/ways-of-working.md` | Bug/feature/spec templates, DoD, quality gates, escalation |
| `ingest.md` | `.claude/commands/ingest.md` | Pipeline orchestrator — classify, investigate, implement, deploy |
| `SPEC-TEMPLATE.md` | `docs/SPEC-TEMPLATE.md` | Full spec format for complex features |

## Placeholders

| Placeholder | Description | Example |
|-------------|-------------|---------|
| `[PROJECT]` | Project name | `penfold` |
| `[ORCHESTRATOR]` | Orchestrating agent name | `agent-penfold` |
| `[IMPLEMENTER]` | Implementing agent name | `agent-mycroft` |
| `[MAINTAINER]` | Platform/infra agent name | `agent-cxp` |
| `[DB_CONN]` | Context Palace connection string | `host=dev02.brown.chat ...` |
| `[PALACE_CLI]` | Path to palace CLI binary | `/Users/dev/bin/palace` |
| `[PREFIX]` | Project shard prefix | `pf` |

## cp init — What It Should Do

`cp init [project-name]` sets up a new project for Context Palace agent workflows:

### Step 1: Register Project

```sql
INSERT INTO projects (name, prefix) VALUES ('[PROJECT]', '[PREFIX]');
```

### Step 2: Scaffold Files

Copy templates into the project directory with placeholders replaced:

```
[project-root]/
├── CLAUDE.md                          ← from claude-template.md
├── context-palace.md                  ← full CP usage guide
├── docs/
│   ├── ways-of-working.md            ← from templates/ways-of-working.md
│   └── SPEC-TEMPLATE.md              ← from specs/cp-cli/SPEC-TEMPLATE.md
└── .claude/
    └── commands/
        └── ingest.md                  ← from templates/ingest.md
```

### Step 3: Create Project Rules Shard

```sql
SELECT create_shard('[PROJECT]', 'Project Rules for [PROJECT]',
  '...', 'doc', '[ORCHESTRATOR]');
```

### Step 4: Customize

The user then:
1. Adds project-specific components to the bug/feature templates
2. Adds domain-specific quality metrics
3. Creates ingest phase files (`.claude/commands/ingest.*.md`) based on the penfold
   reference implementation or writes their own
4. Defines agent roles and routing rules in ways-of-working

### What cp init Does NOT Do

- Does not create the ingest phase files (`.claude/commands/ingest.*.md`) — these are
  project-specific. The orchestrator template describes what each phase does; the project
  implements them.
- Does not set up CI/CD — that's project infrastructure
- Does not create the project repo — assumes it exists

## Reference Implementation

The penfold project (`~/github/otherjamesbrown/penfold/`) is the reference implementation:
- Full 8-file ingest pipeline in `.claude/commands/`
- Customized ways-of-working with quality metrics for email processing
- SPEC-TEMPLATE used for cp-cli specs

New projects should start from the templates and reference penfold for examples of
how to flesh out the phase files.
