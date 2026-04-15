# Launch Pack Evaluation Plan

## Purpose

This document defines the next step after the proposal work:

**test whether a pre-assembled task launch pack is materially better than a strong launcher for short-task AI coding work in unfamiliar repositories.**

The proposal is now strong enough that further editorial review is likely to produce diminishing returns. The main open question is no longer conceptual. It is evaluative:

- does the launch pack reduce wrong first turns?
- does it get the agent to the first correct subsystem plus executable check faster?
- does it outperform a strong simpler baseline enough to justify becoming a real product artifact?

This plan is intended to answer those questions with a small, practical validation loop.

## Core Thesis Under Test

The thesis being tested is:

**AI coding agents doing short bug-fix or investigation work in unfamiliar repositories benefit from a small, trust-aware task launch pack that gets them to the first correct subsystem plus executable check faster than a strong launcher alone.**

This plan is not trying to validate every mechanism in the proposal. It is primarily testing:

- whether the product artifact is real
- whether the difference versus a strong launcher is meaningful
- whether the primary metric is useful

## What We Are Comparing

### Baseline: Strong Launcher

The baseline should be a serious one, not a straw man.

It includes:

- repo rules and source-of-truth guidance
- a small repo map
- task classification
- code/test-first retrieval
- lightweight prose fallback
- stale-doc warning when available

This baseline is assumed to be reasonably strong and already helpful.

### Candidate: Task Launch Pack

The candidate should be the thinnest credible version of the proposed artifact.

It includes:

- repo rules and source-of-truth guidance
- likely subsystem or domain
- likely code entrypoints
- nearest executable evidence
- targeted warnings about stale or risky prose
- only the minimum supporting prose needed for orientation and disambiguation

The key difference is:

- the launcher suggests places to start
- the launch pack pre-assembles the smallest credible working set for the first real turn

## Primary Metric

The primary metric should be:

**time to first correct subsystem plus executable check**

This means:

- how long it takes the agent to reach the right part of the codebase
- and identify the nearest executable evidence that can confirm or falsify the current hypothesis

This metric is better than vague quality language because it measures the exact pain the proposal is trying to solve: wrong first turns.

## Secondary Metrics

Secondary metrics should support, not replace, the primary metric.

Suggested secondary metrics:

- number of wrong-subsystem turns before first correct proof path
- number of irrelevant files or documents opened before reaching the working area
- number of stale-prose detours
- time to first plausible fix or explanation hypothesis
- reviewer judgment of whether the launch artifact reduced stitching burden

If instrumentation is light, these can be recorded manually at first.

## Repositories To Test

Use two kinds of repositories.

### 1. Rapid Development Project

Choose a project where:

- architecture is still moving
- documentation is incomplete or changing
- the codebase is active enough that stale context is plausible

This tests:

- whether the launch pack helps when the KB is light and drift is fast

### 2. Inherited or Legacy-Style Repository

Choose a project where:

- documentation exists but is uneven
- the codebase has multiple eras or styles
- naming, boundaries, or truth surfaces are messier

This tests:

- whether the launch pack helps agents avoid expensive onboarding mistakes in a repo with too much context and uncertain freshness

## Task Selection

Choose short tasks only.

The evaluation should not begin with large implementation work. It should focus on tasks where wrong first turns matter most.

Good task types:

- investigate a bug
- explain a confusing behavior
- locate the subsystem responsible for a failure
- identify why a retry, refresh, or routing path behaves incorrectly

Avoid:

- broad refactors
- large feature implementation
- tasks requiring days of exploration

Use 3-5 tasks per repo if possible.

## Scenario Shape

Each task should naturally create one of these failure risks:

### 1. Wrong Subsystem Risk

The task language plausibly maps to multiple areas of the repo.

### 2. Stale Prose Risk

A relevant document exists but may no longer reflect implementation details.

### 3. Proof Path Risk

The likely code path is easy to find, but the nearest executable evidence is not obvious.

These scenario types match the current product thesis and should be explicitly represented in the evaluation.

## Evaluation Procedure

For each task:

1. Run with the strong launcher baseline.
2. Record:
   - time to first correct subsystem plus executable check
   - wrong turns
   - notable detours
   - whether the agent had to stitch together route, evidence, and warnings manually
3. Run with the task launch pack.
4. Record the same data.
5. Compare:
   - speed
   - wrong-turn reduction
   - stitching burden
   - whether the pack changed the first few actions in a useful way

If possible, use the same model/runtime and similar prompting style for both conditions.

## Minimum Viable Launch Pack

The evaluation should not wait for full automation or a complete system.

The minimum viable launch pack for testing is:

- a task statement
- repo rules or source-of-truth note
- likely subsystem
- likely code entrypoints
- nearest executable evidence
- one or two targeted warnings where relevant

That is enough to test the thesis.

## What We Are Not Testing Yet

This evaluation is not primarily testing:

- whether hot/warm/cold is the final retrieval architecture
- whether `llms.txt`-style lazy loading is the final interaction model
- whether usage-aware promotion works well
- whether a full metadata automation system is production-ready
- whether background review agents should run continuously

Those questions can come later.

The immediate question is simpler:

- is the launch pack a product-distinct and useful artifact?

## Success Criteria

The launch pack should be considered validated enough to move forward if it shows, across representative tasks:

- clear reduction in time to first correct subsystem plus executable check
- fewer wrong first turns
- lower stitching burden than the baseline
- consistent usefulness across both a rapid project and a legacy-style repo

It does not need to win perfectly on every task. It does need to show that the product distinction is real and not just naming or packaging.

## Failure Outcomes

The evaluation may show that:

### 1. The Strong Launcher Is Enough

If the strong launcher performs nearly as well, then the product artifact may be too thinly differentiated.

Implication:

- simplify the proposal
- treat launch packs as a presentation layer rather than a distinct product unit

### 2. The Launch Pack Helps, But Only In Legacy Repos

Implication:

- legacy onboarding may be the true wedge
- greenfield applicability may be secondary

### 3. The Launch Pack Helps, But Only With Good Metadata

Implication:

- metadata collection and drift support become higher-priority prerequisites

### 4. The Launch Pack Wins Clearly

Implication:

- the product artifact is real
- the next step should be building the thinnest system that can produce it reliably

## Recommended Next Deliverables

If this plan is accepted, the next concrete deliverables should be:

1. a short task set for one rapid-development project
2. a short task set for one inherited or legacy-style repo
3. a lightweight baseline launcher format
4. a lightweight launch-pack format
5. an evaluation worksheet for recording timings, wrong turns, and observations

## Recommendation

Do not spend the next cycle debating the proposal abstractly.

Use the current thesis to run a small, disciplined comparison:

- strong launcher
- versus launch pack
- on short tasks
- in one fast-moving repo and one messy legacy-style repo

That is the fastest way to learn whether the proposal is a useful product direction or just a well-argued concept.
