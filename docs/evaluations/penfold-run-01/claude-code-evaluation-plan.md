# Claude Code Evaluation Plan For Penfold

## Purpose

This plan adapts the launch-pack evaluation approach for a repo that already uses Claude Code in practice.

The question is not just:

- would a launch pack help in theory?

It is:

- does a launch pack help **Claude Code** do better short-task work in Penfold?

That is a much stronger validation target.

## Core Thesis Under Test

The thesis is:

**for short bug-fix or investigation tasks in Penfold, Claude Code will reach the first correct subsystem plus executable check faster, with fewer wrong first turns, when given a task launch pack than when using its normal repo exploration flow alone.**

## Comparison Conditions

### A. Normal Claude Code Workflow

This is the real baseline.

Claude Code gets:

- the repo as it exists today
- existing repo law and docs
- existing KB references and runtime structure

It does not get a purpose-built launcher or launch pack beyond normal repo context.

### B. Strong Launcher

Claude Code gets:

- repo rules and source-of-truth guidance
- a tiny repo map
- likely subsystem candidates
- likely code paths
- likely tests or executable checks
- targeted warnings

This is the strongest simpler artifact.

### C. Task Launch Pack

Claude Code gets:

- the smallest credible working set for the first turn
- likely subsystem
- likely code entrypoints
- nearest executable evidence
- disambiguation guidance
- trust guidance about nearby docs or KB material

This is the candidate product artifact.

## Primary Metric

Use one primary metric:

**time to first correct subsystem plus executable check**

This is the cleanest operational test of whether the launch pack improves first turns.

## Secondary Metrics

- number of wrong-subsystem turns
- number of irrelevant files or docs opened before the first correct proof path
- number of misleading prose or KB detours
- time to first plausible explanation or fix hypothesis
- evaluator judgment of stitching burden

## Important Penfold-Specific Factors

Penfold is not just "a bigger repo." It has properties that make this validation more meaningful:

- KB shards are already part of the intended architecture map
- repo-local docs are intentionally thin
- services, workflows, packages, and migrations create multiple plausible first turns
- tests are extensive, but the nearest relevant executable evidence is not always obvious
- Claude Code is already in use, so the comparison is against a real interaction mode

## What To Watch Closely

The most important product question is:

**does the launch pack reduce Claude Code’s stitching burden enough to matter?**

In Penfold, that burden may show up as:

- opening the wrong service first
- following KB guidance without reaching the nearest executable check quickly
- finding the right code but the wrong test surface
- getting lost between `services/`, `pkg/`, `tests/`, `docs/`, and KB expectations

## Success Criteria

The launch-pack idea is validated more strongly if, in Penfold:

- Claude Code reaches the first correct subsystem plus executable check faster
- it makes fewer wrong first turns
- it spends less time stitching together route, evidence, and warnings
- the difference is visible on more than one task type

## Failure Cases

The evaluation should also be prepared to show:

### 1. Claude Code Already Does Almost As Well

If that happens, the product value may be marginal in repos like Penfold, or the launch pack may need to be much thinner.

### 2. The Launch Pack Only Helps On Certain Task Shapes

If that happens, the product wedge may be:

- cross-subsystem ambiguity
- stale-KB or stale-prose risk
- missing proof-path guidance

rather than all short tasks broadly.

### 3. The Launch Pack Helps Mainly Through Test And Evidence Routing

If that happens, the real product value may be more about **proof-path assembly** than about knowledge structure.

## Recommendation

Treat Penfold as the first external-quality validation environment for this idea.

If the launch pack helps Claude Code here, that is a strong signal that the artifact is meaningful beyond internal experimentation.
