# Synthesis Review — Round 2

## Current Core Thesis

M Pipeline automates multi-agent design-to-delivery by using shard state as the single source of truth and ephemeral orchestrator sessions (M) that hydrate from pipeline shards, take one action, update state, and exit. The v1 scope is now clearly defined: decompose, dispatch with dependency ordering, implement, review, merge — with C/D/S automation, split testing, A/B implementation, and automated deploy explicitly deferred.

## Round 1 → Round 2 Progress

Round 1 identified 8 must-fix items. All 8 are resolved:

| # | Round 1 Item | Status |
|---|---|---|
| 1 | MVP definition | Resolved — v1 scope table, workflow, build sequence all present |
| 2 | Trigger mechanism design | Resolved — cron poller, 3 watch conditions, spawn logic, duplicate avoidance |
| 3 | Concurrency model | Resolved — lock/TTL, session ID, 5-min expiry, stale lock reclamation |
| 4 | Phase 1 fast path | Resolved — fast path is default, C/D/S is the exception |
| 5 | Crossover example | Resolved — notification pipeline enrichment (5 tasks, 3 edges) |
| 6 | Iteration budget labeling | Resolved — labeled "defaults pending data" |
| 7 | Test diagnosis step | Resolved — 3-question evaluation in Phase 4 |
| 8 | Defer v2 items | Resolved — Future Capabilities section clearly marked |

The revision also added two new sections: Monitoring & Visibility, and Configuration: Skills and Triggers. These are the source of all round 2 debate.

## Resolution of Round 2 Debates

### Debate 1: YAML Triggers — Over-engineered vs. Same Work as Config

**Critic position:** The triggers YAML is a query language in disguise. Hardcoded Go functions for the 3 known conditions are faster to build and easier to debug. Descope to v2.

**Defender position:** Hardcoded Go implements the same matching logic but in compiled code instead of a config file. The YAML is not a generic query engine — it is a fixed-schema config with 6 known field types. Iteration cost favors config over code during early tuning.

**Resolution: The defender wins the architectural argument but the critic wins the scoping argument. Split the difference.**

The defender is correct that the v1 trigger file is not a generic query language — it has 5 entries with a small, fixed vocabulary. The critic is wrong to call this "the single largest implementation item." But the critic is right that there is a meaningful difference between parsing a YAML config with nested conditions (`parent_has`, `children.all_status`, `older_than`) and writing 3 Go functions. The YAML approach requires: a parser, a condition evaluator for each field type, and error handling for malformed configs. The Go approach requires: 3 functions and a switch statement.

**Decision:** v1 uses hardcoded trigger logic in the poller for the 3 identified conditions. The triggers YAML design stays in the document as the v2 approach, with a note that v1 trigger logic is structured to make the v2 YAML migration straightforward (i.e., each hardcoded trigger maps to one future YAML entry). The skills folder (markdown process files) remains in v1 — skills and triggers are not the same thing. Skills are read by agents as instructions; triggers require an evaluator engine.

**Rationale:** The pipeline is unproven. During the first weeks, the trigger conditions themselves may change in ways that require code changes regardless (new shard field, different relationship check). Hardcoded triggers let the team iterate on what the right conditions are before locking down how they are expressed. Once the conditions stabilize, YAML extraction is a clean refactor.

### Debate 2: Grafana — Scope Creep vs. Zero-Cost Visibility

**Critic position:** Grafana is a separate project not mentioned in the v1 scope table. It competes for build attention and contradicts the MVP boundary.

**Defender position:** Grafana is already running on dev02. The work is 6 SQL queries pasted into a UI — less effort than the CLI dashboard command. Running an automated pipeline without time-series visibility is operationally reckless.

**Resolution: The defender wins. Grafana stays in v1.**

The critic's concern about scope consistency is valid — the v1 scope table should mention monitoring. But the conclusion (move to v2) does not follow. The effort for Grafana panels when the infrastructure is already running is genuinely minimal — SQL queries against a known schema, no application code, no deployment. The critic's framing as "a separate project" mischaracterizes the actual work.

More importantly, the defender's operational argument is strong: M dispatches agents, manages locks, and merges PRs autonomously. Running this without time-series visibility and alerting is a legitimate operational risk. The CLI dashboard gives a snapshot; Grafana gives history, trends, and alerts.

**Decision:** Both `cxp dashboard` and Grafana panels are v1 scope. Add monitoring to the v1 scope table. Note that Grafana is already running on dev02 and the deliverable is SQL queries only. Grafana panels can be built in parallel with items 1-3 since they are independent of the pipeline code.

### Concern 3: Build Sequence Items 7-8 (Evidence, Ralph Loop Integration)

**Resolution: Add brief definitions, but do not block decomposition.**

The critic is right that the storage model for evidence and the Ralph Loop integration mechanism should be stated. The defender is right that full specification is premature for items at the end of the build sequence. Add one sentence each:

