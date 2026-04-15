# Proposal: Evidence-Aware Context for AI Coding Agents

## Summary

This proposal is about one product outcome:

**when an AI coding agent gets a short bug-fix or investigation task in an unfamiliar repository, it should reach the first correct subsystem plus executable check faster.**

The proposed product artifact is a **task-specific context pack**: the minimal task-launch working set that helps the agent orient, route to the right code and checks, and calibrate how much to trust surrounding knowledge.

This is not a proposal for a new documentation structure. It is a proposal that, in drifting codebases, agents need a small assembled launch context that is more selective and more trust-aware than either a flat context dump or a thin launcher built from search alone.

A strong launcher can rank likely places to start. A context pack pre-assembles the smallest credible working set for the first turn, so the agent does not have to stitch route, evidence, and warnings together itself.

The main tensions are familiar:

- too much context wastes attention and tokens
- too little context leaves the agent without a map
- stale context misroutes the agent even when retrieval looks relevant

The current leading design direction is:

- small hot/warm/cold layered retrieval
- index-first lazy loading in the spirit of `llms.txt`
- usage-aware promotion where repeated value is observed
- evidence-aware trust and freshness signals
- automated metadata collection and lightweight maintenance

These are candidate mechanisms in support of the product outcome, not equally proven commitments.

## Primary User, Workflow, and Outcome

The primary user is:

**an AI coding agent asked to do a short bug-fix or investigation task in a repository it does not yet know well.**

The primary workflow is:

1. the agent receives a short task such as `investigate duplicate digest bug`
2. the system assembles a compact context pack for that task
3. the pack points the agent to the likely subsystem, likely code entrypoints, nearest executable evidence, and important warnings
4. the agent verifies behavior in code, tests, and other executable checks instead of over-trusting prose

The primary promised outcome is:

**less time to first correct subsystem plus executable check.**

Operationally, that should also mean:

- fewer irrelevant files or documents loaded before reaching the working area
- fewer stale-document detours in unfamiliar repos

## The Core Problem

Agents in real repositories do not fail only because they lack information. They often fail because the information environment is badly shaped for machine use.

Useful knowledge is spread across:

- code
- tests
- docs
- design notes
- tickets
- migration history
- configuration
- operational conventions

For a short task, the agent has to answer a small set of high-value questions quickly:

- what area of the repo does this task belong to?
- where should I look in code first?
- what tests or other executable checks are closest to the behavior in question?
- which documents are worth reading?
- which documents are conceptually useful but operationally dangerous?

That creates three failure modes:

- **too much context**: the agent gets a large dump and loses signal in noise
- **too little context**: the agent has no map and does not know what to search for
- **stale context**: the agent retrieves something relevant-looking that no longer matches implementation reality

The dominant pain this proposal focuses on is:

**wrong first turns before the agent reaches the first correct proof path.**

That wrong turn may look like:

- entering the wrong subsystem because the task language is ambiguous
- spending time in a plausible but stale document before checking the live implementation
- finding code but not the nearest executable evidence needed to confirm or falsify a hypothesis

So the problem is not generic retrieval. It is:

**task-specific launch routing and trust calibration in a drifting codebase.**

## Core Claim and Product Artifact

### Core Claim

The central claim of this proposal is:

**AI coding agents need task-specific context packs that help them reach the right code and nearest executable evidence quickly while calibrating how much to trust surrounding prose.**

That is the thesis this proposal should stand or fall on.

### What A Context Pack Is

A context pack is not just a ranked list of retrieval hits.

It is the **minimal task-launch working set** assembled around one job:

- orient the agent
- reduce likely wrong turns
- point to the first credible proof path
- warn when surrounding prose should be treated carefully

In other words, the product distinction is not "more documents." It is the packaging of:

- orientation
- routing
- proof-surface pointers
- trust guidance

around a specific short task.

That pack may be very small. In some cases it should contain little more than:

- repo rules and source-of-truth guidance
- a likely subsystem pointer
- likely code files
- likely tests or other executable checks
- a warning that a nearby doc may be stale

The proposal succeeds if that assembled launch set gets the agent to the first correct subsystem plus executable check materially faster than a thinner launcher does.

## Why This Thesis Matters In Practice

