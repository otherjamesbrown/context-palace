# Synthesis Review

## Current Core Thesis

The M Pipeline turns manual, attention-consuming multi-agent coordination into an automated, fault-tolerant pipeline by using shard state as the single source of truth and ephemeral orchestrator sessions that can be killed and restarted without losing progress.

## Current Assessment

The proposal has a strong architectural core and a clear problem worth solving. It is weakened by treating the full five-phase system as a single undifferentiated deliverable. The document conflates the vision (what the pipeline eventually becomes) with the plan (what gets built first and why). The most important next step is separating these two concerns.

## Review Phase

**Shaping phase.** The proposal is not ready for implementation planning because it lacks an MVP definition, a build sequence, and a concurrency model. The architecture and phase structure are sound enough to build from, but the document needs one more revision focused on scoping before any code gets written.

## What Both Sides Agree On

These points carry the strongest signal because neither side disputes them:

1. **MVP must be defined explicitly.** Both sides say the 10-item build list needs sequencing and the proposal needs to identify the smallest useful pipeline. This is the single highest-priority revision.

2. **The trigger mechanism is critical path.** The critic calls it "the entire nervous system." The defender agrees it needs to be built first. Neither side offers a concrete design, which means the next draft must.

3. **Concurrency on shared pipeline state needs a real design.** Lock field with TTL, check-and-set, backed-off retries. Both sides agree this is a real gap. The defender sketches the answer in three sentences; the proposal should include it.

4. **Iteration budgets are educated guesses and should be labeled as such.** Both sides accept the budgets as useful guardrails but agree they need data-driven tuning over time.

5. **The ephemeral M model is architecturally correct.** The critic worries about M's decision quality but does not argue for a persistent orchestrator. The defender argues the risk is playbook quality, not architecture. Both are right: the architecture stays, but the playbook design becomes a first-class concern.

6. **The test diagnosis gap is real.** When an e2e test fails, the pipeline needs a step to determine whether the test or the implementation is wrong. Neither side disputes this.

7. **Phase 1 C/D/S should not be the default path.** The critic says it will be skipped for most work. The defender agrees the escape hatch should be foregrounded. The full loop should be the exception for complex/risky designs, not the norm.

## Open Disagreements

### 1. Is the "simpler baseline" comparison a real gap?

The critic demands the proposal justify itself against "James + /implement." The defender argues this comparison is misframed because it ignores multi-task coordination cost.

**Judgment:** The defender wins the argument but the critic wins the point. The pipeline obviously targets multi-task work, not single-task work. But the proposal should still include a brief worked example showing where the crossover happens (roughly: 3+ dependent tasks). This is not because the comparison is intellectually necessary but because it pre-empts the obvious skeptical reading. One paragraph, not a full cost model.

### 2. Can M make quality orchestration decisions from a lean hydration payload?

The critic says M's decisions (skip C/D/S? stall or scoping problem? design flaw or code flaw?) require more context than the hydration model provides. The defender says most are heuristic routing decisions and the rest use the documents themselves as context.

**Judgment:** Both are partly right. The proposal should distinguish between M's routing decisions (simple, heuristic-driven, low-context) and M's quality judgments (require reading documents, benefit from good prompts). The playbook must provide clear decision trees for the routing cases. For quality judgments, the proposal should acknowledge that M's accuracy will improve iteratively as the playbook gets refined from real pipeline runs. Do not over-specify this now.

### 3. Does split test authoring (e2e from spec, unit from implementation) add value?

The critic says spec-only e2e tests will be brittle and misaligned. The defender says the task shards provide enough concrete detail and that testing intent-not-implementation is genuinely valuable.

**Judgment:** Defer to v2. The critic is right that this adds coordination cost before the pipeline is proven. The defender is right that the idea has merit. Mark it as a future capability. For v1, the implementing agent writes all tests.

### 4. Is multi-model diversity asserted or proven?

The critic wants evidence. The defender says it is established in practice.

**Judgment:** The defender is right that this does not need a research citation. But the proposal should be more precise about *where* model diversity matters (reviews, stall recovery) versus where domain fit matters more (implementation). The current framing is fine; no change needed beyond the existing table in Cross-Cutting Concerns.

## Must Fix Now

