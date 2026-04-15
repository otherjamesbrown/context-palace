# Critic Review — Round 2

## Round 1 Issues: Resolution Status

### 1. MVP definition — RESOLVED
The "Minimum Viable Pipeline (v1)" section is now concrete. The scope table (included vs excluded) draws a clear line. The v1 workflow is a single readable sequence. The build sequence is ordered with rationale. This was the highest-priority item and it is addressed well.

### 2. Trigger mechanism — RESOLVED
The trigger mechanism now has a concrete design: cron poller at 30s intervals, three specific watch conditions (new design, needs-review task, satisfied waiting_for), spawn logic via tmux, and duplicate avoidance via lock check. This is sufficient to build from.

### 3. Concurrency model (lock/TTL) — RESOLVED
Lock with TTL is specified: session ID in `locked_by`, 5-minute TTL, refresh during long operations, stale lock reclamation by poller. Optimistic locking with no central coordinator. Adequate for v1.

### 4. Phase 1 fast path — RESOLVED
Phase 1 now leads with the fast path (readiness check, implementability check, proceed). The full C/D/S loop is clearly a branch triggered by specific conditions. Structure is correct.

### 5. Crossover example — RESOLVED
The "notification pipeline enrichment" example (5 tasks, 3 dependency edges) is in the Problem section. It concretely illustrates why manual coordination becomes the bottleneck at 3+ tasks. Sufficient.

### 6. Iteration budget labeling — RESOLVED
Budgets are now labeled "Defaults pending data from early pipeline runs." The table header says "Default" not "Limit." Acknowledged as tunable.

### 7. Test diagnosis step — RESOLVED
Phase 4 now includes an explicit test diagnosis step with three evaluation questions (test expectations match spec? implementation matches spec but fails test? implementation diverges from spec?). This directly prevents the bad-test loop the synthesis identified.

### 8. Defer v2 items — RESOLVED
Split test authoring, A/B implementation, multi-model diversity enforcement, deploy/test state machine, and automated deploy are all in a clearly marked "Future Capabilities (v2+)" section at the end.

**Summary: All eight round 1 must-fix items are resolved.**

## Overall Judgment

The design is substantially improved. The v1 scope is clear, the architecture sections answer the critical "how" questions that were missing in round 1, and the vision/plan conflation is fixed. The document is now closer to buildable than aspirational.

However, the revision introduced new material (Monitoring & Visibility, Configuration: Skills and Triggers) that warrants scrutiny, and there are gaps in the build sequence that would block decomposition into tasks.

## New Concerns

### 1. The Skills/Triggers configuration model is premature for v1

The "Configuration: Skills and Triggers" section introduces a YAML-driven trigger definition language with structured watch conditions including nested field matching (`parent_has`, `children.all_status`, `metadata.pipeline.last_progress: { older_than: 30m }`). This is a query language. Building a generic YAML-to-query evaluator is a meaningful piece of engineering that is not accounted for in the build sequence.

For v1, the cron poller can use hardcoded Go functions that check the three specific conditions already identified (new design, needs-review task, waiting_for satisfied). The YAML trigger language is a v2 concern — it becomes valuable when there are multiple projects with different trigger patterns. With one pipeline running, hardcoded triggers are faster to build, easier to debug, and sufficient.

The skills folder concept (markdown files as process definitions) is sound and low-cost. Keep that. But decouple it from the YAML trigger engine.

**Risk:** If this stays in v1 scope, it becomes the single largest implementation item and delays the core pipeline by days.

### 2. Grafana is scope creep for v1

The Monitoring section proposes two deliverables: `cxp dashboard` (a CLI command) and a Grafana installation with ~6 panels. The CLI dashboard is useful and low-cost — it is just SQL queries rendered in the terminal. But installing Grafana on dev02, connecting it to postgres, and building 6 panels is a separate project.

The v1 scope table does not mention monitoring at all. Yet the section says "v1 build" and lists Grafana as item 2. This contradicts the MVP scope boundary.

**Recommendation:** `cxp dashboard` is v1. Grafana is v2 (or a separate initiative that runs in parallel, outside the pipeline build sequence).

### 3. Build sequence is missing items that are in scope

The build sequence has 8 items but the v1 scope table includes items not in the sequence:

