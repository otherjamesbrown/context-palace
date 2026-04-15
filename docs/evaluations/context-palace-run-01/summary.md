# Context Palace Evaluation Run 01 Summary

## Purpose

This summary captures what the first real launch-pack evaluations on Context Palace itself taught us.

The aim of this run was not to prove the full product. It was to test whether a **task launch pack** is meaningfully better than a **strong launcher** for short-task AI coding work in a fast-moving repository with light documentation and multiple product surfaces.

## Tasks Evaluated

### 1. KB Root Config After Init

Files:

- `01-kb-root-config-launcher.md`
- `01-kb-root-config-launch-pack.md`
- `01-kb-root-config-worksheet.md`

Main finding:

- the launch pack was meaningfully better than the launcher because it pre-assembled the actual proof path across:
  - `init` output
  - config loading precedence
  - KB command enforcement

The most important practical insight was:

- global legacy config fallback from `~/.cp/config.yaml` masked the missing project-local `knowledge_base.root`

That was exactly the kind of thing a strong launcher can leave an agent to discover the hard way.

### 2. TUI Launch Surface

Files:

- `04-tui-launch-surface-launcher.md`
- `04-tui-launch-surface-launch-pack.md`
- `04-tui-launch-surface-worksheet.md`

Main finding:

- the launch pack was meaningfully better because it made the product-level question explicit up front:
  - is the TUI already acting like a launch surface?

The most important practical insight was:

- `cxpv` already behaves like a **proto-launch surface**

It already bundles:

- board/work/KB entry points
- auto-discovered KB roots
- shard detail
- knowledge children and triggers
- access counts and recent access

Without the launch-pack framing, it would have been easier to treat the TUI as a secondary implementation area rather than as a real part of how agents and users can launch into work.

## What We Learned

### 1. The Launch Pack Difference Is Real

In both evaluated tasks, the launcher was not wrong. It identified the right general area.

But the launch pack reduced:

- stitching burden
- wrong first framing
- time to the decisive check

That suggests the product distinction is real:

- a launcher points to candidates
- a launch pack assembles the first credible working set

### 2. The Value Is Highest When The Repo Has Hidden Interaction Surfaces

The biggest win did not come from simple search acceleration.

It came from surfacing interactions that are easy to miss:

- project config vs global config fallback
- CLI surface vs TUI surface
- routing information vs proof information

That is promising because those are common sources of wasted time in real repositories.

### 3. Context Palace Itself Is A Good Test Case

This repo is:

- fast-moving
- only partially organized for agent onboarding
- split across CLI, TUI, KB, docs, and proposals

That makes it a realistic environment for testing whether launch-pack assembly reduces early mistakes.

## Limits Of This Run

This run is still small.

It does not prove:

- final retrieval architecture
- final metadata model
- final automation design
- broad product-market fit

It does suggest that the thesis is worth continuing to test.

## Recommendation

Continue.

The gains observed so far do not look like trivial packaging improvements.

They look like meaningful reductions in:

- wrong first turns
- hidden-surface misses
- manual stitching at the start of short tasks

The next sensible step is:

- run at least one more evaluation on knowledge-routing or future integration work
- then decide whether to prototype a minimal real launch-pack generator for Context Palace itself

## Current Judgment

At this stage, the work looks:

- more than marginal
- not yet fully proven
- promising enough to justify continued focused investment

The key is to stay disciplined:

- keep testing on real short tasks
- avoid overbuilding architecture before the product difference is clearer
- keep comparing against a strong launcher baseline
