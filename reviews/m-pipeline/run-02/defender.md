# Defender Review — Round 2

## Overall Response

The critic confirms all 8 round-1 issues are resolved, and the design is now "closer to buildable than aspirational." That is a significant upgrade in assessment. The 6 new concerns are a mix of valid refinements, scope-policing overreach, and implementation-planning standards being applied to a design document. Three deserve revision; three do not.

The central question this round is: **is the critic helping finish a design, or starting to write the implementation plan?** A design document's job is to give a decomposer enough to break the work into tasks. It is not a task specification itself. Several of the critic's "gaps" are decomposition-phase details that belong in Phase 2, not in this document.

## Round 1 Resolution Acknowledgment

All 8 round-1 items resolved. The critic confirms this cleanly and without qualification. No further discussion needed on any round-1 item.

## Criticisms That Are Correct

### 5. The `waiting_for` field needs a concrete definition

This is the strongest concern in the round. The document references `waiting_for` in the trigger mechanism and concurrency model but never defines its structure. The critic is right: is it a list of `{shard_id, expected_status}` tuples? A single shard reference? An arbitrary predicate?

For v1, the answer is simple and should be stated: `waiting_for` is a list of `{shard_id, expected_status}` pairs, and the condition is satisfied when every referenced shard has reached its expected status. This is 2-3 sentences and it unblocks both the poller design and the pipeline shard schema. Concede fully.

### 4. The pipeline shard schema should be enumerated

The critic asks for the field list: `phase`, `locked_by`, `lock_expires`, `waiting_for`, `last_progress`, etc. This is fair. The design says "Pipeline shard type" is build item 1 and everything depends on it, but the document never shows what that type contains. A 10-15 line field table would close this gap and costs nothing. Concede.

## Criticisms That Are Partly Correct but Overstated

### 3. Build sequence items 7-8 need more definition

The critic says evidence logging and Ralph Loop integration are underspecified and would block decomposition into tasks. This is partly true — a sentence or two on the storage model for evidence (appended to shard content, not a new shard type) and the Ralph Loop integration mechanism (completion promise checks shard status via `cxp shard status`) would help.

But the critic's framing — that these are "blockers to decomposing items 7-8 into tasks" — overstates the urgency. Items 7-8 are last in the build sequence. Items 1-6 will take weeks. There is ample time to define 7-8 in more detail before they become actionable. A brief note is warranted; a full specification is premature. Add a sentence each, but do not treat this as a blocker to declaring the design ready.

### 6. M playbook content is not scoped

The critic asks: who writes the playbook, what format is it, what does it contain? Fair questions, but the critic answers them in the asking. The design already says it contains decision trees for routing, escalation criteria, and phase transition rules. It is a markdown file (the skills folder section makes this obvious). It is derived from this design document (the decision trees are described in the phase sections). Who writes it? The agent who decomposes build item 3.

The critic wants a table of contents for the playbook. That is a Phase 2 decomposition artifact. The design gives enough information for a decomposer to produce that table of contents. Adding a one-line note ("v1 playbook is a single markdown file derived from the phase rules in this design") is reasonable. Writing the outline is the decomposer's job.

## Criticisms That Are Unconvincing or Misframed

### 1. The YAML trigger engine is NOT premature for v1

This is the critic's headline concern and it is wrong. The critic frames the choice as "YAML trigger engine vs. hardcoded Go functions." But look at what the hardcoded poller needs to do:

- Watch for shards matching specific type + status + metadata conditions
- Check parent/child relationships between shards
- Evaluate temporal conditions (last_progress older than N minutes)
- Map matched conditions to specific skill files and spawn actions

If you hardcode this in Go, you are building the same logic — the same field matching, the same relationship traversal, the same spawn dispatch — but embedding it in compiled code instead of a config file. The YAML is not a "query language engine." It is a structured config file that the poller reads to know what to check. The alternative is not simpler; it is the same complexity with worse ergonomics.

The critic says "hardcoded triggers are faster to build." This is only true if you ignore iteration cost. Every time a trigger condition changes — and they will change constantly during early pipeline runs — hardcoded means recompile and redeploy. YAML means edit a file. For a system that is explicitly designed to be tuned from real pipeline runs, the config-file approach is cheaper across the first month of use, not more expensive.

The skills folder and the triggers file are the same design pattern: process-as-configuration. The critic endorses the skills folder but rejects the triggers file. This is inconsistent. Both are declarative definitions that M reads. Both avoid code changes for process changes. Accepting one while rejecting the other draws an arbitrary line.

