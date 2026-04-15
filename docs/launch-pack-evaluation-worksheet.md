# Launch Pack Evaluation Worksheet

## Purpose

Use this worksheet to compare:

- the strong launcher baseline
- the task launch pack

for a short task in a real repository.

The main question is:

**does the launch pack get the agent to the first correct subsystem plus executable check faster and with fewer wrong turns?**

## Evaluation Metadata

- repo:
  - <repo name>
- task:
  - <short task description>
- task type:
  - <bug-fix / investigation / explanation>
- evaluator:
  - <name>
- date:
  - <date>

## Expected Correct Working Area

- correct subsystem:
  - <name/path>
- nearest executable check:
  - <test / command / script / fixture / repro path>

## Condition A: Strong Launcher

### Artifact Used

- file or prompt:
  - <launcher artifact>

### Observations

- first subsystem chosen:
  - <what the agent went to first>
- first executable check chosen:
  - <what it used first>
- wrong turns:
  - <count and brief notes>
- stale-prose detours:
  - <count and brief notes>
- irrelevant files/docs opened before correct working area:
  - <count>
- time to first correct subsystem:
  - <time>
- time to first correct subsystem plus executable check:
  - <time>

### Stitching Burden

- did the agent have to stitch route, evidence, and warnings together itself?
  - yes / no / partly
- notes:
  - <brief explanation>

## Condition B: Task Launch Pack

### Artifact Used

- file or prompt:
  - <launch pack artifact>

### Observations

- first subsystem chosen:
  - <what the agent went to first>
- first executable check chosen:
  - <what it used first>
- wrong turns:
  - <count and brief notes>
- stale-prose detours:
  - <count and brief notes>
- irrelevant files/docs opened before correct working area:
  - <count>
- time to first correct subsystem:
  - <time>
- time to first correct subsystem plus executable check:
  - <time>

### Stitching Burden

- did the agent have to stitch route, evidence, and warnings together itself?
  - yes / no / partly
- notes:
  - <brief explanation>

## Comparison

- which reached the correct subsystem faster?
  - <launcher / launch pack / tie>
- which reached the executable check faster?
  - <launcher / launch pack / tie>
- which caused fewer wrong turns?
  - <launcher / launch pack / tie>
- which reduced stitching burden more?
  - <launcher / launch pack / tie>
- was the launch pack materially better?
  - yes / no / unclear

## Qualitative Notes

- what helped most:
  - <notes>
- what still failed:
  - <notes>
- was the pack genuinely distinct from the launcher?
  - <notes>

## Decision

- outcome:
  - validate / mixed / not validated
- why:
  - <brief explanation>

## Follow-Up

- changes to launcher:
  - <notes>
- changes to launch pack:
  - <notes>
- changes to evaluation method:
  - <notes>
