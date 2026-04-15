# Task Launch Pack: KB Root Config After Init

## Task

```text
Investigate why `cxp kb search` still requires knowledge_base.root configuration after `cxp init`.
```

## Primary Objective

- get to:
  - the first correct subsystem plus executable check for explaining whether this is a config split, onboarding gap, or implementation bug

## Why This Pack Exists

- dominant risk:
  - wrong first turn into general docs or KB concepts instead of the actual config-loading path
- why a thin launcher is incomplete here:
  - the relevant behavior spans three places at once: init output, config loading precedence, and KB command enforcement

## Source Of Truth

- proof surfaces:
  - `cxp/cmd/init_cmd.go`
  - `cxp/internal/client/client.go`
  - `cxp/cmd/kb.go`
- prose role:
  - only to confirm intended user-facing behavior or onboarding expectations

## First Credible Route

- likely subsystem:
  - CLI config and KB command initialization
- likely code entrypoints:
  - `cxp/cmd/init_cmd.go`
  - `cxp/internal/client/client.go`
  - `cxp/cmd/kb.go`
- nearest executable evidence:
  - run `cxp init --project demo`
  - inspect generated `.cxp.yaml`
  - run `cxp kb search "test"` and compare error path against loaded config expectations

## Disambiguation

- ambiguity:
  - is init incomplete, or is KB root intentionally global rather than project-local?
- how to disambiguate quickly:
  - compare what `init_cmd.go` writes with what `client.Config` can load and what `kb.go` requires

## Trust Guidance

- likely current:
  - `README.md` config precedence section
- use with caution:
  - broader KB architecture docs, because they explain intent but not this exact CLI behavior
- why:
  - the code path is small and directly inspectable

## Minimal Supporting Prose

- `README.md`
  - use only to confirm user-facing expectations around config precedence

## First Three Moves

1. inspect `cxp/cmd/init_cmd.go` to see exactly what `cxp init` writes
2. inspect `cxp/internal/client/client.go` to see what config fields can come from project config versus global config
3. inspect `cxp/cmd/kb.go` and then run `cxp init` plus `cxp kb search` to confirm the live behavior

## Stop Conditions

- if `init` intentionally writes only project and agent:
  - treat the issue as onboarding/product expectation, not necessarily as a code bug
- if project config could reasonably include `knowledge_base` but `init` omits it:
  - treat it as an init gap or UX gap

## Why This Should Beat The Launcher

- pre-assembled route:
  - it points immediately to the three-file interaction instead of leaving the agent to discover the dependency chain
- earlier proof linkage:
  - it ties the code path to the exact reproducible CLI behavior
- earlier trust calibration:
  - it prevents spending early time in architecture docs when the answer is mostly in config code
