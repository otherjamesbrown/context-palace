# Context Palace — INTEROP

How agents in **other projects** interact with Context Palace. Load this when you need to create work items, read or write KB articles, subscribe to scheduled workflows, or cite CP shards from outside the context-palace repo. For working **inside** context-palace, read `CLAUDE.md` instead.

## What Context Palace produces

- **Shard store** `[Live]` — every design, task, bug, knowledge article, message, memory, and handoff in every project. Shard IDs are project-prefixed (`pf-`, `my-`, `cb-`, `mi-`, `mp-`, `cp-`). Accessed via the `cxp` CLI.
- **Knowledge base with hot/warm/cold retrieval** `[Live]` — trigger-based tree navigation plus hybrid BM25+vector search. `cxp kb search|show|create`.
- **Scheduled workflow runner** `[Live]` — periodic workflows (drift-scan, canary, triage, custom). Runs tracked in `schedules` / `schedule_runs`.
- **Canonical-shape citation rendering in `cxp kb` output** `[Planned]` — `(Context Palace, <kind> <id>, <date>) [Tx]` for copy-paste. Follow-up to the 2026-04-15 citation-format adoption.

## What Context Palace consumes from other projects

| From | What | Mechanism | Status |
|------|------|-----------|--------|
| Any project | Work items (designs/tasks/bugs) | `cxp` or `cobuild wi create --project context-palace` | Live |
| Any project | KB shards under the project's own prefix | `cxp knowledge create --project <name>` | Live |
| Any project | KB gap reports | `/kb-gaps` or `cxp shard append` to the project's gaps shard | Live |
| Any project | Schedule subscriptions | `cxp schedule create --project <name>` | Live |

## How to interact with us

Always pass `--project <name>` when touching another project's pool — omitting it silently writes into your own.

```bash
cxp bug create --project <target> "Title" --body "..."        # file a bug/task/design
cxp kb search "query"                                         # hybrid BM25 + vector search
cxp kb show <shard-id>                                        # article + children
cxp knowledge create --title "..." --body "..." --parent <id> # create/update articles
cxp schedule create <name> --workflow <type> --cron "<expr>"  # subscribe to periodic work
```

For the full reference: `cxp --help` or the `README.md`.

## Where to look

| Need | Location |
|------|----------|
| KB shard concepts | `docs/kb-shard-architecture.md` |
| Bootstrapping and structuring a KB | `docs/kb-authoring-guide.md` |
| Keeping a KB accurate over time | `docs/kb-maintenance-guide.md` |
| Schema and SQL conventions | `context-palace.md` |

## How to cite Context Palace

Format is defined in `~/decisions/citation-format.md`. Context Palace's `<ref>` convention: `<kind> <id>` where kind is one of:

- `kb` — a knowledge-base article (the common case; `cxp kb search` / `cxp kb show`)
- `shard` — any other shard: work items (design/task/bug), messages, memories, handoffs, triage

KB articles are shards in CP's schema; the two-kind split is a surface convention for citation clarity, not a separate namespace. Date is shard `updated_at`.

Default tiers: **T2** for `kb` (summarised knowledge); **T1** for `shard` when the shard is a message, raw source, or work-item record; **T3** for `shard` when the shard is a handoff, triage, or playbook extract.

Examples:

> ✓ "Per the KB article on shard retrieval (Context Palace, kb cp-a1b2c3, 2026-04-10) [T2]."
> ✓ "Penfold's triage hand-off flagged this (Context Palace, shard pf-9f8e7d, 2026-04-14) [T3]."
> ✗ "The KB says X" — no ref, no date, not re-resolvable.

## Don't modify from outside Context Palace

- `cxp/migrations/` — schema is shared infrastructure; changes go through context-palace designs.
- Another project's knowledge-tree structure — parent/child edges and triggers.
- Shards you don't own — read freely; write only via your own project's prefix.

Reading any shard from any project is encouraged. Writing into another project's namespace needs `--project <name>` and a clear reason.

## Requesting changes

```bash
cobuild wi create --type <bug|task|design> --project context-palace \
  --title "..." \
  --body "..."
```

Good reports include: the `cxp` command that failed (or expected-vs-actual behaviour), shard IDs involved, and — for KB retrieval issues — the query string and the top-k results you saw.

## Critical gotchas

1. **The `--project` flag is load-bearing.** Omitting it silently writes into your own project's pool. This is the single most common cause of shards landing in the wrong place, and there is no post-hoc warning — the shard just appears in the wrong pool. When in doubt, pass it explicitly.
2. **KBs don't maintain themselves.** New knowledge namespaces rot silently unless you subscribe to the drift-scan, canary, and triage workflows via `cxp schedule create`. The maintenance machinery is free, but opt-in. If you set up a KB and never subscribe, expect gradual retrieval-quality decay with no signal.
