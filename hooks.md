# Claude Code Hooks Integration

Context Palace integrates with Claude Code's hook system to automatically preserve and restore session context across conversations.

## Overview

Three hooks are configured in `.claude/settings.json` within each project repo:

| Event | Trigger | Action |
|-------|---------|--------|
| **SessionStart** | `startup`, `resume` | Injects last checkpoint context into the conversation |
| **PreCompact** | Context window compression | Saves a checkpoint before context is lost |
| **SessionEnd** | `clear`, `prompt_input_exit` | Saves a checkpoint on exit |

The pattern is: **read on open, write on compress and close.**

## Hook Configuration

```json
{
  "hooks": {
    "SessionStart": [{
      "matcher": "startup|resume",
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/session-start.sh",
        "timeout": 10
      }]
    }],
    "PreCompact": [{
      "hooks": [{
        "type": "command",
        "command": "cxp session checkpoint '[auto-compact] Context compressed' 2>/dev/null; true",
        "timeout": 10
      }]
    }],
    "SessionEnd": [{
      "matcher": "clear|prompt_input_exit",
      "hooks": [{
        "type": "command",
        "command": "cxp session checkpoint '[auto-exit] Session ended' 2>/dev/null; true",
        "timeout": 5
      }]
    }]
  }
}
```

## Hook Details

### SessionStart (read)

Runs `.claude/hooks/session-start.sh`, which:

1. Tries `cxp session inject --tag main` (outputs formatted context from the last checkpoint)
2. Falls back to `cxp session show -o json` for basic session info if inject isn't available
3. stdout becomes `additionalContext` in Claude Code's system prompt

The injected context includes the session ID, last checkpoint note (what was being worked on, what's next), and inbox count. This gives the agent continuity across conversations without manual briefing.

### PreCompact (write)

Fires when Claude Code is about to compress the conversation to stay within context limits. Runs:

```bash
cxp session checkpoint '[auto-compact] Context compressed'
```

This is critical — without it, the agent would lose awareness of what it was doing when context gets trimmed. The checkpoint preserves state so the next `SessionStart` inject can restore it.

### SessionEnd (write)

Fires on `clear` or exit. Runs:

```bash
cxp session checkpoint '[auto-exit] Session ended'
```

Captures final state so the next session can pick up where this one left off.

## Constraints

- **Only `command` hooks work** for SessionStart, PreCompact, and SessionEnd events. The `agent` and `prompt` hook types are not supported for these events (they only work for PreToolUse, PostToolUse, etc.).
- This means PreCompact **cannot do LLM-based summarization** — it can only run a simple shell command.
- All hooks include `2>/dev/null; true` or `set -euo pipefail` to fail gracefully and not block the agent.

## Richer Context via Skills

The hooks handle automatic checkpoint/restore, but richer context operations are done through skills (slash commands) that the agent invokes explicitly:

- `/handoff [tag]` — Writes a detailed handoff shard with structured context for another agent or a future session
- `/pickup [tag]` — Reads handoff shards and restores detailed working context
- `/recap` — Morning briefing: inbox, open tasks, recent activity
- `/session-end` — Explicit session close with summary (more detailed than the auto hook)

These skills can use the full LLM to generate summaries, unlike the command-only hooks.

## Data Flow

```
Session Start:
  Claude Code starts → SessionStart hook fires
  → session-start.sh → cxp session inject --tag main
  → Last checkpoint text injected as system context

During Work:
  Agent uses /handoff, /pickup, cxp memory, cxp message, etc.
  Context Palace stores shards in PostgreSQL

Context Compression:
  Claude Code hits context limit → PreCompact hook fires
  → cxp session checkpoint "[auto-compact] ..."
  → State preserved before compression

Session End:
  User exits → SessionEnd hook fires
  → cxp session checkpoint "[auto-exit] ..."
  → Final state captured for next session
```
