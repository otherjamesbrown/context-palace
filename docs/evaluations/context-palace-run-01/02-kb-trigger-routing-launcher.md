# Strong Launcher: KB Trigger Routing And Organization

## Task

```text
Investigate how KB tree trigger metadata is stored and surfaced so Context Palace can organize its own knowledge more effectively.
```

## Repo Rules And Source Of Truth

- rules:
  - prefer code paths that actually create, store, and render child trigger metadata
- source-of-truth order:
  - knowledge children commands and client logic
  - KB tree/search and shard display commands
  - README explanations of navigation behavior
- workflow constraints:
  - understand the actual retrieval surface before proposing new docs organization

## Tiny Repo Map

- knowledge child management:
  - `cxp/cmd/knowledge_children.go`, `cxp/internal/client/knowledge_children.go`
  - likely relevant because trigger/description metadata is created and listed here
- KB browsing:
  - `cxp/cmd/kb.go`
  - likely relevant because tree search/navigation surfaces child metadata
- shard display:
  - `cxp/cmd/shard.go`
  - likely relevant because `shard show` presents children and related metadata

## Task Classification

- likely task type:
  - investigation / architecture understanding
- likely subsystem candidates:
  - knowledge child relationship storage
  - KB tree display
  - shard detail display
- main ambiguity:
  - whether routing metadata primarily lives on edges, on shard metadata, or in derived tree queries

## Likely Code Starting Points

- `cxp/cmd/knowledge_children.go`
- `cxp/internal/client/knowledge_children.go`
- `cxp/cmd/kb.go`
- `cxp/cmd/shard.go`

## Likely Tests Or Executable Checks

- `cxp knowledge children add ... --description ... --trigger ...`
  - confirms how metadata is authored
- `cxp knowledge children list <parent-id>`
  - confirms how metadata is surfaced directly
- `cxp kb tree` or `cxp shard show <id>`
  - confirms where routing data becomes visible in navigation

## Relevant Prose Fallback

- `README.md`
  - explains warm navigation, triggers, and descriptions
- `docs/kb-shard-architecture.md`
  - explains why trigger text matters for agent navigation

## Warnings

- docs describe the intended behavior strongly, but this task is really about actual storage and display surfaces
- there may be multiple user-facing routes to the same metadata

## Expected First Moves

1. inspect `cxp/internal/client/knowledge_children.go`
2. inspect `cxp/cmd/knowledge_children.go`
3. inspect `cxp/cmd/kb.go` and `cxp/cmd/shard.go`
