# Critic Review

## Overall Judgment

The M Pipeline proposal describes a sophisticated five-phase automation system built on a sound foundational insight: shard-based state makes orchestration resumable and stateless. However, the proposal over-engineers the design phase, under-specifies the hardest coordination problems, and fails to demonstrate that the pipeline's overhead justifies itself compared to the strong baseline of James giving a well-scoped task directly to an agent with `/implement`.

The proposal reads as an architecture document for a mature platform, but the "what needs building" list reveals that most of the pipeline does not exist yet. The gap between the described system and the current state is large enough that the proposal risks becoming shelfware — too ambitious to build incrementally, too interconnected to deliver in useful slices.

## Major Concerns

### 1. The C/D/S loop at design phase is not justified for the claimed audience

The primary users are AI coding agents and James. James provides "worked-through designs that need sharpening." The proposal then subjects these designs to up to 5 rounds of Critic/Defender/Synthesis, each round spawning 3 agent sessions across multiple model families.

**The problem:** Most designs James would feed into this pipeline are for features in a codebase he knows intimately. The C/D/S loop is designed for adversarial review of uncertain proposals. For "add notification rollups to the pipeline" or "create a new shard type," multi-round adversarial review is pure overhead. The proposal acknowledges this ("M also judges whether the design needs the full C/D/S loop at all") but then spends 80% of Phase 1 detailing the full loop, making the escape hatch feel like an afterthought.

**What this means:** The pipeline's most visible feature — automated multi-model design review — is likely to be skipped for most real work. The pipeline's value proposition shifts entirely to Phases 2-5, which are less developed in the proposal.

### 2. M's hydration model is underspecified for the decisions M must make

M is described as ephemeral: it reads a pipeline shard, does one action, exits. But the decisions M must make are not simple state transitions:

- "Judge whether the design needs the full C/D/S loop" — requires understanding the design's complexity and risk
- "Evaluate synthesis against exit rubric" — requires reading and assessing multiple review documents
- "Decide if a stall is a task scoping problem, a model problem, or truly stuck" — requires understanding the implementation context
- "Decide if a review escalation reveals a design problem vs. a code problem" — requires judgment

These are not lookup decisions. They require the kind of nuanced reasoning that benefits from context that M, by design, does not have. The proposal assumes M can make high-quality orchestration decisions from a lean hydration payload, but the hardest decisions M faces are exactly the ones where context matters most.

**The real risk:** M becomes a dumb dispatcher that either rubber-stamps transitions or escalates everything to James, collapsing the pipeline back to manual orchestration with extra steps.

### 3. The trigger mechanism has unaddressed coordination gaps

The proposal says M is "event-driven" and lists triggers like "agent completes a task (status -> needs-review) -> spawn M." But:

- **Who watches for these events?** A cron job polling shard status? A webhook? This is listed as "what needs building" item #3 but it is the entire nervous system of the pipeline.
- **What happens if two events fire simultaneously?** M-1 is processing a needs-review for task 3 while task 4 also completes. Does a second M spawn? Do they coordinate? The proposal says "they don't conflict because each reads its own pipeline shard" but both tasks belong to the same pipeline shard.
- **What about the DEPLOYING/TESTING/READY state machine?** This is a shared mutex across the entire pipeline. If M-1 sets DEPLOYING and then crashes, who clears it? The proposal says M is killable ("Kill M at any time"), but the state machine creates exactly the kind of shared state where a killed M leaves the pipeline locked.
- **What is the latency model?** If M is spawned per event and each spawn involves loading a Claude Code session with playbook + shard context, the overhead per transition could be 30-60 seconds. For a pipeline with 15-20 transitions, that is 10-15 minutes of pure orchestration overhead.

### 4. Two-pass decomposition assumes capabilities that don't exist

Phase 2 describes a two-pass decomposition where a domain agent produces a task tree, then separate sessions review each top-level task in detail. This requires:

- The domain agent to have enough codebase knowledge to identify correct file paths, function names, and schema impacts in Pass 1
- Fresh sessions in Pass 2 to meaningfully review scope and testability without deep codebase context
- The implementing agent to review tasks "they'll receive" before receiving them — meaning the agent must be spawned just to review, then killed, then re-spawned to implement

**The problem:** Agent sessions don't have persistent codebase memory. Each session starts cold. The "domain agent knows the codebase" claim is only true if that agent's playbook contains sufficient codebase documentation, which is a significant maintenance burden the proposal does not acknowledge. Pass 2's "fresh session per top-level task" means each session must re-orient itself in the codebase to evaluate scope — expensive and error-prone.

The implementing agent review step (2e) is the most valuable part of Phase 2 and the one most likely to be cut for cost reasons.

### 5. Splitting test authoring from implementation creates more problems than it solves

The proposal splits testing into:
- Unit tests: written by the implementing agent (needed for Ralph Loop)
- E2e tests: written by a separate test agent from the spec, not from the code

**The claimed benefit:** E2e tests verify what was specified, not what was built.

**The actual problem:** E2e tests written from a spec without seeing the code will frequently:
- Test at the wrong abstraction level (testing CLI output format when the spec says "users can trigger rollups")
- Miss setup/teardown requirements that only make sense when you see the implementation
- Duplicate coverage with unit tests because the spec doesn't distinguish layers
- Fail on first run because the test agent didn't know about infrastructure constraints (database state, file paths, service dependencies)

