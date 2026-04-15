# Strong Launcher: KB Root Config After Init

## Task

```text
Investigate why `cxp kb search` still requires knowledge_base.root configuration after `cxp init`.
```

## Repo Rules And Source Of Truth

- rules:
  - treat code and commands as the proof surface
  - use docs and README as supporting explanation, not final truth
- source-of-truth order:
  - command code and config loading
  - CLI help text and README
  - proposal docs and broader prose
- workflow constraints:
  - prefer understanding config precedence before changing behavior

## Tiny Repo Map

- CLI boot and config:
  - `cxp/cmd/root.go`, `cxp/internal/client/client.go`
  - likely relevant because config precedence and loading live here
- project initialization:
  - `cxp/cmd/init_cmd.go`
  - likely relevant because `cxp init` creates project config
- KB commands:
  - `cxp/cmd/kb.go`
  - likely relevant because the failure appears when running `kb search`

## Task Classification

- likely task type:
  - bug investigation / behavior explanation
- likely subsystem candidates:
  - config loading
  - project init
  - KB search command behavior
- main ambiguity:
  - whether the issue is a bug, an intended split between project and global config, or a missing onboarding/documentation step

## Likely Code Starting Points

- `cxp/cmd/init_cmd.go`
  - creates `.cxp.yaml` and determines what init writes
- `cxp/cmd/kb.go`
  - enforces `knowledge_base.root` requirement and error text
- `cxp/internal/client/client.go`
  - defines config structure and precedence, including `knowledge_base`

## Likely Tests Or Executable Checks

- `cxp init --project <name>`
  - confirms what project config actually contains
- `cxp kb search "anything"`
  - reproduces the KB root requirement path
- `go test ./...`
  - broad confidence check after any change, though not targeted

## Relevant Prose Fallback

- `README.md`
  - explains config precedence and KB usage at a high level
- `CLAUDE.md`
  - useful for maintainer context and operating assumptions

## Warnings

- the issue may be product behavior, not a code bug
- KB root currently appears to belong to richer global config, while `init` writes only thin project config

## Expected First Moves

1. read `cxp/cmd/kb.go`
2. read `cxp/cmd/init_cmd.go`
3. read `cxp/internal/client/client.go`