An earlier version of this proposal was easier to read as:

- a knowledge architecture for AI agents
- a layered retrieval model
- a way to organize documentation and trust signals

That framing was directionally useful, but it was still too abstract. It described a system shape more than a concrete product win.

The stable thesis is narrower and more practical:

- the problem is wrong first turns
- the user is an AI coding agent on a short task in an unfamiliar repo
- the product win is faster arrival at the first correct subsystem plus executable check
- the artifact is a small launch pack that pre-assembles route, evidence, and warnings

That shift matters because it changes the question from:

- how should project knowledge be organized for agents?

to:

- how do we help an agent become useful quickly in a real repo without over-trusting stale prose?

### Why This Matters For Rapid Development Projects

In fast-moving projects, the problem is usually not missing documentation alone. It is that:

- architecture changes quickly
- docs drift quickly
- teams do not want to pause and build a perfect KB before shipping
- AI coding can increase the rate of change even further

The stable thesis helps here because it does not require complete documentation before the system becomes useful.

Instead, it supports a lighter operating model:

- keep rules and a minimal map current
- treat code and executable checks as the proof surfaces
- use prose mainly to reduce repeated confusion and route agents faster
- let metadata and drift review be increasingly automated

That means the value is immediate:

- agents lose less time guessing which subsystem a task belongs to
- agents are less likely to follow stale build-time intent as if it were implemented truth
- reusable knowledge can grow from repeated work instead of from up-front completeness

### Why This Matters For Inherited Legacy Codebases

In a large legacy codebase, the problem is usually different:

- there is already a lot of documentation
- different eras of the system disagree
- ownership and naming are blurry
- the first wrong turn can waste a lot of time

In that setting, the proposal helps because it does not assume the docs can be cleaned up first.

Instead, it focuses on a narrower but higher-value job:

- get the agent to the most likely subsystem quickly
- point it to the nearest executable proof surface
- surface warnings when a relevant document may be stale or only conceptually useful

That makes onboarding into legacy reality more practical. The goal is not "understand the whole system first." The goal is:

- stop making expensive early mistakes
- reach live evidence faster
- use documentation as routing and warning material instead of assumed truth

### The Practical Difference

The earlier thesis mainly helped explain a knowledge system.

The stable thesis helps answer a much more immediate question:

**how do we make AI coding agents useful on real short-task work in fast-moving and messy repositories?**

That is why this proposal can matter in both:

- rapid development projects that need lightweight, automation-friendly context support
- inherited legacy codebases that need safer, faster agent onboarding into a drifting system

## The Truth Hierarchy

One point should be explicit:

**code and tests are the primary proof surfaces, broadened where needed to the nearest executable evidence.**

Context packs, indexes, and knowledge units are subordinate. They are mainly:

- routing aids
- summaries
- warnings
- accelerators

Their job is to help the agent reach and interpret implementation evidence faster. They do not replace implementation verification.

In practice:

- code shows what is currently implemented
- tests show what behavior is exercised or expected when good tests exist
- other executable evidence may include scripts, fixtures, commands, migrations, logs, or reproducible checks tied to the task
- prose helps the agent navigate, frame the task, and avoid known traps

This also means a pack may sometimes include very little prose. If the shortest trustworthy route is "repo rules + likely files + likely checks + freshness warning," that should be acceptable.

## Why The Stronger Baseline Is Still Not Enough

The strongest simpler baseline is not generic search. It is something like:

- tiny repo map
- source-of-truth rules
- task classification
- code/test-first retrieval
- lightweight prose fallback
- stale-doc warning when possible

That baseline is necessary and may already improve many repos. The proposal should be judged against it honestly.

The remaining claim is narrower:

**for short-task work in unfamiliar repos, that launcher still leaves too much stitching burden on the agent at the moment when false starts are most expensive.**

The launcher is not wrong; it is just incomplete. It can suggest where to start, but it still leaves the agent to assemble the working set, interpret the evidence, and decide what to trust before the first real turn.

### Scenario 1: Wrong Subsystem First

Task:

```text
investigate duplicate digest bug
```

A strong launcher may return:

- top search hits for "digest"
- a repo map showing ingestion, notifications, and scheduling
- a few candidate tests

