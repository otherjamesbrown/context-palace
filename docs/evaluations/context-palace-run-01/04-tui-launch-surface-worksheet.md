# Launch Pack Evaluation Worksheet

## Evaluation Metadata

- repo:
  - Context Palace
- task:
  - Investigate how the TUI currently surfaces knowledge hierarchy, access counts, and shard detail, and whether it already behaves like a launch surface for short tasks
- task type:
  - investigation / product-surface understanding
- evaluator:
  - Codex
- date:
  - 2026-03-18

## Expected Correct Working Area

- correct subsystem:
  - TUI browse model and detail rendering
- nearest executable check:
  - run `cxpv` and trace `loadKBTree` / `loadDetail` / `RenderDetail`

## Condition A: Strong Launcher

### Artifact Used

- file or prompt:
  - `04-tui-launch-surface-launcher.md`

### Observations

- first subsystem chosen:
  - TUI browse model and detail rendering
- first executable check chosen:
  - inspect `cxp/cmd/cxpv/main.go`, then trace `loadKBTree`, `loadDetail`, and `RenderDetail`
- wrong turns:
  - 1 moderate wrong turn: the launcher approach initially treated the TUI as just another surface to inspect rather than as a potentially integrated launch surface in its own right
- stale-prose detours:
  - 0
- irrelevant files/docs opened before correct working area:
  - 1 light detour into README/product framing before the TUI load path became the obvious proof surface
- time to first correct subsystem:
  - about 4 minutes
- time to first correct subsystem plus executable check:
  - about 9 minutes

### Stitching Burden

- did the agent have to stitch route, evidence, and warnings together itself?
  - yes
- notes:
  - The launcher identified the right files, but it did not foreground the key product question: whether the TUI already bundles enough route and evidence to behave like a launch surface. The evaluator still had to assemble that interpretation manually from the model, detail, and tree files.

## Condition B: Task Launch Pack

### Artifact Used

- file or prompt:
  - `04-tui-launch-surface-launch-pack.md`

### Observations

- first subsystem chosen:
  - TUI browse model and detail rendering
- first executable check chosen:
  - inspect `model.go` first, then `detail.go`, then compare with `tree.go` and `cxpv/main.go`
- wrong turns:
  - 0 meaningful wrong turns
- stale-prose detours:
  - 0
- irrelevant files/docs opened before correct working area:
  - 0
- time to first correct subsystem:
  - about 2 minutes
- time to first correct subsystem plus executable check:
  - about 6 minutes

### Stitching Burden

- did the agent have to stitch route, evidence, and warnings together itself?
  - partly
- notes:
  - The launch pack made the central question explicit up front: does the TUI already function as a launch surface? That led directly to the important proof path: `loadKBTree` auto-discovers KB roots, `loadDetail` shells out to `cxp shard show`, and `RenderDetail` exposes knowledge children, triggers, access counts, and recent access. Some interpretation was still required, but much less stitching was needed.

## Comparison

- which reached the correct subsystem faster?
  - launch pack
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
  - The launch pack forced the evaluation to treat the TUI as an integrated product surface rather than just another implementation area. That surfaced the most important finding quickly: `cxpv` is not yet a task-specific launch pack, but it already behaves like a proto-launch surface because it bundles navigation, detail, KB children, triggers, and access telemetry in one place.
- what still failed:
  - The evaluation did not include a live `cxpv` session because the code path inspection was already sufficient to answer the structural question. A future pass should observe actual interactive use as well.
- was the pack genuinely distinct from the launcher?
  - yes. The launcher found the TUI files; the pack made the product-level interpretation of those files explicit and faster to validate.

## Decision

- outcome:
  - validate
- why:
  - For this task, the launch pack more quickly answered the question that matters for agent usefulness: the TUI already packages several launch-relevant signals and should not be treated as an afterthought. The pack reduced wrong framing and stitching burden enough to count as a meaningful win.
