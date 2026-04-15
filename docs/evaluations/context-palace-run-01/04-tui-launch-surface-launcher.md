# Strong Launcher: TUI Launch Surface

## Task

```text
Investigate how the TUI currently surfaces knowledge hierarchy, access counts, and shard detail, and whether it already behaves like a launch surface for short tasks.
```

## Repo Rules And Source Of Truth

- rules:
  - treat current TUI code as the source of truth for what the TUI actually exposes
- source-of-truth order:
  - `cxpv` entrypoint and TUI implementation
  - client calls used by the TUI
  - README and broader docs for conceptual framing
- workflow constraints:
  - do not assume the CLI is the only important surface

## Tiny Repo Map

- TUI entrypoint:
  - `cxp/cmd/cxpv/main.go`
  - relevant because it defines how the browser is launched
- TUI model and load behavior:
  - `cxp/internal/tui/model.go`
  - relevant because it loads board, work, KB tree, messages, and details
- TUI detail rendering:
  - `cxp/internal/tui/detail.go`
  - relevant because it shows access counts, child triggers, and detail data
- TUI tree structure:
  - `cxp/internal/tui/tree.go`
  - relevant because it shapes how knowledge and work are navigated

## Task Classification

- likely task type:
  - investigation / product-surface understanding
- likely subsystem candidates:
  - TUI browse model
  - TUI detail pane
  - KB display behavior inside the TUI
- main ambiguity:
  - whether the TUI is just a browser or already a practical launch surface for the first correct task turn

## Likely Code Starting Points

- `cxp/cmd/cxpv/main.go`
- `cxp/internal/tui/model.go`
- `cxp/internal/tui/detail.go`
- `cxp/internal/tui/tree.go`

## Likely Tests Or Executable Checks

- run `cxpv`
  - verify what tabs and detail surfaces appear
- inspect the load commands in `model.go`
  - confirm what data sources are preloaded
- inspect detail rendering in `detail.go`
  - confirm whether access counts, children, triggers, and logs are visible enough to support launch decisions

## Relevant Prose Fallback

- `README.md`
  - confirms the intended KB/tree navigation model
- proposal docs in `docs/`
  - useful only to compare current product surface with future ideas

## Warnings

- it is easy to overfocus on CLI commands and miss that the TUI may already bundle information in a more launch-like way
- the TUI may be surfacing raw shard detail rather than an explicit launch pack, so the distinction matters

## Expected First Moves

1. inspect `cxp/cmd/cxpv/main.go`
2. inspect `cxp/internal/tui/model.go`
3. inspect `cxp/internal/tui/detail.go` and `cxp/internal/tui/tree.go`
