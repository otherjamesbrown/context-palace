# Proposal: Task-Run Observation Events

## Purpose

This document defines the minimum observation events Context Palace should record for the first Postgres-backed evidence loop.

The goal is not full replay.

The goal is to capture a small number of high-value signals that tell us:

- whether the agent reached the right working area
- what became the real proof surface
- where the pack or launcher still failed

These events are designed to map directly onto the schema proposed in [task-run-evidence-postgres.md](/Users/james/github/otherjamesbrown/context-palace/docs/task-run-evidence-postgres.md).

## First-Version Principle

Record only events that are both:

- semantically important
- likely to improve future launch-pack assembly or shard metadata

Do **not** start by recording every file open, search query, or prompt turn.

That would create noise before we know what matters.

## MVP Event Set

The first version should write six events:

1. `task_run_started`
2. `correct_subsystem_reached`
3. `executable_proof_surface_found`
4. `dead_end`
5. `pack_missing_reference`
6. `task_run_completed`

This is enough to make the loop real without building a large telemetry system.

## Event Definitions

### 1. `task_run_started`

This event marks the creation of a run.

In practice:

- create a row in `task_runs`
- optionally also emit an observation if the system wants a consistent event stream

When it should fire:

- when an agent begins work on an active shard or task
- after the launcher or launch pack has been selected

Required fields:

- `project`
- `task_id` or `shard_id`
- `agent_runtime`
- `launch_artifact_type`
- `started_at`

Recommended `details` payload:

```json
{
  "task_prompt": "Investigate why digest behavior differs between search results, scheduled digests, and journal digest output.",
  "launch_artifact_ref": "compiled/launch-pack/feature/digest-routing-fix",
  "branch": "feature/digest-routing-fix"
}
```

Why it matters:

- it anchors all later observations
- it lets us compare launcher vs launch-pack runs

### 2. `correct_subsystem_reached`

This event records the moment the agent reaches the right working area.

This is one of the most important signals in the whole system.

When it should fire:

- when the agent has enough evidence that a subsystem, service, package, or route is the correct place to continue
- ideally only once per run unless the first belief was later contradicted

Required fields:

- `task_run_id`
- `subject_type`
- `subject_ref`

Recommended values:

- `subject_type`
  - `service`
  - `package`
  - `module`
  - `subsystem`
  - `file`

Example:

```json
{
  "subject_type": "package",
  "subject_ref": "pkg/digest",
  "role": "final_route",
  "details": {
    "reason": "behavior split visible in shared digest model",
    "task_phase": "initial_routing"
  }
}
```

Why it matters:

- it tells us whether the launcher or pack helped the agent stop guessing
- repeated patterns can improve future routing

### 3. `executable_proof_surface_found`

This event records the nearest executable evidence the agent actually used or identified as decisive.

When it should fire:

- when the agent identifies the test, command, fixture, migration, script, or repro path that best validates the task

Required fields:

- `task_run_id`
- `subject_type`
- `subject_ref`

Recommended `subject_type` values:

- `test`
- `command`
- `script`
- `migration`
- `repro`

Example:

```json
{
  "subject_type": "test",
  "subject_ref": "tests/e2e/digest_search_test.go",
  "role": "proof_surface",
  "details": {
    "proof_kind": "e2e_test",
    "reason": "closest comparative behavior check",
    "task_phase": "proof_selection"
  }
}
```

Why it matters:

- it teaches the system which proof surfaces actually matter
- it provides a concrete basis for future `references.tests` metadata

### 4. `dead_end`

This event records a plausible but unhelpful path.

When it should fire:

- when the agent spent meaningful effort on a file, shard, or subsystem that turned out not to be the right path
- only for consequential dead ends, not tiny detours

Required fields:

- `task_run_id`
- `subject_type`
- `subject_ref`

Recommended `details` payload:

```json
{
  "reason": "plausible digest search entrypoint, but behavior split was clearer in shared digest package",
  "task_phase": "initial_routing",
  "cost_hint": "moderate"
}
```

Why it matters:

- dead ends are one of the clearest signals that the launch artifact was incomplete
- repeated dead ends can improve disambiguation guidance

### 5. `pack_missing_reference`

This event records something that should probably have been in the launch pack or launcher but was not.

When it should fire:

- when the agent finds an important file, test, command, or shard that the artifact failed to surface

Required fields:

- `task_run_id`
- `subject_type`
- `subject_ref`

Recommended `details` payload:

```json
{
  "missing_from": "launch_pack",
  "suggested_field": "references.tests",
  "reason": "important digest proof surface not surfaced in initial pack"
}
```

Why it matters:

- this is the cleanest event for improving future pack assembly
- repeated occurrences can generate metadata proposals automatically

### 6. `task_run_completed`

This event marks the end of the run.

In practice:

- update `task_runs.status`
- store summary fields
- optionally emit a terminal observation for stream completeness

When it should fire:

- when the task ends, pauses, or reaches an evaluation checkpoint

Recommended fields:

- `finished_at`
- `status`
- `summary`

Recommended `details` payload:

```json
{
  "outcome": "completed",
  "summary": "Launch pack reduced routing ambiguity and led directly to comparative digest proof surfaces.",
  "useful_artifact": true
}
```

Why it matters:

- it closes the run
- it gives the maintenance job a point to begin aggregation

## Minimal Mapping To Postgres

The simplest implementation is:

- `task_run_started`
  - create `task_runs` row
- middle events
  - append to `task_run_observations`
- `task_run_completed`
  - update `task_runs` row and optionally append a final observation

That means the system can begin learning without a complex event bus.

## Event Shape

All non-start events should be easy to serialize through one common structure:

```json
{
  "task_run_id": "53be89e8-4d77-49a8-8f89-b28b6f3f2ca0",
  "observation_type": "executable_proof_surface_found",
  "subject_type": "test",
  "subject_ref": "tests/e2e/digest_search_test.go",
  "role": "proof_surface",
  "confidence": 0.92,
  "source": "agent_runtime",
  "details": {
    "reason": "closest comparative behavior check",
    "task_phase": "proof_selection"
  }
}
```

## Emission Rules

To keep the first version trustworthy:

1. emit fewer, higher-signal events
2. prefer explicit agent conclusions over inferred low-level telemetry
3. allow confidence values, but do not require fake precision
4. do not emit duplicate events unless the later event clearly corrects the earlier one
5. make the event source explicit

Recommended `source` values:

- `agent_runtime`
- `cxp_wrapper`
- `user_annotation`
- `background_worker`

## First Integration Points

The first places these events could be emitted are:

- a `cxp` wrapper that launches work on an active shard
- a compile-and-run flow such as `cxp context compile`
- a future TUI task-launch action
- evaluation harnesses used in current launch-pack experiments

This should not require deep product integration before the loop becomes useful.

## What We Can Derive Later

Once these events exist, the maintenance layer can start deriving:

- repeated proof-surface links
- repeated dead-end routes
- candidate file and test references
- drift flags when the live proof path differs from the shard's current references

That is enough to begin closing the loop from:

- task shard
- to launch pack
- to task run
- to metadata improvement

## Out Of Scope For Version One

Do not include these in the first event set:

- every file read
- every search query
- token counts
- prompt-level transcript fragments
- full command replay
- passive UI navigation noise

Those may be useful later, but they should not define the first version.

## Recommended Next Step

After this event spec, the next implementation step should be:

- add Postgres migrations for `task_runs`, `task_run_observations`, `metadata_patch_proposals`, and `drift_flags`

Then:

- add a very small writer API in `cxp`
- emit these events from one controlled entry path first
