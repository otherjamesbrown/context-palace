# M Pipeline — Automated Design-to-Delivery

M Pipeline turns designs into working code by orchestrating agents through a structured pipeline with enforced stage gates: design review, decomposition with integration tests, dispatch, implement, review, merge. M is an ephemeral orchestrator — it reads shard state, takes one action, updates state, and exits. Kill it anytime; the next M picks up.

## Status

**v1 implemented** (2026-03-22), **stage gates added** (2026-03-23).

- [PR #1](https://github.com/otherjamesbrown/context-palace/pull/1) — Core pipeline: commands, poller, skills
- [PR #2](https://github.com/otherjamesbrown/context-palace/pull/2) — Per-repo config and registry system

First real use: 6 penfold pipeline designs taken through Phase 1 (readiness review) and Phase 2 (decomposition with integration tests and cross-design ordering).

## Key Shards

| Shard | Type | Description |
|-------|------|-------------|
| `pf-a23493` | design | M Pipeline design document (the full spec) |
| `pf-4bdd1e` | outcome | Automated AI-code creation pipeline (parent outcome) |
| `pf-0ef956` | outcome | Reliable, data-driven content pipeline (penfold designs) |
| `pf-ca054d` | outcome | futures - CoWork Style (research on Claude CoWork patterns) |
| `pf-2b22a3` | knowledge | Cross-design review: 6 penfold pipeline designs |
| `pf-f5c919` | design | Cross-design integration test (blocked by all 6 designs) |

## How It Works — The Full Pipeline

### Phase 1: Design Review (`design` → `decompose`)

An agent creates a design following `skills/create-design.md`. M evaluates it:

1. **Readiness check** (5 criteria): links to outcome, problem stated, user identified, success criteria, scope boundaries
2. **Implementability check**: could an agent code this without asking questions?
3. **Record verdict** — creates audit trail automatically:
   ```bash
   cxp shard pipeline review <design-id> --verdict pass|fail --readiness <N> --body "<findings>"
   ```

The review command:
- Creates a `review` sub-shard linked to the design (audit trail)
- Updates pipeline metadata with structured verdict, round number, history
- If pass: advances to `decompose`
- If fail: stays in `design` — fix gaps and re-run

Designs that fail get specific feedback. The process loops until they pass.

### Phase 2: Decomposition (`decompose` → `implement`)

A domain agent breaks the design into tasks:

1. **Structure pass** — produce task tree with titles, scope, deps
2. **Detail review** — verify each task is single-session sized, has testable criteria, code locations
3. **Create tasks** with `cxp task create --parent <design-id>`
4. **Create integration test task** (required) — labeled `integration-test`, blocked by all other tasks, converts design success criteria into concrete test cases
5. **Decomposition review** — a separate agent validates the breakdown (right-sized? gaps? correct deps?)
6. **Record verdict:**
   ```bash
   cxp shard pipeline decompose <design-id> --verdict pass|fail --body "<findings>"
   ```

The decompose gate validates:
- At least one task exists
- All tasks have substantive content
- Dependencies are acyclic
- **An integration test task with label `integration-test` exists**

If validation fails, you get a specific error. Fix and retry.

### Cross-Design Ordering

When multiple designs touch the same codebase, use `blocked-by` edges between designs to enforce implementation order:

```bash
cxp shard link <later-design> --blocked-by <earlier-design>
```

Create a cross-design integration test design (blocked by all others) to verify they work together after independent implementation.

### Phase 3: Implement (`implement`)

Tasks dispatched to agents in isolated worktrees:

```bash
cxp task dispatch <task-id>              # single task
cxp task dispatch-all <design-id>        # all ready tasks
```

Each agent receives: task spec, design context, completion instructions (test, evidence, PR, needs-review).

### Phase 4: Review (`review`)

```bash
cxp task review <task-id>                # spawn reviewer
cxp task review-verdict <id> approve     # record verdict
cxp task pr merge <task-id>              # squash merge, clean up
```

### Phase 5: Done

All tasks merged, integration tests pass, design closed.

## Setup

### Register a repo

```bash
cd ~/github/otherjamesbrown/penfold
cxp pipeline setup
```

Auto-detects language, build/test commands, GitHub remote. Creates `.cxp/pipeline.yaml` and updates `~/.cxp/repos.yaml`.

### Initialize a pipeline

```bash
cxp shard pipeline init <design-id>
```

### Check status

```bash
cxp dashboard                          # high-level overview
cxp shard pipeline show <design-id>    # pipeline state + review history
cxp task deps <design-id>              # dependency graph
```

## Config Hierarchy (Claude Code pattern)

```
~/.cxp/pipeline.yaml          # global defaults
<repo>/.cxp/pipeline.yaml     # repo overrides (wins)

<repo>/skills/                 # repo-specific skills
~/.cxp/skills/                 # global fallback skills
```

## Skills

| Skill | Who uses it | Purpose |
|-------|------------|---------|
| `skills/create-design.md` | Any agent creating designs | Required structure, implementability test |
| `skills/m-playbook.md` | M (orchestrator) | Decision trees, phase rules, commands |
| `skills/m-readiness-check.md` | M | Phase 1 readiness + implementability evaluation |
| `skills/m-implementability.md` | M | Implementability criteria reference |
| `skills/m-dispatch-task.md` | M | Phase 3 task dispatch procedure |
| `skills/m-review-pr.md` | M | Phase 4 PR review procedure |
| `skills/m-merge-and-verify.md` | M | Phase 4 merge + post-merge verification |
| `skills/m-stall-check.md` | M | Stall diagnosis for stuck agents |

`create-design.md` and `m-readiness-check.md` are cross-referenced — the design skill tells authors what's required, the readiness check evaluates against the same criteria.

## Stage Gates

Every phase transition is enforced by a command that validates preconditions and creates an audit trail:

| Gate | Command | What it checks |
|------|---------|---------------|
| design → decompose | `cxp shard pipeline review` | 5 readiness criteria + implementability |
| decompose → implement | `cxp shard pipeline decompose` | Tasks exist, have content, deps acyclic, integration test exists |

Both commands create sub-shards (type `review`) linked to the design. Pipeline metadata tracks round numbers and history. You cannot advance a phase without going through the gate.

## Commands Reference

| Command | Purpose |
|---------|---------|
| `cxp pipeline setup` | Register a repo for pipeline automation |
| `cxp pipeline poller` | Run the trigger poller |
| `cxp shard pipeline init/show/update` | Pipeline state management |
| `cxp shard pipeline review` | Phase 1 gate: readiness review with audit trail |
| `cxp shard pipeline decompose` | Phase 2 gate: decomposition validation with audit trail |
| `cxp shard pipeline lock/unlock/lock-check` | Concurrency control |
| `cxp dashboard` | Outcomes, pipelines, blockers, agents |
| `cxp task deps <design-id>` | Dependency graph with dispatch plan |
| `cxp task dispatch / dispatch-all` | Spawn agents with full context |
| `cxp task worktree create/show/remove` | Isolated git worktrees |
| `cxp task evidence` | Structured evidence append |
| `cxp task pr create / merge` | PR lifecycle via gh CLI |
| `cxp task review / review-verdict` | Review dispatch and verdicts |

## Files

| Path | Purpose |
|------|---------|
| `skills/m-playbook.md` | M's decision trees and phase rules |
| `skills/m-*.md` | Phase-specific skill files |
| `skills/create-design.md` | Design authoring guide for all agents |
| `.cxp/pipeline.yaml` | Per-repo pipeline config |
| `~/.cxp/repos.yaml` | Global repo registry (project → repo path) |
| `.claude/hooks/m-session-start.sh` | M session context hydration |

## Registered Repos

| Project | Repo | Build |
|---------|------|-------|
| penfold | ~/github/otherjamesbrown/penfold | `cd penfold-go-pipeline && go build/test` |
| context-palace | ~/github/otherjamesbrown/context-palace | `cd cxp && go build/test` |

## What's Next

- **Poller deployment** (`pf-20e7a2`) — launchd service on dev01 for autonomous operation
- **Grafana panels** (`pf-7ff195`) — monitoring dashboard on dev02
- **Session/output capture** — record agent output as task_runs for full traceability
- **CoWork patterns** (`pf-ca054d`) — plugin model, MCP connectors for Jira/Linear
