# Task Launch Pack: KB Trigger Routing And Organization

## Task

```text
Investigate how KB tree trigger metadata is stored and surfaced so Context Palace can organize its own knowledge more effectively.
```

## Primary Objective

- get to:
  - the first correct subsystem plus executable check for how trigger metadata is authored, stored, and surfaced in navigation

## Why This Pack Exists

- dominant risk:
  - wrong first turn into architecture prose without identifying the concrete edge-metadata path
- why a thin launcher is incomplete here:
  - the task spans authoring commands, client storage logic, and user-facing navigation displays

## Source Of Truth

- proof surfaces:
  - `cxp/internal/client/knowledge_children.go`
  - `cxp/cmd/knowledge_children.go`
  - `cxp/cmd/kb.go`
  - `cxp/cmd/shard.go`
- prose role:
  - explain why the routing text matters, not where the behavior is implemented

## First Credible Route

- likely subsystem:
  - knowledge-child edge metadata and KB navigation surfaces
- likely code entrypoints:
  - `cxp/internal/client/knowledge_children.go`
  - `cxp/cmd/knowledge_children.go`
  - `cxp/cmd/kb.go`
  - `cxp/cmd/shard.go`
- nearest executable evidence:
  - create or inspect a child relationship with `cxp knowledge children add/list`
  - compare with `cxp shard show <parent>` and `cxp kb tree`

## Disambiguation

- ambiguity:
  - is routing metadata stored on shards, on edges, or only exposed through derived tree functions?
- how to disambiguate quickly:
  - check `AddKnowledgeChild` and `ListKnowledgeChildren` first, then verify which CLI surfaces display the same fields

## Trust Guidance

- likely current:
  - `README.md` KB navigation section
- use with caution:
  - architecture docs that explain the intended retrieval model but not the exact implementation surface
- why:
  - this task is best resolved from the code plus direct CLI behavior

## Minimal Supporting Prose

- `README.md`
  - use to confirm the intended user model
- `docs/kb-shard-architecture.md`
  - use only for background on why this organization matters

## First Three Moves

1. inspect `cxp/internal/client/knowledge_children.go` to locate where trigger and description are stored
2. inspect `cxp/cmd/knowledge_children.go` and `cxp/cmd/shard.go` to see how operators and agents see that metadata
3. verify live behavior with `cxp knowledge children list`, `cxp kb tree`, and `cxp shard show`

## Stop Conditions

- if edge metadata is clearly the source of truth:
  - focus future organization work on creating strong trigger/description text and making display surfaces consistent
- if display surfaces omit important routing metadata:
  - treat that as a navigation UX gap rather than a docs-only problem

## Why This Should Beat The Launcher

- pre-assembled route:
  - it immediately ties docs organization to the precise storage and display path
- earlier proof linkage:
  - it points straight to the commands that prove the behavior
- earlier trust calibration:
  - it prevents spending the first turn in prose that explains the philosophy without revealing the implementation path
