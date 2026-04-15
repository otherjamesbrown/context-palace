# Launch Pack Evaluation Worksheet

## Evaluation Metadata

- repo:
  - Context Palace
- task:
  - Investigate why `cxp kb search` still requires knowledge_base.root configuration after `cxp init`
- task type:
  - bug investigation / behavior explanation
- evaluator:
  - Codex
- date:
  - 2026-03-18

## Expected Correct Working Area

- correct subsystem:
  - CLI config loading + KB command initialization
- nearest executable check:
  - `cxp init --project demo`, inspect `.cxp.yaml`, then run `cxp kb search "test"`

## Condition A: Strong Launcher

### Artifact Used

- file or prompt:
  - `01-kb-root-config-launcher.md`

### Observations

- first subsystem chosen:
  - CLI config loading + KB command initialization
- first executable check chosen:
  - build temporary `cxp` binary, run `init`, inspect generated `.cxp.yaml`, then run `kb search`
- wrong turns:
  - 1 significant wrong turn: assumed project-local config alone would explain the behavior, but global legacy config fallback from `~/.cp/config.yaml` masked the missing `knowledge_base.root`
- stale-prose detours:
  - 0
- irrelevant files/docs opened before correct working area:
  - 1 light detour into README-level framing before the code/config path was fully resolved
- time to first correct subsystem:
  - about 4 minutes
- time to first correct subsystem plus executable check:
  - about 12 minutes

### Stitching Burden

- did the agent have to stitch route, evidence, and warnings together itself?
  - yes
- notes:
  - The launcher identified the right files, but it did not pre-assemble the interaction between project config, global fallback config, and KB root enforcement. The evaluator had to discover that the legacy global config was satisfying `knowledge_base.root`, then rerun with `--config ./.cxp.yaml` to isolate project-local behavior.

## Condition B: Task Launch Pack

### Artifact Used

- file or prompt:
  - `01-kb-root-config-launch-pack.md`

### Observations

- first subsystem chosen:
  - CLI config loading + KB command initialization
- first executable check chosen:
  - inspect `init_cmd.go`, `client.go`, and `kb.go` first, then run `init`, inspect `.cxp.yaml`, and run `kb search` with `--config ./.cxp.yaml`
- wrong turns:
  - 0 meaningful wrong turns
- stale-prose detours:
  - 0
- irrelevant files/docs opened before correct working area:
  - 0
- time to first correct subsystem:
  - about 3 minutes
- time to first correct subsystem plus executable check:
  - about 7 minutes

### Stitching Burden

- did the agent have to stitch route, evidence, and warnings together itself?
  - partly
- notes:
  - The launch pack still required active reasoning, but it narrowed the first route correctly: compare what `init` writes, what `LoadConfig` can read, and what `kb.go` enforces. That made the global fallback question visible earlier and led naturally to the isolated `--config ./.cxp.yaml` repro.

## Comparison

- which reached the correct subsystem faster?
  - tie on subsystem; both got into config/KB code quickly
- which reached the executable check faster?
  - launch pack
- which caused fewer wrong turns?
  - launch pack
- which reduced stitching burden more?
  - launch pack
- was the launch pack materially better?
  - yes

## Qualitative Notes

- what helped most:
  - The launch pack made the three-file interaction explicit and tied it directly to a reproducible CLI check. The key win was surfacing the need to isolate project-local config instead of assuming the first observed CLI behavior reflected only `init`.
- what still failed:
  - The pack did not explicitly mention legacy global fallback config (`~/.cp/config.yaml`), so the evaluator still had to notice that from `client.go`.
- was the pack genuinely distinct from the launcher?
  - yes, modestly but meaningfully. The launcher suggested the right area; the pack assembled the first proof path and reduced the need to stitch the config interaction together manually.

## Decision

- outcome:
  - validate
- why:
  - For this task, the launch pack reached the decisive repro faster, reduced the main wrong turn, and made the “launcher is incomplete, not wrong” distinction concrete. The difference was not dramatic, but it was real and product-meaningful.
