# Penfold Evaluation Run 01

## Purpose

This run is designed to test the launch-pack thesis in a larger, more operational repository than Context Palace.

Penfold is a strong validation case because:

- it is substantially larger than Context Palace
- it already uses Context Palace KB shards as the preferred architecture map
- it has multiple runtime services and many shared packages
- it has extensive integration and e2e test surfaces
- it is actively used with Claude Code today

That last point matters a lot.

This run is not just testing whether a generic AI coding agent might benefit from a task launch pack. It is testing whether **Claude Code in a real repo** benefits from a pre-assembled launch artifact over its normal exploration flow.

## Validation Goal

The main question is:

**does a task launch pack materially improve Claude Code’s first turns on short Penfold tasks compared with the current strong launcher and normal repo exploration flow?**

This run also helps answer a narrower version of the broader scaling thesis:

- does the launch pack still add value once the repo is larger and the agent runtime is already strong?

It does **not** by itself prove the strongest version of the claim:

- that the value gap between a strong launcher and a task launch pack grows substantially as repository entropy, subsystem ambiguity, and stale context increase

## What This Run Will Compare

For each selected task, compare:

1. normal Claude Code workflow in Penfold
2. Claude Code given a strong launcher
3. Claude Code given a task launch pack

The strongest signal will come from tasks where:

- the wrong subsystem is easy to choose first
- the right proof surface is not obvious
- the KB is useful but not sufficient on its own
- short-task investigation quality matters more than long-form implementation planning

## Why Penfold Is Different From Context Palace

Context Palace helped validate the concept in a fast-moving, lightly organized repo.

Penfold adds pressures that make the thesis more serious:

- many services
- many tests
- KB-backed architecture routing
- multiple eras of implementation
- multiple user-facing and agent-facing surfaces
- a real currently used agent runtime

If the launch-pack idea helps here, that is much stronger evidence than an internal-only validation.

At the same time, Penfold is still not the final stress case for the thesis.

It is larger and more operational than Context Palace, but it is also:

- relatively coherent
- actively curated for agent use
- already equipped with KB routing and repo law
- not yet a high-entropy legacy system with deeply stale documentation and uneven proof discoverability

That means Penfold is best understood as a **middle-case validation repo**.

If launch packs help here, that is encouraging.

If we want to prove that launch-pack value grows with complexity, we still need a more legacy-like validation target or more legacy-like task slices.

## Suggested Next Steps

1. choose 2-3 short candidate tasks from `candidate-tasks.md`
2. run one task with normal Claude Code workflow
3. run the same task with a strong launcher
4. run the same task with a task launch pack
5. compare:
   - time to first correct subsystem
   - time to first executable check
   - wrong first turns
   - stale or misleading doc detours
   - stitching burden

## Files In This Folder

- `README.md`
  - this overview
- `summary.md`
  - current judgment from the completed evaluations in this run
- `claude-code-evaluation-plan.md`
  - how to frame the run when Claude Code is part of the validation
- `candidate-tasks.md`
  - first short tasks worth testing in Penfold
