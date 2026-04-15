# Context Palace Task Runs

When you work on a shard-backed task, record a lightweight task run in Context Palace so future agents can learn which context was actually useful.

Use `cxp run` like this:

## 1. Start the run

At the start of meaningful work on a shard:

```bash
cxp run start <shard-id> --runtime <runtime> --artifact-type <none|launcher|launch_pack|manual>
```

Example:

```bash
cxp run start pf-123 --runtime claude_code --artifact-type launch_pack
```

This returns a run ID such as `tr-...`. Keep it and use it for the rest of the task.

## 2. Record only high-signal observations

Do not log every file read or every search. Only record observations that would help future task launch packs.

Supported observation types:

- `correct_subsystem_reached`
- `executable_proof_surface_found`
- `dead_end`
- `pack_missing_reference`

Examples:

```bash
cxp run observe <run-id> correct_subsystem_reached package pkg/digest --role final_route --detail '{"reason":"shared digest behavior"}'
cxp run observe <run-id> executable_proof_surface_found test tests/e2e/digest_search_test.go --role proof_surface
cxp run observe <run-id> dead_end file services/gateway/searchservice/service.go --detail '{"reason":"plausible entrypoint but not the decisive path"}'
cxp run observe <run-id> pack_missing_reference test tests/e2e/weekly_digest_test.go --detail '{"suggested_field":"references.tests"}'
```

Record observations when:

- you identify the first correct subsystem for the task
- you identify the nearest executable proof surface
- you hit a meaningful dead end that could mislead future agents
- you discover an important file, test, or shard that the launcher or launch pack should have included but missed

## 3. Complete the run

When the task pass is done:

```bash
cxp run complete <run-id> --summary "Reached the correct subsystem and identified the nearest executable proof surface"
```

## 4. Inspect if needed

To review recorded evidence:

```bash
cxp run show <run-id>
cxp run list <shard-id>
```

## Rules

- Use this for shard-backed work.
- Keep it lightweight.
- Prefer a few high-signal observations over noisy logging.
- Do not dump raw runtime notes into the shard itself.
- The shard remains curated knowledge.
- The task run is the evidence layer for what actually happened during execution.
