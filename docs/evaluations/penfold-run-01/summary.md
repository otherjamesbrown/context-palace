# Penfold Evaluation Run 01 Summary

## Purpose

This summary captures what the first completed Penfold launch-pack evaluation taught us.

The aim of this run was not to prove the whole product. It was to test whether a **task launch pack** still adds value when:

- the repository is larger and more operational than Context Palace
- the repo already has KB routing and repo law
- the active runtime under test is **Claude Code**, not just a hypothetical generic agent

## Task Evaluated

### 1. Digest Behavior Routing

Files:

- `01-digest-behavior-routing-launcher.md`
- `01-digest-behavior-routing-launch-pack.md`
- `01-digest-behavior-routing-worksheet.md`

Main finding:

- the launch pack beat normal Claude Code clearly and beat the strong launcher narrowly but meaningfully

The most important practical insight was:

- once the launcher was already good, the remaining value came from pre-assembling the **comparative proof surface**
  - the digest e2e test family
  - the shared digest domain files in `pkg/digest/`

That changed the investigation from:

- "which digest-related area should I explore first?"

to:

- "compare the digest surfaces side by side, then expand only if the shared domain model does not explain the divergence"

## What We Validated

### 1. The Launch-Pack Thesis Survives A Stronger Repo And A Strong Runtime

This matters because Penfold is not a toy case.

Claude Code in Penfold already had:

- repo law
- KB routing
- a larger codebase
- substantial executable proof surfaces

Even in that environment, the launch pack still improved first-turn quality.

That is useful evidence that the artifact is not only compensating for weak tooling or weak prompting.

### 2. The Value Difference Narrows As The Launcher Improves

This is an important result.

The gap between:

- normal Claude Code
- strong launcher
- task launch pack

was not enormous.

The strong launcher already captured much of the easy gain by reducing subsystem ambiguity early.

The launch pack still helped, but mostly by:

- pre-assembling the exact working set
- linking the nearest proof surface more explicitly
- reducing the remaining stitching burden

That suggests the pack's strongest value is not "replace the launcher."

It is:

- outperform the launcher on tasks where the launcher still leaves too much comparative assembly to the agent

### 3. Penfold Is A Good Middle Case, Not The Final Scaling Proof

Penfold helps validate that the idea still works in a bigger, more operational repo.

But it does not fully prove the stronger scaling claim:

- that the value gap between a strong launcher and a task launch pack increases as complexity, ambiguity, and stale context increase

Why not:

- Penfold is larger than Context Palace, but still relatively coherent
- it is actively curated for agent use
- it already has KB routing and rule files
- it is not yet a high-entropy inherited legacy system

So Penfold is best treated as:

- confirmation that the launch pack still adds value beyond the small internal repo case
- not yet proof that the value differential grows sharply with complexity

## What Remains Unproven

We have not yet proven that launch packs pull away decisively in:

- large legacy codebases
- repos with overlapping subsystem vocabulary
- repos with multiple architectural eras
- repos with stale docs mixed with current code
- repos where the nearest executable proof is much harder to discover

That is the next thing the evaluation program should target.

## Current Judgment

At this stage, the evaluation suggests:

- the idea is real
- the idea is not just compensating for poor baseline setup
- the launch pack still adds value in a stronger repo with a strong agent runtime
- the strongest scaling claim remains plausible but unproven

## Recommendation

Continue, but sharpen the next validation target.

The next repo or task set should be chosen to stress:

- higher context entropy
- more overlapping subsystem language
- more stale or partial prose
- weaker initial proof discoverability

That is where we should expect the value gap between a strong launcher and a task launch pack to widen if the stronger thesis is true.