The unresolved problem is that the agent still has to decide whether "digest" means:

- content hashing
- email digest generation
- duplicate job suppression

If the first turn is wrong, the agent can spend several retrieval hops and multiple file reads before reaching the first relevant check. A context pack adds value if it pre-assembles the likely working set for the dominant interpretation while also surfacing the nearest disambiguating evidence.

### Scenario 2: Stale Prose Detour

Task:

```text
why does token refresh fail after reconnect?
```

A strong launcher may correctly retrieve a design note and a few auth files. But if the design note predates a migration from session-based refresh to rotating tokens, the agent can take a plausible but wrong route before checking code.

The missing piece is not just a stale-doc label after retrieval. It is launch guidance that says, in effect:

- this note is conceptually relevant
- implementation details may have drifted
- verify first in these files and this test or repro path

That changes the order of operations at launch time, not just the ranking.

### Scenario 3: Code Found, Proof Path Missing

Task:

```text
investigate why sync retries never stop
```

A strong launcher may find the likely retry code quickly. But the agent can still lose time if it does not know whether the nearest proof surface is:

- a unit test
- an integration test
- a queue worker script
- a fixture-driven repro command

The context pack adds value if it connects the code entrypoint to the nearest executable evidence instead of leaving the agent to discover that linkage ad hoc.

These scenarios are the real comparative case for pack assembly. If they do not hold up, the proposal should weaken. If they do hold up, the proposal is meaningfully more than "better retrieval results."

## Leading Retrieval Shape: Hot, Warm, and Cold

The current leading retrieval shape is a three-tier model:

- **hot context**: small always-loaded rules, navigation guidance, and project map
- **warm context**: subsystem or domain indexes loaded after initial task classification
- **cold context**: focused knowledge units loaded only when they materially help the task

This model exists to balance two risks:

- loading everything overwhelms the agent
- loading nothing leaves the agent without a map

It should be treated as a plausible assembly strategy, not the heart of the thesis.

### Hot Context

Hot context should stay small and stable. Its job is orientation, not exhaustive explanation.

Examples:

- repo-level rules
- workflow conventions
- source-of-truth precedence
- a short project map of major domains

### Warm Context

Warm context helps narrow the task into the correct area of the repo.

Examples:

- authentication index
- ingestion index
- scheduling index
- frontend architecture index

Its role is to reduce search space and point to the most relevant next evidence.

### Cold Context

Cold context contains focused knowledge for a narrowed task area.

Examples:

- retry behavior in one subsystem
- token refresh flow
- sync pipeline caveats
- digest generation workflow

Cold context should accelerate the task without pretending to be the final proof surface.

## Index-First Lazy Loading

This proposal treats index-first lazy loading as a strong candidate pattern:

- start with a compact overview
- enumerate what more detailed material exists
- load specifics only when needed

That is the useful lesson borrowed from `llms.txt` style thinking. Applied here:

- the always-loaded layer should be a map, not a manual
- the next layer should narrow the search space
- detailed knowledge should be loaded lazily

This should still be treated as a hypothesis under evaluation, not a doctrinal claim. The right review questions are:

- is index-first retrieval better than a richer always-loaded guide plus search?
- do the extra navigation steps help enough to justify the indirection?
- can the indexes stay concise and useful as the repo changes?

## Evidence-Aware Trust and Freshness Signals

Retrieval quality is not enough if the agent cannot judge whether a retrieved document is likely current.

The proposal therefore includes evidence-aware trust and freshness signals such as:

- likely current
- conceptually useful but detail drift suspected
- likely stale
- structurally volatile

The important point is not the label names. It is that the labels should change agent behavior.

Intended behavior:

- if a knowledge unit is likely current, use it as a routing and summary aid, then verify in code/tests
- if detail drift is suspected, use it for concepts and terminology, then verify operational details immediately
- if a unit is likely stale, do not use it as execution guidance; use it only as a clue about where to investigate
- if content is structurally volatile, rely more heavily on recent code, tests, and change history

These signals should also be explainable. The system should say:

- why the warning exists
- what evidence produced it
- what verification move is recommended

The reliability bar should be conservative:

