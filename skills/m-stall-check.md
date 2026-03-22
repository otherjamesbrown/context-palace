# M Skill: Stall Diagnosis

You are M, diagnosing a stalled task.

## Input
- Task shard ID (in_progress, no progress for >30 minutes)

## Steps

### 1. Gather context
```bash
cxp task get <task-id>
cxp shard show <task-id>
```
Note: updated_at, any progress notes, evidence appended.

### 2. Check tmux
Is the agent session still running?
```bash
tmux list-windows -t main | grep <task-id>
```

### 3. Diagnose

**Agent crashed (no tmux window)**
→ Re-dispatch:
```bash
cxp shard status <task-id> ready
cxp task dispatch <task-id>
```

**Agent running but no progress**
Check iteration count (if available from shard content).

| Iterations | Action |
|-----------|--------|
| <= 5 | Leave it. Exit. Check again next cycle. |
| 6-10 | Diagnose root cause (see below) |
| > 10 | Escalate to James |

### 4. Root cause analysis (6-10 iterations)

Read the task shard content and any progress notes. Determine:

**Scoping problem** — task is too large or crosses subsystems
→ Back to decomposition:
```bash
cxp shard append <task-id> --body "Stall diagnosis: task scope too broad. Needs re-decomposition."
cxp shard label add <task-id> blocked
```

**Missing information** — agent needs a decision or clarification
→ Surface the question:
```bash
cxp shard append <task-id> --body "Stall diagnosis: agent blocked on: <question>"
cxp shard label add <task-id> blocked
```

**Technical limitation** — hitting model limits or API issues
→ Note for James:
```bash
cxp shard append <task-id> --body "Stall diagnosis: technical limitation. <details>"
cxp shard label add <task-id> blocked
```

### 5. Escalation (>10 iterations)
```bash
cxp shard append <task-id> --body "## Escalation
Task stalled for >10 iterations.
Agent: <agent>
Last progress: <timestamp>
Diagnosis: <summary>
Action needed: James to review and re-scope or unblock."
cxp shard label add <task-id> blocked
```
