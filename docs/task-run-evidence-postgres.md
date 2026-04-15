# Proposal: Postgres Task-Run Evidence Layer

## Purpose

This document defines a simple Postgres-backed evidence layer for Context Palace.

Its job is to store:

- machine-generated task-run observations
- derived metadata patch proposals
- drift flags that need review

It should **not** store curated shard content itself.

The intended split is:

- shards and KB files:
  - durable human-reviewed knowledge
- Postgres evidence layer:
  - runtime observations and machine-generated suggestions

That keeps telemetry and curated knowledge separate.

## Why Postgres

Postgres is the best first home for this layer because task-run evidence is operational data:

- append-heavy
- query-heavy
- time-oriented
- cross-run by nature

We will want to ask questions like:

- which files keep becoming the real proof surface for tasks in this area?
- which tests are repeatedly the nearest executable evidence?
- which shards are often loaded but rarely useful?
- which warnings keep being confirmed?
- which shards are likely drifting because their references no longer match real work?

That kind of aggregation is awkward in Markdown files and unnecessary to model as shards.

## Design Principles

1. observations are facts about runs, not curated truth
2. proposals are derived from observations, not written by hand
3. drift flags are review signals, not automatic verdicts
4. shards should only change after review or an explicit auto-apply policy
5. the first schema should be small and append-friendly

## Lifecycle

The intended loop is:

1. a task shard becomes active
2. Context Palace compiles a launch pack
3. an agent works the task
4. runtime observations are written to Postgres
5. a maintenance job aggregates those observations
6. the maintenance job creates:
   - metadata patch proposals
   - drift flags
7. reviewed proposals update shard metadata

This means the task shard remains clean while the system still learns from real work.

## MVP Tables

The minimum useful schema is:

- `task_runs`
- `task_run_observations`
- `metadata_patch_proposals`
- `drift_flags`

### `task_runs`

One row per agent run on a task.

Suggested fields:

```sql
create table task_runs (
    id text primary key,
    project text not null,
    task_id text,
    shard_id text references shards(id) on delete set null,
    branch text,
    agent_runtime text not null,
    agent_name text,
    launch_artifact_type text not null,
    launch_artifact_ref text,
    status text not null,
    started_at timestamptz not null default now(),
    finished_at timestamptz,
    summary text,
    created_at timestamptz not null default now()
);
```

Notes:

- `id` is text so the first implementation can generate IDs in `cxp` without adding DB UUID extensions.
- `task_id` can refer to an external work item if one exists.
- `shard_id` is the main durable Context Palace linkage.
- `launch_artifact_type` should distinguish:
  - `none`
  - `launcher`
  - `launch_pack`
  - `manual`
- `launch_artifact_ref` can point to the compiled pack or prompt snapshot when useful.

### `task_run_observations`

One row per observed fact during a run.

Suggested fields:

```sql
create table task_run_observations (
    id text primary key,
    task_run_id text not null references task_runs(id) on delete cascade,
    observed_at timestamptz not null default now(),
    observation_type text not null,
    subject_type text not null,
    subject_ref text not null,
    role text,
    confidence numeric(4,3),
    source text not null,
    details jsonb not null default '{}'::jsonb
);

create index idx_task_run_observations_run
    on task_run_observations(task_run_id);

create index idx_task_run_observations_subject
    on task_run_observations(subject_type, subject_ref);

create index idx_task_run_observations_type
    on task_run_observations(observation_type);
```

Recommended `observation_type` values:

- `file_opened`
- `test_opened`
- `test_used_as_proof`
- `shard_loaded`
- `shard_useful`
- `shard_not_useful`
- `warning_confirmed`
- `warning_contradicted`
- `dead_end`
- `correct_subsystem_reached`
- `executable_proof_surface_found`
- `pack_missing_reference`

Recommended `subject_type` values:

- `file`
- `test`
- `shard`
- `warning`
- `service`
- `command`
- `migration`

Recommended `role` values:

- `candidate`
- `proof_surface`
- `dead_end`
- `supporting_context`
- `warning_target`
- `final_route`

`details` should hold flexible run-specific context, for example:

```json
{
  "path": "pkg/digest/gather.go",
  "reason": "shared digest behavior",
  "opened_order": 3,
  "task_phase": "initial_routing"
}
```

### `metadata_patch_proposals`

Machine-generated suggestions for durable shard metadata updates.

Suggested fields:

```sql
create table metadata_patch_proposals (
    id text primary key,
    shard_id text not null references shards(id) on delete cascade,
    proposal_type text not null,
    target_field text not null,
    target_ref text not null,
    evidence_count integer not null default 1,
    status text not null default 'proposed',
    rationale text not null,
    payload jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    reviewed_at timestamptz,
    reviewed_by text
);

create index idx_metadata_patch_proposals_shard
    on metadata_patch_proposals(shard_id);

create index idx_metadata_patch_proposals_status
    on metadata_patch_proposals(status);
```

