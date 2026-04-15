# Context Palace Evaluation Run 01

## Purpose

This folder contains the first real evaluation set for Context Palace itself.

The goal is to test whether a **task launch pack** is materially better than a **strong launcher** for short-task AI coding work in this fast-moving repo.

The three chosen tasks are intentionally practical:

1. investigate why `cxp kb search` still needs KB root configuration after `cxp init`
2. investigate how KB tree trigger metadata is stored and surfaced for navigation
3. investigate where a future `cxp context compile` feature should hook into the current CLI
4. investigate how the TUI already acts as a launch surface for navigating shards and knowledge

These tasks cover three useful failure risks:

- config and precedence ambiguity
- knowledge-routing and documentation-organization ambiguity
- roadmap/integration ambiguity
- product-surface ambiguity between CLI-first and TUI-first interaction

## Files

For each task there are three files:

- `*-launcher.md`
  - the strongest simpler baseline
- `*-launch-pack.md`
  - the candidate task-launch artifact
- `*-worksheet.md`
  - the evaluation worksheet for comparing both

## Suggested Use

1. Take one task.
2. Use the launcher artifact first.
3. Record observations in the worksheet.
4. Use the launch-pack artifact.
5. Record observations again.
6. Compare time to first correct subsystem plus executable check, wrong turns, and stitching burden.

## Why These Tasks

This repo currently has:

- meaningful CLI surface area
- knowledge-base and work-tracking concepts
- multiple documentation surfaces
- active product evolution
- relatively light agent-facing repo organization

That makes it a good test case for the proposal:

- there is enough structure to navigate
- there is enough ambiguity to make wrong first turns plausible
- there is not yet enough clean documentation to make the answer obvious
- there is both a CLI and a TUI, which means an agent can miss an important interaction surface if it looks only at commands and docs