- Evidence: appended to shard content (not a separate shard type or new field).
- Ralph Loop integration: completion promise checks shard status via `cxp shard status`; on task completion, Ralph Loop calls `cxp shard status <id> needs-review`.

### Concern 4: Pipeline Shard Schema

**Resolution: Must be added. Both sides agree.** This is foundational — every other component reads and writes pipeline shard fields. Enumerate: `phase`, `locked_by`, `lock_expires`, `waiting_for`, `last_progress`, `task_shards`, `cumulative_tokens`.

### Concern 5: `waiting_for` Structure

**Resolution: Must be defined. Both sides agree.** List of `{shard_id, expected_status}` pairs. Condition satisfied when all entries match. 2-3 sentences.

### Concern 6: M Playbook Scoping

**Resolution: Add one line.** "v1 playbook is a single markdown file containing decision trees derived from the phase rules in this design." The critic's request for a table of contents is a decomposition artifact, not a design concern.

### Remaining Gaps (Worktree Lifecycle, Poller Error Handling)

- **Worktree lifecycle:** The design is sufficient. Whether this is a `cxp` command or a function in the M skill is a decomposition decision. The design says what needs to happen (worktree + feature branch per task); the decomposer decides how.
- **Poller error handling:** Add one sentence: "Transient failures (DB unreachable, tmux spawn failure) are retried on the next 30-second cycle; the lock TTL prevents orphaned state from failed spawns."

## Remaining Must-Fix Items

Three items must be addressed before decomposition. All are small additions (collectively under 30 lines):

| # | Item | Effort | Why It Blocks |
|---|---|---|---|
| 1 | Pipeline shard schema (field enumeration with types) | 10-15 lines | Build item 1; everything reads/writes it |
| 2 | `waiting_for` structure definition | 2-3 sentences | Poller logic and pipeline state transitions depend on it |
| 3 | Descope YAML triggers to v2, state v1 uses hardcoded poller | Rewrite ~1 paragraph | Prevents overbuilding the trigger mechanism |

## Is This Ready for Decomposition?

**Yes, after the 3 must-fix items above.** These are editorial changes, not structural. The design does not need another C/D/S round.

The design is ready for decomposition because:

- The v1 scope is clearly bounded (included vs excluded table, build sequence with rationale)
- The architecture answers the critical "how" questions (ephemeral M, shard-as-state, lock/TTL, cron poller)
- All 5 phases have concrete sub-phase sequences with enough detail for a decomposer
- The concurrency model is specified
- Cross-cutting concerns (budgets, token tracking, traceability) are addressed
- v2+ items are cleanly separated

The 3 must-fix items are concrete, small, and unambiguous. They can be made in a single editing pass (30 minutes), after which the design goes directly to Phase 2 decomposition.

## Final Revision Agenda

Apply in a single pass. Estimated effort: 30 minutes of editing.

**Must-fix (blocking decomposition):**

1. **Add pipeline shard schema.** Insert a field table after the "M's Hydration Payload" section: `phase` (string), `locked_by` (string, nullable), `lock_expires` (timestamp, nullable), `waiting_for` (list of `{shard_id, expected_status}` pairs), `last_progress` (timestamp), `task_shards` (list of shard IDs), `cumulative_tokens` (integer). Include initial values.

2. **Define `waiting_for` semantics.** In the trigger mechanism section, add: "`waiting_for` is a list of `{shard_id, expected_status}` pairs. The condition is satisfied when every referenced shard has reached its expected status. The poller evaluates this by querying current status for each referenced shard."

3. **Descope YAML triggers to v2.** Replace the triggers YAML section content with: v1 uses hardcoded trigger logic in the poller for 3 conditions (new design, task needs-review, waiting_for satisfied). Keep the YAML trigger design as the v2 approach — note that v1 trigger functions are structured to map 1:1 to future YAML entries. Keep the skills folder section unchanged (skills are v1).

**Should-fix (strengthen but do not block):**

4. **Add monitoring to the v1 scope table.** Add row: "`cxp dashboard` + Grafana panels" in Included column, "Grafana alerting rules" in Excluded column.

5. **Add evidence storage note.** In Phase 3, after "Agent evidence on completion," add: "Evidence is appended to the task shard content field. No separate shard type or new metadata field."

6. **Add Ralph Loop integration note.** In build item 8, add: "Ralph Loop completion promise checks shard status via `cxp shard status`. On task completion, the loop calls `cxp shard status <id> needs-review`."

7. **Add playbook format note.** In build item 3, add: "v1 playbook is a single markdown file containing decision trees derived from the phase rules in this design."

8. **Add poller resilience note.** In the trigger mechanism section, add: "Transient failures are retried on the next 30-second cycle. The lock TTL prevents orphaned state from failed spawns."

**Do not change:**

- The ephemeral M architecture
- The five-phase structure
- The v1 scope boundary (except adding monitoring)
- The non-goals section
- The build sequence ordering
- The skills folder design

After this revision, the design proceeds to Phase 2 decomposition. No further C/D/S rounds are needed.