Recommended `proposal_type` values:

- `add_reference`
- `remove_reference`
- `promote_reference`
- `add_test_link`
- `mark_stability`
- `suggest_related_shard`

Example targets:

- `references.files`
- `references.tests`
- `references.services`
- `stability`

Suggested `status` values:

- `proposed`
- `accepted`
- `rejected`
- `applied`

### `drift_flags`

Review items for likely stale or risky shard context.

Suggested fields:

```sql
create table drift_flags (
    id text primary key,
    shard_id text not null references shards(id) on delete cascade,
    severity text not null,
    flag_type text not null,
    status text not null default 'open',
    rationale text not null,
    evidence jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    reviewed_at timestamptz,
    reviewed_by text
);

create index idx_drift_flags_shard
    on drift_flags(shard_id);

create index idx_drift_flags_status
    on drift_flags(status);
```

Recommended `flag_type` values:

- `referenced_file_changed`
- `referenced_test_removed`
- `path_missing`
- `high_runtime_divergence`
- `doc_detail_drift`

Recommended `severity` values:

- `low`
- `medium`
- `high`

Recommended `status` values:

- `open`
- `accepted`
- `dismissed`
- `resolved`

## Example Run

Imagine:

- shard:
  - `feature/digest-routing-fix`
- project:
  - `penfold`
- runtime:
  - `claude_code`
- launch artifact:
  - `launch_pack`

Example `task_runs` row:

```text
id: tr-1742420000000-3f7bc0b677f0ab12
project: penfold
task_id: feature/digest-routing-fix
shard_id: feature/digest-routing-fix
agent_runtime: claude_code
launch_artifact_type: launch_pack
status: completed
```

Example `task_run_observations` rows:

```text
1. observation_type: test_used_as_proof
   subject_type: test
   subject_ref: tests/e2e/digest_search_test.go
   role: proof_surface

2. observation_type: file_opened
   subject_type: file
   subject_ref: pkg/digest/gather.go
   role: final_route

3. observation_type: dead_end
   subject_type: file
   subject_ref: services/gateway/searchservice/service.go
   role: dead_end

4. observation_type: pack_missing_reference
   subject_type: test
   subject_ref: tests/e2e/weekly_digest_test.go
   role: supporting_context
```

Example `metadata_patch_proposals` row:

```text
shard_id: feature/digest-routing-fix
proposal_type: add_test_link
target_field: references.tests
target_ref: tests/e2e/digest_search_test.go
evidence_count: 3
rationale: Repeatedly used as nearest proof surface across digest routing runs
status: proposed
```

Example `drift_flags` row:

```text
shard_id: kb/digest-architecture
severity: medium
flag_type: referenced_file_changed
status: open
rationale: Linked digest workflow files changed after shard update and recent runs are using different proof surfaces
```

## How Context Palace Would Use This

### At Task Start

When a shard becomes active:

- create a `task_runs` record
- record the runtime and launch artifact type
- compile the launch pack

### During The Run

As the agent works:

- append `task_run_observations`
- prefer lightweight event writes over large snapshots
- record decisive observations rather than every keystroke

The goal is not complete replay. The goal is useful learning.

### After The Run

At completion:

- mark the run complete
- attach a short summary
- queue aggregation

### In Background Maintenance

A maintenance worker can:

- aggregate repeated useful files and tests
- identify dead-end candidates
- generate patch proposals
- generate drift flags
- improve future launch-pack ranking and assembly

## What Should Not Happen

This schema is intentionally designed to prevent a few failure modes:

- raw observations should not be appended directly into shard bodies
- every file open should not become permanent metadata
- drift flags should not silently rewrite curated knowledge
- one noisy run should not dominate future pack assembly

The evidence layer should be:

- queryable
- auditable
- reversible
- separate from curated knowledge

## MVP Implementation Guidance

For an initial implementation:

1. create the four tables above
2. write only a small set of observation types
3. record only high-value events:
   - correct subsystem reached
   - proof surface found
   - dead end
   - warning confirmed or contradicted
   - missing pack reference
4. generate only two kinds of derived outputs:
   - `add_reference`
   - `drift_flags`
5. keep shard mutation manual or review-gated

That is enough to make the loop real without building a large autonomous system too early.

## Open Questions

Important design questions still remain:

- should runs be scoped to a single agent session or a completed shard state transition?
- should observation capture happen directly in `cxp`, in wrappers around agent runtimes, or both?
- when should repeated observations auto-promote into pack assembly hints?
- what confidence threshold should be required before generating a metadata proposal?
- how much of this should be user-visible in the TUI?

Those are important, but they should not block the basic evidence-layer split.