The real risk the critic is gesturing at — that building a fully generic condition evaluator is over-engineering — is valid in the abstract but not applicable here. The v1 trigger file has 5 entries with a small, fixed vocabulary of conditions (`type`, `status`, `metadata.*`, `parent_has`, `children.all_status`, `older_than`). This is not a generic query language. It is a fixed-schema config parser. Any Go developer can implement the evaluator for these 6 field types in an afternoon. It is not "the single largest implementation item."

**Recommendation:** Keep the triggers YAML in v1. Add a note that the v1 evaluator supports a fixed vocabulary of condition types (not arbitrary expressions) and that the YAML schema may expand in v2.

### 2. Grafana is NOT scope creep

The critic says Grafana is "a separate project" that "contradicts the MVP scope boundary." This significantly overstates the effort and misunderstands the operational context.

Grafana is already installed and running on dev02. The postgres connection to the contextpalace database is a single connection string. Building 6 panels is 6 SQL queries pasted into the Grafana UI. There is no application code. There is no deployment. There is no infrastructure to provision.

The actual work is: write 6 SQL queries, paste them into Grafana panel configs, arrange a dashboard. This is 1-2 hours of work for someone who knows the schema, which the person building the pipeline shard type (item 1) will. It is less work than writing the `cxp dashboard` CLI command, which requires Go code, terminal formatting, and argument parsing.

The critic says monitoring is "not in the v1 scope table." This is a fair observation about document consistency — the scope table should mention it. But the conclusion ("move to v2") does not follow from the premise ("not listed in the table"). The fix is to add it to the table, not to cut useful near-zero-cost work.

More importantly: the pipeline is an automated system that dispatches agents, manages locks, and merges PRs. Running it without visibility into what it is doing is operationally reckless. `cxp dashboard` provides a snapshot; Grafana provides time-series visibility and alerting. Both are needed before trusting the pipeline with real work. This is not "competing for build attention." It is basic operational hygiene that happens to be nearly free because the infrastructure already exists.

**Recommendation:** Keep Grafana in v1. Add monitoring to the v1 scope table for consistency. Note that Grafana is already running on dev02 and the work is SQL queries only, no infrastructure.

### Remaining Gaps (worktree lifecycle, poller error handling)

The critic raises these under "Remaining Gaps" rather than as numbered concerns, appropriately signaling they are lower priority. Brief responses:

- **Worktree lifecycle:** The design says worktree support is a Claude Code built-in. Item 4 is the automation wrapper — M creating the worktree and feature branch for each task before spawning the agent. Whether this is a `cxp` command or a function in the M skill is a decomposition detail, not a design gap.

- **Poller error handling:** A sentence on retry/backoff is reasonable. But the poller is a 30-second cron loop; transient failures resolve themselves on the next cycle. The lock TTL handles the case where a spawn succeeds but the spawned M dies. This is inherently self-healing by design. Adding a note is fine; treating it as a gap is overstating it.

## Best Next Revisions

In priority order, addressing only what strengthens the design without descoping correctly-included items:

1. **Add pipeline shard schema.** Enumerate fields with types and initial values. 10-15 lines. Unblocks everything downstream. (Conceded, critic concern #4.)

2. **Define `waiting_for` structure.** List of `{shard_id, expected_status}` tuples, satisfied when all entries match. 2-3 sentences. (Conceded, critic concern #5.)

3. **Add monitoring to the v1 scope table.** Fix the inconsistency the critic identified. Both `cxp dashboard` and Grafana panels belong in v1 scope.

4. **Note that v1 trigger evaluator has a fixed condition vocabulary.** Clarify that the YAML is a structured config with 6 known field types, not a generic query language. Defuses the overengineering concern without descoping.

5. **Add one sentence each on evidence storage and Ralph Loop integration.** Evidence is appended to shard content. Ralph Loop checks shard status via `cxp shard status`. (Partly conceded, critic concern #3.)

6. **Add one line on playbook format.** "v1 playbook is a single markdown file containing decision trees derived from the phase rules in this design." (Partly conceded, critic concern #6.)

7. **Add a sentence on poller resilience.** "Transient failures (DB unreachable, tmux spawn failure) are retried on the next 30-second cycle; the lock TTL prevents orphaned state."

These are all small additions (collectively under 30 lines of content). The design does not need structural changes. It needs a few concrete definitions that the revision omitted because they felt like implementation details — but they are, as the critic correctly argues, design-level decisions that downstream work depends on.
