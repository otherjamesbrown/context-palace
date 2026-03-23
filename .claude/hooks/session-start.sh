#!/bin/bash
# SessionStart hook — injects context from Context Palace
# No session required. Board, playbook, and work queue work independently.
# stdout becomes additionalContext in Claude Code.

set -euo pipefail

# If dispatched by the pipeline, skip the work queue and menu.
if [ "${CXP_DISPATCH:-}" = "true" ]; then
  echo "[Dispatched session — task context provided via prompt]"
  exit 0
fi

# Instance identity (first 8 chars of conversation UUID)
PROJ_DIR="$HOME/.claude/projects/-Users-james-github-otherjamesbrown-context-palace"
INSTANCE_ID=$(basename "$(ls -t "$PROJ_DIR"/*.jsonl 2>/dev/null | head -1)" .jsonl 2>/dev/null | cut -c1-8)
echo "[Instance: steve:${INSTANCE_ID:-unknown}]"

# Board
echo ""
cxp session board --agent agent-steve 2>/dev/null || true

# Playbook
echo ""
echo "# Playbook #"
echo ""
cxp knowledge show pf-a6209a --agent agent-steve 2>/dev/null || true

# Work queue: ready items for Steve
echo ""
echo "# Work Queue #"
echo ""
cxp shard list --type bug,task --assigned-to agent-steve --agent agent-steve 2>/dev/null || true

# In-progress items
echo ""
echo "# In Progress #"
echo ""
cxp shard list --status in_progress --assigned-to agent-steve --agent agent-steve 2>/dev/null || true

# Blocked items
echo ""
echo "# Blocked #"
echo ""
cxp shard list --label blocked --owner agent-steve --agent agent-steve 2>/dev/null || true

# Presentation instructions
cat <<'INST'

# Startup Instructions

Present the work queue above to James as a table:

| # | ID | Type | Title |
|---|-----|------|-------|
| 1 | pf-xxx | bug | ... |

Then ask what he wants to work on:

1. All items
2. Bugs only (list IDs)
3. Tasks only (list IDs)
4. Pick specific items

If there are in-progress items from a previous session, mention them first and ask
whether to resume or start fresh work.

If there are blocked items, flag them prominently — these need James's input.

Do NOT check the inbox — context is injected by this hook.
Do NOT send messages — communicate via shard status transitions.
INST
