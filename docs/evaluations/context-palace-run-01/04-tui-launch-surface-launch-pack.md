# Task Launch Pack: TUI Launch Surface

## Task

```text
Investigate how the TUI currently surfaces knowledge hierarchy, access counts, and shard detail, and whether it already behaves like a launch surface for short tasks.
```

## Primary Objective

- get to:
  - the first correct subsystem plus executable evidence for whether `cxpv` already acts like a launch surface and what an agent must know about it

## Why This Pack Exists

- dominant risk:
  - missing a major product surface because the first turn stays inside CLI commands and docs
- why a thin launcher is incomplete here:
  - the TUI pulls together board, work, KB tree, messages, and detail rendering, which can materially change how launch context is surfaced

## Source Of Truth

- proof surfaces:
  - `cxp/cmd/cxpv/main.go`
  - `cxp/internal/tui/model.go`
  - `cxp/internal/tui/detail.go`
  - `cxp/internal/tui/tree.go`
- prose role:
  - compare intended retrieval ideas with what the TUI already exposes

## First Credible Route

- likely subsystem:
  - TUI browse model and detail rendering
- likely code entrypoints:
  - `cxp/cmd/cxpv/main.go`
  - `cxp/internal/tui/model.go`
  - `cxp/internal/tui/detail.go`
  - `cxp/internal/tui/tree.go`
- nearest executable evidence:
  - run `cxpv`
  - inspect tabs and detail rendering
  - trace `loadKBTree`, `loadBoard`, `loadDetail`, and `RenderDetail`

## Disambiguation

- ambiguity:
  - is the TUI just a browser over shards, or is it already packaging enough route and evidence to act as a launch surface?
- how to disambiguate quickly:
  - compare what the TUI preloads and renders against what the launch-pack proposal says an agent needs for the first turn

## Trust Guidance

- likely current:
  - TUI implementation files
- use with caution:
  - evaluation/proposal docs that describe future launch behavior
- why:
  - this task is about current product reality, not just planned direction

## Minimal Supporting Prose

- `README.md`
  - use only to compare current KB navigation intent with the TUI implementation

## First Three Moves

1. inspect `cxp/internal/tui/model.go` to see what the TUI loads and when
2. inspect `cxp/internal/tui/detail.go` to see whether access counts, children, triggers, and usage data already help with first-turn decisions
3. run `cxpv` and compare the actual surface with the proposed launch-pack artifact

## Stop Conditions

- if the TUI already pre-assembles enough context for common first turns:
  - treat it as an existing launch surface that future work should build on
- if it mostly exposes raw detail without task-specific routing:
  - treat it as adjacent but not yet equivalent to a launch pack

## Why This Should Beat The Launcher

- pre-assembled route:
  - it starts from the real integrated surface instead of separate CLI and doc paths
- earlier proof linkage:
  - it ties the evaluation directly to the code that controls what the user or agent sees
- earlier trust calibration:
  - it avoids missing existing access counts, child triggers, and detail rendering that may already influence launch quality
