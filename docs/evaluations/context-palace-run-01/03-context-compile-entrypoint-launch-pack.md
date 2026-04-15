# Task Launch Pack: Future `cxp context compile` Entrypoint

## Task

```text
Investigate where a future `cxp context compile` or launch-pack feature should hook into the current Context Palace CLI.
```

## Primary Objective

- get to:
  - the first correct subsystem plus executable evidence for where the future feature naturally fits in the current CLI and product shape

## Why This Pack Exists

- dominant risk:
  - wrong first turn into high-level proposal prose without checking the current command surface that already exists
- why a thin launcher is incomplete here:
  - the task needs both current code reality and future design intent, and the first turn should anchor in current CLI shape

## Source Of Truth

- proof surfaces:
  - `cxp/cmd/context.go`
  - `cxp/cmd/root.go`
  - existing search/navigation commands such as `cxp/cmd/kb.go`
- prose role:
  - define intended future behavior and product expectations only

## First Credible Route

- likely subsystem:
  - `context` command namespace plus current retrieval command surfaces
- likely code entrypoints:
  - `cxp/cmd/context.go`
  - `cxp/cmd/root.go`
  - `cxp/cmd/kb.go`
- nearest executable evidence:
  - inspect existing `cxp context` subcommands and CLI help shape
  - compare with `docs/context-compiler.md` proposed `cxp context compile` UX

## Disambiguation

- ambiguity:
  - is the right answer “extend `context`”, “compose existing retrieval commands”, or “introduce a more distinct command path later”?
- how to disambiguate quickly:
  - compare what `context` means today with the proposed compile behavior and what retrieval primitives already exist

## Trust Guidance

- likely current:
  - `cxp/cmd/context.go`, `cxp/cmd/root.go`, `README.md`
- use with caution:
  - `docs/context-compiler.md` and `docs/evidence-aware-context-proposal.md`
- why:
  - the proposal docs are design intent, not implementation

## Minimal Supporting Prose

- `docs/context-compiler.md`
  - to understand the proposed future command shape
- `docs/evidence-aware-context-proposal.md`
  - to understand the user problem the feature is supposed to solve

## First Three Moves

1. inspect `cxp/cmd/context.go` to understand the current meaning of the `context` namespace
2. inspect `cxp/cmd/root.go` and `cxp/cmd/kb.go` to see how retrieval-related commands are currently organized
3. only then compare that code reality with `docs/context-compiler.md` to decide the natural integration point

## Stop Conditions

- if the `context` namespace already represents project-level synthesized views:
  - treat `context compile` as a natural extension candidate
- if retrieval surfaces are too distributed:
  - treat the problem as needing a new integration layer rather than just one more subcommand

## Why This Should Beat The Launcher

- pre-assembled route:
  - it anchors the first turn in current command reality rather than future prose
- earlier proof linkage:
  - it identifies the exact current files and command surfaces that must constrain any future feature
- earlier trust calibration:
  - it prevents treating proposal docs as if they already define the implemented CLI architecture