The proposal also doesn't address who fixes broken e2e tests. If the test agent writes a test that fails because the test itself is wrong (not the implementation), the pipeline needs a diagnosis step that isn't described.

## Secondary Concerns

### 6. Multi-model diversity claim is asserted, not demonstrated

"Different models produce genuinely different perspectives, not simulated disagreement from the same model in different personas." This is stated as fact without evidence. It may be true for some classes of review, but the proposal doesn't distinguish where model diversity adds signal versus where it adds latency and cost. Using Gemini to review Claude's code doesn't automatically produce better reviews — it might produce worse ones if Gemini is less familiar with the idioms Claude tends to generate.

### 7. A/B implementation is expensive and rarely useful

Running two agents on the same task with different models, then comparing results, doubles implementation cost. The proposal correctly notes this is "expensive" but still includes it as a pipeline feature. In practice, A/B implementation is a research tool, not a production pipeline feature. Including it makes the proposal feel aspirational rather than practical.

### 8. Iteration budgets are arbitrary

Max 5 C/D/S rounds, max 10 Ralph Loop iterations, max 3 review rounds. These numbers are presented without rationale. Are they based on observed data? Guesses? The difference between 3 and 10 Ralph Loop iterations is the difference between a 5-minute task and a 50-minute task. Getting these wrong in either direction (too few = incomplete work, too many = wasted tokens) is a real operational risk.

### 9. The "what needs building" list is honest but alarming

10 items, several of which are substantial systems (trigger mechanism, deploy/test state machine, stall detection). The proposal doesn't estimate effort, sequence dependencies between these items, or identify a minimum viable pipeline. This means the first useful pipeline run is blocked by building all 10 items — there is no incremental path described.

## Strongest Simpler Alternative

**James writes a design doc. James runs `/implement` with the spec. The agent uses Ralph Loop to iterate. James reviews the PR.**

This baseline:
- Requires zero new infrastructure
- Works today
- Gives James full control over scope and priority
- Produces working code in a single agent session
- Has no coordination overhead

The M Pipeline must demonstrate that it is materially better than this baseline for a meaningful class of work. The proposal never directly compares against this baseline or identifies the threshold where pipeline overhead pays for itself.

The honest answer is probably: the pipeline pays off only for multi-task designs that span 4+ tasks with dependencies, where the coordination cost of manually dispatching and sequencing agents exceeds the overhead of running M. That is a narrower value proposition than the proposal implies.

## Differentiation Test

Ask: "What can M Pipeline do that James + `/implement` + manual dispatch cannot?"

1. **Automated sequencing of dependent tasks** — Real value, but only for multi-task designs
2. **Cross-model review** — Possible value, unproven
3. **Automated C/D/S on designs** — Marginal value for the typical design James writes
4. **Stall detection and model swapping** — Real value, but could be built as a standalone feature
5. **Full traceability** — Real value for audit, low value for day-to-day work
6. **Concurrent multi-pipeline execution** — Real value only at scale James hasn't described needing

Items 1 and 4 are the strongest. The pipeline should be designed around these, not around the C/D/S loop.

## Questions the Proposal Must Answer

1. **What is the minimum viable pipeline?** Which of the 5 phases can be deferred, and what is the smallest useful subset that delivers value over the baseline?

2. **How does M handle shared pipeline state when multiple tasks in the same pipeline trigger events simultaneously?** The "each M reads its own pipeline shard" claim breaks down when multiple tasks share a pipeline.

3. **What is the expected cost (in tokens and time) of running a design through the full pipeline versus James doing it manually?** If the overhead is 3x for 1.2x quality improvement, the pipeline loses.

4. **What happens when M makes a bad orchestration decision?** The proposal describes escalation to James but not recovery from M's own mistakes (e.g., M merges a PR that breaks things, M skips a review round it shouldn't have).

5. **How does the trigger mechanism actually work?** This is the single most important piece of new infrastructure and it gets one bullet point.

6. **What is the cold-start cost of an M session?** If hydrating M takes 30 seconds and 10k tokens each time, the ephemeral model may be more expensive than a persistent orchestrator.

7. **Who maintains the domain agents' codebase knowledge?** Pass 1 decomposition depends on agents knowing file paths and function names. How does this knowledge stay current as the codebase evolves?

## Recommended Revisions

1. **Define the minimum viable pipeline explicitly.** Probably: Phase 2 (decompose) + Phase 3 (implement with dispatch) + simple Phase 4 (review). Skip automated C/D/S and automated deploy for v1.

2. **Design the trigger mechanism first.** It is the critical path for everything. The rest of the pipeline is process documentation without it.

3. **Drop A/B implementation from the core pipeline.** Make it an opt-in experiment mode, not a standard feature.

4. **Merge the test agent back into the implementing agent for v1.** Split test authoring is a nice theory but adds coordination cost that isn't justified until the pipeline is proven.

5. **Add a cost model.** For a representative design (e.g., "add notification rollups"), estimate: how many agent sessions, how many tokens, how much wall-clock time for pipeline vs. manual. If the numbers don't clearly favor the pipeline for multi-task work, the proposal needs to narrow its scope.

6. **Address the shared-state coordination problem directly.** The DEPLOYING/TESTING/READY state machine and multi-task pipelines need explicit concurrency design, not hand-waving about separate shards.

7. **Compare against the baseline in the proposal itself.** The absence of this comparison is the single biggest credibility gap. Every pipeline proposal should answer: "Why not just do it manually?"
