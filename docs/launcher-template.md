# Strong Launcher Template

## Purpose

This template defines the strongest simpler baseline for short-task AI coding work in an unfamiliar repository.

It is intentionally thin.

Its job is to help the agent start well without pre-assembling a full launch pack.

Use this template when comparing:

- a strong launcher baseline
- versus a task launch pack

## Task

```text
<short task description>
```

## Repo Rules And Source Of Truth

- rules:
  - <repo-level rules the agent must obey>
- source-of-truth order:
  - <for example: code/tests > recent migrations > KB/prose > older design docs>
- workflow constraints:
  - <review/test/deployment constraints>

## Tiny Repo Map

- area 1:
  - name: <subsystem or domain>
  - why it may matter: <one sentence>
- area 2:
  - name: <subsystem or domain>
  - why it may matter: <one sentence>
- area 3:
  - name: <subsystem or domain>
  - why it may matter: <one sentence>

Keep this to the smallest set of plausible domains.

## Task Classification

- likely task type:
  - <bug-fix / investigation / behavior explanation / routing question>
- likely subsystem candidates:
  - <candidate 1>
  - <candidate 2>
  - <candidate 3>
- main ambiguity:
  - <what term or concept could map to multiple areas?>

## Likely Code Starting Points

- path 1:
  - <file, package, or directory>
  - why: <one sentence>
- path 2:
  - <file, package, or directory>
  - why: <one sentence>
- path 3:
  - <file, package, or directory>
  - why: <one sentence>

## Likely Tests Or Executable Checks

- check 1:
  - <test file / command / script / fixture / repro path>
  - why: <one sentence>
- check 2:
  - <test file / command / script / fixture / repro path>
  - why: <one sentence>
- check 3:
  - <test file / command / script / fixture / repro path>
  - why: <one sentence>

## Relevant Prose Fallback

- note 1:
  - <doc or shard>
  - why it may help: <one sentence>
- note 2:
  - <doc or shard>
  - why it may help: <one sentence>

Only include prose that is likely relevant.

## Warnings

- warning 1:
  - <possible stale doc / ambiguous term / missing tests / risky assumption>
- warning 2:
  - <possible stale doc / ambiguous term / missing tests / risky assumption>

## Expected First Moves

1. <first move the agent should take>
2. <second move the agent should take>
3. <third move the agent should take>

## Notes

This launcher is meant to:

- reduce blind search
- improve first-step quality
- remain thin

It is not meant to:

- fully assemble the working set
- resolve ambiguity completely
- decide trust order dynamically
- pre-link all route, evidence, and warnings into one artifact