- a trust signal does not need to prove truth
- it does need to be explainable enough that a reviewer can see why it was attached
- false confidence is worse than visible uncertainty
- "drift suspected" is often a more credible first-version output than precise claims about staleness

Illustrative evidence sources:

- related files changed after the knowledge unit was updated
- linked tests changed materially
- referenced paths no longer exist
- migrations landed in the same area
- the document contains implementation-sensitive operational detail

## Metadata and Minimal Automation

This approach depends on metadata, but it cannot depend on heavy manual upkeep.

That is a practical constraint:

- teams already struggle to maintain docs manually
- architecture notes drift even in disciplined organizations
- AI-accelerated development increases the rate of change

So the operating model should be:

- automate collection by default
- let humans review or correct important inferences
- continuously flag likely drift

### Minimal Metadata Shape

Useful lightweight metadata may include:

- knowledge type
- related files
- related tests or executable checks
- related migrations or schema references
- subsystem or service mapping
- stability or volatility hints

This metadata should still be useful when incomplete.

### Essential Maintenance Capabilities

The proposal does not need a large automation cloud to be credible. It mainly depends on one or two maintenance capabilities such as:

- linking changed code and tests back to likely knowledge units
- flagging likely drift when implementation evidence moves beyond what a knowledge unit still supports

Broader automation can remain deferred.

## Task Lifecycle And Feedback Loop

This proposal becomes more practical when the lifecycle is explicit.

The intended loop is:

1. a work item is created
2. that work item becomes the trigger for task-launch assembly
3. the system compiles a context pack for the active task
4. the agent works from that pack and verifies in code, tests, and other executable evidence
5. the system records what was actually useful during the task
6. maintenance workflows turn those observations into better future metadata, drift flags, and pack assembly

### 1. The Work Item

In Context Palace terms, the durable task artifact is usually a shard:

- feature shard
- bug shard
- investigation shard
- review shard

That shard is the task seed, not the launch pack itself.

Its role is to capture the human-facing statement of work:

- what is changing
- why it matters
- constraints or acceptance hints
- any known subsystem pointers

### 2. Launch-Pack Assembly

When the task becomes active, the system compiles a task-specific context pack from current evidence.

That assembly can be triggered by:

- agent startup on a task
- a `cxp context compile` style command
- a background precompute step for active work
- a TUI or CLI action such as "launch task context"

The important point is architectural:

- the shard is durable planning context
- the launch pack is a generated execution artifact

### 3. What The Pack Is Built From

The pack builder should assemble from several layers:

- hot context
  - repo rules
  - source-of-truth order
  - task shard
  - current high-signal branch or work context
- warm context
  - linked KB shards
  - relevant parent and sibling shards
  - subsystem indexes
  - promoted high-value descendants
- cold context
  - likely code entrypoints
  - tests and other executable checks
  - migrations
  - service boundaries
  - recent change evidence
- trust signals
  - likely current
  - drift suspected
  - likely stale
  - structurally volatile

The result should usually be small. The goal is not to restate the project. The goal is to pre-assemble the first credible working set for one task.

### 4. Where The Pack Lives

The best default is to treat the launch pack as an ephemeral generated view:

- injected into the agent's startup context
- rendered in the TUI as a task-launch surface
- printed by CLI
- optionally cached for inspection

It may be useful to persist a snapshot for debugging or evaluation, but the pack should not be maintained manually as a second durable source of truth.

### 5. What Gets Recorded After Work

After the task, the system should record what the agent actually used or discovered, for example:

- files opened repeatedly
- tests or checks used as proof surfaces
- shards loaded
- warnings that proved accurate
- important missing context the pack failed to include
- paths that were suggested but turned out to be dead ends

This is the key distinction:

- the system should first write these as **task observations**
- it should not immediately rewrite the shard as if those observations were already reviewed truth

That observation layer can live as:

- structured task-run metadata
- pack evaluation traces
- lightweight usage logs linked to the shard
- suggested metadata patches awaiting confirmation

### 6. How Those Observations Improve The System

A separate maintenance process can then consume the observations and decide what to do.

That process might:

- update shard metadata
- strengthen links to files or tests
- flag a shard for review because its evidence no longer matches current code
- promote a frequently used descendant into a higher-level index
- improve future launch-pack assembly for similar tasks

