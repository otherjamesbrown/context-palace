# Strong Launcher: Future `cxp context compile` Entrypoint

## Task

```text
Investigate where a future `cxp context compile` or launch-pack feature should hook into the current Context Palace CLI.
```

## Repo Rules And Source Of Truth

- rules:
  - prefer current CLI command structure and proposal docs together
- source-of-truth order:
  - current command surfaces
  - client capabilities and search/config surfaces
  - proposal docs for intended future behavior
- workflow constraints:
  - distinguish clearly between current implementation and proposed product direction

## Tiny Repo Map

- current context command:
  - `cxp/cmd/context.go`
  - likely relevant because this is the obvious command namespace
- CLI root and boot:
  - `cxp/cmd/root.go`
  - likely relevant because new command flow must fit existing CLI shape
- future proposal docs:
  - `docs/context-compiler.md`, `docs/evidence-aware-context-proposal.md`
  - likely relevant because they define the desired behavior

## Task Classification

- likely task type:
  - architecture / integration investigation
- likely subsystem candidates:
  - context command namespace
  - search and retrieval support
  - future proposal layering
- main ambiguity:
  - whether `context compile` belongs as a natural extension of current `context` commands or should be a distinct command family

## Likely Code Starting Points

- `cxp/cmd/context.go`
- `cxp/cmd/root.go`
- `docs/context-compiler.md`
- `docs/evidence-aware-context-proposal.md`

## Likely Tests Or Executable Checks

- inspect current `cxp context` subcommands
  - confirms how much “project context” means today
- inspect `cxp kb search` and related client search surfaces
  - confirms what retrieval capabilities already exist
- `go test ./...`
  - broad safety check if a command is later added

## Relevant Prose Fallback

- `README.md`
  - explains current product shape and retrieval order
- `docs/context-compiler.md`
  - explains intended compile behavior

## Warnings

- this task mixes current code and future design; do not treat proposal docs as implemented truth
- there may be no single existing integration point yet

## Expected First Moves

1. inspect `cxp/cmd/context.go`
2. inspect `cxp/cmd/root.go`
3. inspect `docs/context-compiler.md` and current retrieval-related command surfaces