1. **Add a Minimum Viable Pipeline section.** Define v1 explicitly: pipeline shard type, trigger mechanism (cron polling shard status), Phase 3 dispatch with dependency ordering, simple Phase 4 review (single reviewer, same model family is acceptable for v1). Phase 1 C/D/S automation, split test authoring, A/B implementation, and Phase 5 deploy automation are v2+. Order the 10 build items into a delivery sequence with v1/v2/v3 labels.

2. **Design the trigger mechanism concretely.** v1 is a cron job polling shard status every 30 seconds. State what it watches for, what it spawns, and how it avoids spawning duplicate Ms. This does not need to be elaborate, but it must exist in the document.

3. **Specify the concurrency model for pipeline shards.** Lock field with TTL on the pipeline shard. M acquires lock before acting, releases on completion. Stale locks expire after TTL (e.g., 5 minutes). Second M backs off and retries. Three to five sentences in the proposal.

4. **Foreground the Phase 1 fast path.** Restructure Phase 1 so the default is: M runs implementability check, design goes to Phase 2. The full C/D/S loop is triggered only when the design is complex (touches multiple subsystems, has unclear scope boundaries, or James explicitly requests it). Move the detailed C/D/S sub-phases into a subsection rather than the main flow.

5. **Add a brief crossover example.** One paragraph showing a representative multi-task design (e.g., notification rollups: 5 tasks, 3 dependency edges) and why pipeline coordination beats manual dispatch for this case. Not a full cost model. Just enough to anchor the value proposition concretely.

## Defer

- **Full cost model with token estimates.** Useful eventually but premature before the pipeline has run once. Track actuals from early runs instead.
- **Split test authoring.** v2 capability. For v1, implementing agent writes all tests.
- **A/B implementation.** Already positioned as opt-in/expensive. Label it v2+ and move on.
- **Multi-model diversity enforcement.** Worth pursuing but not a v1 blocker. v1 can run all phases on Claude. v2 introduces cross-model reviews.
- **Deploy/test state machine (DEPLOYING/TESTING/READY).** v2. For v1, deploys are manual or single-threaded (one merge at a time, no mutex needed).
- **Detailed M playbook design.** The playbook is a separate document. The proposal should describe what the playbook must contain (decision trees for routing, escalation criteria, phase transition rules) but not write it inline.

## What Not to Change

- **The ephemeral M architecture.** This is the strongest idea in the proposal. Shard-is-the-state, kill-and-resume, no persistent process. Do not weaken this in response to the critic's decision-quality concerns. Address those concerns via playbook quality instead.
- **The five-phase structure.** The phases are logically sound and well-ordered. The issue is not the structure but the lack of an explicit MVP that uses a subset. Keep all five phases in the document as the full vision; add the MVP section to show what ships first.
- **The non-goals section.** Well-chosen scope boundaries. Do not expand scope in response to any feedback.
- **The parking lot concept in Phase 1.** Practical and directly addresses review bloat. Keep as-is.
- **Dependency graph driving merge order.** This is the pipeline's highest-value automation. Do not dilute it.
- **The "what already exists" table.** Important context showing this is integration work, not greenfield. Keep it.

## Revision Plan for Next Draft

Priority order for the next revision:

1. **Add "Minimum Viable Pipeline" section** immediately after the Architecture section. Define v1 scope, delivery sequence for build items, and what is explicitly deferred to v2+. This is the single most important change.

2. **Add "Trigger Mechanism" subsection** under Architecture (or as its own section). Concrete v1 design: cron poller, what it watches, spawn logic, duplicate avoidance.

3. **Add "Concurrency Model" subsection** under Architecture. Lock/TTL on pipeline shards, check-and-set, back-off behavior.

4. **Restructure Phase 1** so the fast path (skip C/D/S, run implementability check, proceed) is the primary flow and the full C/D/S loop is a clearly marked branch for complex designs.

5. **Add one worked crossover example** in the Problem section or Primary Outcome section. Brief, concrete, anchoring.

6. **Label iteration budgets as defaults-pending-data** and note they will be tuned from early pipeline runs.

7. **Add test diagnosis step** to Phase 4 (or between Phase 3 and 4): when e2e tests fail, determine whether the test or the implementation is at fault before looping back to the implementer.

8. **Move split test authoring, A/B implementation, and deploy automation to a "Future Capabilities" section** at the end, clearly marked as v2+.

Items 1-3 are structural and should be done together. Items 4-8 are refinements that can follow.
