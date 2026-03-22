# M Pipeline — Automated Design-to-Delivery

M Pipeline turns designs into working code by orchestrating agents through a structured pipeline: decompose, dispatch, implement, review, merge. M is an ephemeral orchestrator — it reads shard state, takes one action, updates state, and exits. Kill it anytime; the next M picks up.

## Status

**v1 implemented** (2026-03-22). All pipeline tooling merged to main across two PRs:
- [PR #1](https://github.com/otherjamesbrown/context-palace/pull/1) — Core pipeline: 19 tasks, 4 dependency waves
- [PR #2](https://github.com/otherjamesbrown/context-palace/pull/2) — Per-repo config and registry system

Not yet running autonomously. The poller exists but isn't deployed as a service.

## Key Shards

| Shard | Type | Description |
|-------|------|-------------|
| `pf-a23493` | design | M Pipeline design document (the full spec) |
| `pf-4bdd1e` | outcome | Automated AI-code creation pipeline (parent outcome) |
| `pf-ca054d` | outcome | futures - CoWork Style (research on Claude CoWork patterns) |

View task graph: `cxp task deps pf-a23493`

## Quick Start

### Set up a repo

```bash
cd ~/github/otherjamesbrown/context-palace
cxp pipeline setup
```

Creates `.cxp/pipeline.yaml` (per-repo config) and registers the repo in `~/.cxp/repos.yaml`. Auto-detects language, build/test commands, GitHub remote.

### Initialize a pipeline on a design

```bash
cxp shard pipeline init <design-id>
```

### Check status

```bash
cxp dashboard                          # high-level overview
cxp shard pipeline show <design-id>    # pipeline state
cxp task deps <design-id>              # dependency graph
```

### Dispatch work

```bash
cxp task dispatch <task-id>            # single task → agent in tmux
cxp task dispatch-all <design-id>      # all ready tasks respecting deps
cxp task dispatch <task-id> --dry-run  # preview without executing
```

### Review and merge

```bash
cxp task review <task-id>              # spawn reviewer in tmux
cxp task review-verdict <id> approve   # record verdict
cxp task pr merge <task-id>            # squash merge, clean up
```

### Run the poller (autonomous mode)

```bash
cxp pipeline poller                    # 30s loop, spawns M on triggers
cxp pipeline poller --once --dry-run   # single check, preview only
```

## Architecture

```
Design shard → Pipeline metadata (phase, locks, waiting_for, tasks)
     ↓
Poller detects actionable state → spawns M in tmux
     ↓
M reads playbook (skills/m-playbook.md) + pipeline state
     ↓
M takes one action (dispatch, review, merge, escalate)
     ↓
M updates pipeline state → exits
     ↓
Poller spawns next M when conditions change
```

### Config hierarchy (Claude Code pattern)

```
~/.cxp/pipeline.yaml          # global defaults
<repo>/.cxp/pipeline.yaml     # repo overrides (wins)

<repo>/skills/                 # repo-specific skills
~/.cxp/skills/                 # global fallback skills
```

### Pipeline phases

| Phase | What happens |
|-------|-------------|
| `design` | M checks readiness, runs implementability check |
| `decompose` | Domain agent breaks design into tasks with deps |
| `implement` | Tasks dispatched to agents in isolated worktrees |
| `review` | PRs reviewed against task spec and design |
| `done` | All tasks merged and verified |

### Concurrency

Each pipeline shard has a lock (session ID + 5-min TTL). Only one M operates on a pipeline at a time. Stale locks auto-expire.

## Commands Reference

| Command | Purpose |
|---------|---------|
| `cxp pipeline setup` | Register a repo for pipeline automation |
| `cxp pipeline poller` | Run the trigger poller |
| `cxp shard pipeline init/show/update` | Pipeline state management |
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
| `skills/m-*.md` | Phase-specific skill files (6 total) |
| `.cxp/pipeline.yaml` | Per-repo pipeline config |
| `.claude/hooks/m-session-start.sh` | M session context hydration |

## What's Next

- **Poller deployment** (`pf-20e7a2`) — launchd service on dev01 for autonomous operation
- **Grafana panels** (`pf-7ff195`) — monitoring dashboard on dev02
- **CoWork patterns** (`pf-ca054d`) — plugin model, MCP connectors for Jira/Linear, hook-based context injection
