#!/bin/bash
# M Session Start hook — hydrates M with pipeline context
# Called when M is spawned by the poller or manually.
# Expects PIPELINE_SHARD_ID environment variable or first argument.
# stdout becomes additionalContext in Claude Code.

set -euo pipefail

PIPELINE_ID="${PIPELINE_SHARD_ID:-${1:-}}"

if [ -z "$PIPELINE_ID" ]; then
  echo "ERROR: No pipeline shard ID. Set PIPELINE_SHARD_ID or pass as argument."
  exit 1
fi

# Identity
echo "[Instance: M:${PIPELINE_ID}]"
echo ""

# M Playbook
echo "# M Playbook #"
echo ""
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
if [ -f "$REPO_ROOT/skills/m-playbook.md" ]; then
  cat "$REPO_ROOT/skills/m-playbook.md"
else
  echo "WARNING: m-playbook.md not found at $REPO_ROOT/skills/"
fi
echo ""

# Pipeline state
echo "# Pipeline State #"
echo ""
cxp shard pipeline show "$PIPELINE_ID" -o json 2>/dev/null || echo '{"error": "failed to read pipeline state"}'
echo ""

# Design shard
echo "# Design Shard #"
echo ""
cxp shard show "$PIPELINE_ID" -o json 2>/dev/null || echo '{"error": "failed to read design shard"}'
echo ""

# Active task shards
echo "# Task Dependency Graph #"
echo ""
cxp task deps "$PIPELINE_ID" -o json 2>/dev/null || echo '{"waves": [], "dispatchable": []}'
echo ""

# In-progress tasks with details
echo "# Active Tasks #"
echo ""
TASK_IDS=$(cxp task deps "$PIPELINE_ID" -o json 2>/dev/null | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for wave in data.get('waves', []):
        for task in wave.get('tasks', []):
            if task.get('status') in ('in_progress', 'needs-review'):
                print(task['id'])
except:
    pass
" 2>/dev/null)

if [ -n "$TASK_IDS" ]; then
  for tid in $TASK_IDS; do
    echo "## Task: $tid"
    cxp shard show "$tid" -o json 2>/dev/null | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f\"Title: {d['title']}\")
print(f\"Status: {d['status']}\")
print(f\"Content: {d['content'][:500]}...\")
" 2>/dev/null || true
    echo ""
  done
else
  echo "(no active tasks)"
fi

# Instructions
cat <<'INST'

# M Instructions

You are M, an ephemeral pipeline orchestrator. Your playbook is loaded above.

1. Read the pipeline state to determine your current phase
2. Follow the decision tree for that phase in the playbook
3. Take ONE action, update pipeline state, and exit
4. Always lock before acting: `cxp shard pipeline lock <id>`
5. Always unlock when done: `cxp shard pipeline unlock <id>`

If you cannot determine what to do, escalate:
```bash
cxp shard append <pipeline-id> --body "M session: unable to determine next action. Escalating."
cxp shard label add <pipeline-id> blocked
```
INST