This separation matters.

If every runtime observation is written straight back into the shard, the shard becomes noisy and untrustworthy. If observations are ignored entirely, the system never learns from real work.

So the right model is:

- shard as durable human-reviewed task context
- launch pack as generated task-execution context
- observations as a separate evidence layer
- maintenance workflows as the bridge between observations and durable metadata

### Why This Loop Matters

Without this loop, launch packs risk becoming one more static artifact that drifts.

With this loop, work improves future work:

- new tasks trigger better packs
- completed tasks produce evidence about what was actually useful
- maintenance workflows turn that evidence into better routing, better trust signals, and better shard metadata over time

That is especially important in both:

- fast-moving projects where manual upkeep will always lag
- legacy codebases where the system has to learn which context is still operationally reliable

## Scope: Entry Story And Broader Applicability

The entry story should be:

**an agent handling a short task in an unfamiliar existing repository.**

That is the clearest wedge because it concentrates the routing, proof, and stale-context problems in one workflow.

The proposal still aims to apply across both greenfield and legacy settings:

- in greenfield projects, the challenge is keeping lightweight maps and knowledge aligned as the system evolves quickly
- in legacy projects, the challenge is triaging many competing truth surfaces with uneven freshness

The thesis does not require fully solving both modes at once. The product should start with the unfamiliar-repo wedge and expand outward.

## Anti-Goals And First-Version Exclusions

This proposal is not trying to do all of the following in a first version:

- replace normal code search or repository exploration
- make prose a source of truth over code and executable evidence
- fully automate architecture understanding
- solve every promotion or background-review workflow
- define exact decision policies for every freshness label
- become an implementation spec for storage, indexing, or ranking internals

Those may matter later, but they should not blur the core product claim now.

## Risks And Open Questions

Important issues remain open:

- is the context pack meaningfully better than a thinner task-launch bundle, or is the distinction mostly packaging language?
- is "reduced false starts before the first correct proof path" measurable enough in practice to guide evaluation?
- is hot/warm/cold actually the right retrieval spine, or just a plausible first shape?
- is index-first lazy loading better than a richer always-loaded guide plus search?
- will trust and freshness signals be credible enough to change agent behavior?
- how much metadata can be inferred accurately enough to be useful without creating noise?
- when should a pack include prose at all, versus returning mostly code/tests and warnings?
- can greenfield and legacy remain one product with different emphasis, or do they eventually diverge?

These should stay visible rather than being written around.

## Recommendation

The recommendation is to evaluate this direction in three layers:

1. **Core product claim**
   AI coding agents benefit from task-specific context packs that reduce false starts on short tasks in unfamiliar repos by getting them to the correct subsystem and nearest executable evidence faster.

2. **Current leading mechanisms**
   Hot/warm/cold layering, index-first lazy loading, usage-aware promotion, and evidence-aware trust signals are the current best candidates for assembling those packs.

3. **Deferred or exploratory extensions**
   Richer promotion logic, broader background reviewers, and expanded maintenance workflows remain in scope but should not carry the proposal.

If reviewers accept the core claim but challenge parts of the retrieval shape, the proposal is still useful because it makes its assumptions explicit. If reviewers also accept the leading mechanisms, evidence-aware context packs become a strong next product direction.

## Review Questions

1. Is the primary wedge now sharp enough: short-task bug-fix or investigation work in an unfamiliar existing repo?
2. Is the dominant failure mode clear enough: wrong first turns before the first correct proof path?
3. Is the primary promised outcome specific enough: reduced false starts and less time to first correct subsystem plus executable check?
4. Is the context pack definition clear as a minimal task-launch working set, rather than as a richer name for retrieval results?
5. Is the case against the strongest simpler baseline now fair and convincing?
6. Is the truth hierarchy explicit enough that code, tests, and nearest executable evidence remain the proof surfaces?
7. Are hot/warm/cold, index-first loading, and usage-aware promotion presented at the right confidence level: candidate mechanisms, not overclaimed certainties?
8. Are trust and freshness signals explained in terms of changed agent behavior and a conservative reliability bar?
9. Does the proposal keep greenfield and legacy applicability without losing the existing-repo entry wedge?