- **Evidence + session logging to shards** is item 7 in the sequence but has no dependency or description of what "evidence" means structurally. Is this a new shard field? A child shard? An append to content? The "Agent evidence on completion" section in Phase 3 describes the data but not the storage model.
- **Ralph Loop <-> shard integration** (item 8) depends on the Ralph Loop understanding shard status transitions. What does this integration actually look like? Does the Ralph Loop plugin need modification? Is this a code change to the Claude Code plugin or a wrapper script?

These are not blockers to starting, but they are blockers to decomposing items 7-8 into tasks.

### 4. The "waiting_for" field is referenced but never defined

The trigger mechanism says the poller checks `waiting_for` on pipeline shards, and the triggers YAML references `pipeline.last_progress`. But the pipeline shard schema is never specified. What fields does a pipeline shard have? At minimum: `phase`, `locked_by`, `lock_expires`, `waiting_for`, `last_progress`. This should be defined explicitly — it is item 1 in the build sequence ("Pipeline shard type") and everything else depends on it.

### 5. The poller's "watch" for satisfied waiting_for is underspecified

The poller watches for "pipeline shards where the waiting_for trigger is now satisfied." But what does `waiting_for` contain? A shard ID and expected status? A list of shard IDs? An arbitrary condition? The trigger YAML suggests structured conditions, but the concurrency model section just says "check waiting_for field against current shard states." For a hardcoded v1 poller, this needs to be concrete: `waiting_for` is a list of `{shard_id, expected_status}` tuples, and the condition is satisfied when all referenced shards have reached their expected status. Or something else. But it must be defined.

### 6. M playbook is item 3 but its content is not scoped

The build sequence says item 3 is "M playbook (v1: decision trees for routing)." The document describes what the playbook should contain (decision trees, escalation criteria, phase transition rules) but this is the only build item that is a document, not code. How is this built? Is it a single markdown file? Multiple files? Who writes it — James, an agent, or is it derived from this design? The playbook is M's "brain" and its quality determines pipeline quality, but the design treats it as a line item rather than a first-class deliverable.

## Remaining Gaps

### Pipeline shard schema
The design needs a concrete field list for the pipeline shard type. This is foundational — every other component reads and writes it.

### Worktree lifecycle
Item 4 says "Worktree + PR automation per task." Is this a new `cxp` command? A shell script M runs? The "What Already Exists" table says worktree support is "Available (Claude Code built-in)" — so what is actually being built here? Clarify what is new code vs existing capability.

### Error handling in the poller
The poller runs every 30 seconds. What happens when: the database is unreachable? A tmux spawn fails? The lock write fails due to a race? These are operational concerns that do not need elaborate design, but the poller is the single point of failure for the entire system. A sentence or two on retry/backoff/alerting would be appropriate.

## Is This Ready for Decomposition?

**Almost, but not quite.** The design is ready for decomposition into tasks for build items 1-6. Items 7-8 need slightly more definition. The Skills/Triggers YAML engine should be explicitly descoped from v1, and the Grafana work should be moved out of the v1 build sequence.

Specifically, the design is ready for decomposition if these three changes are made:

1. Define the pipeline shard schema (fields, types, initial values)
2. Descope YAML trigger engine to v2 (keep hardcoded poller triggers for v1)
3. Move Grafana to v2 or parallel track

With those changes, the document supports breaking build items 1-6 into implementable tasks. Items 7-8 can be shaped in parallel while 1-6 are being built.

## Recommended Revisions

1. **Add pipeline shard schema.** Enumerate the fields: `phase`, `locked_by`, `lock_expires`, `waiting_for` (define structure), `last_progress`, `task_shards` (or equivalent reference to child tasks), `cumulative_tokens`. This is 10-15 lines and it unblocks everything.

2. **Descope YAML trigger engine to v2.** Replace the triggers.yaml section with a note that v1 uses hardcoded trigger logic in the poller for the three identified conditions. Keep the YAML design as v2 vision — it is good design, just premature.

3. **Move Grafana to v2 or separate track.** Keep `cxp dashboard` in v1. Grafana is independently valuable and can be built anytime, but it is not on the critical path for the pipeline and should not compete for build attention.

4. **Clarify what "Worktree + PR automation" (item 4) actually builds.** Is it a `cxp worktree create <task-shard-id>` command? A function M calls? Spell out the interface.

5. **Define the M playbook deliverable.** State what the v1 playbook contains (even as a bulleted outline), who creates it, and what format it takes. If it is a markdown file, say so and sketch its table of contents.

6. **Add one sentence on poller error handling.** Log and retry on transient failures, alert (how?) on persistent failures.
