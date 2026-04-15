# Penfold Launch Pack Evaluation Worksheet

## Evaluation Metadata

- repo:
  - Penfold
- task:
  - Investigate why digest behavior differs between search results, scheduled digests, and journal digest output
- task type:
  - investigation / routing question
- evaluator:
  - <fill in>
- runtime:
  - Claude Code
- date:
  - <fill in>

## Expected Correct Working Area

- correct subsystem:
  - digest domain plus the right digest-related e2e proof surface
- nearest executable check:
  - whichever of `digest_test.go`, `digest_search_test.go`, `journal_digest_test.go`, or `scheduled_digest_test.go` most directly matches the reported behavior difference

## Condition A: Normal Claude Code Workflow

### Artifact Used

- file or prompt:
  - none beyond normal Penfold repo context

### Observations

- first subsystem chosen:
  - cross-subsystem digest behavior with an early emphasis on the search integration point
- first executable check chosen:
  - digest-related e2e tests, especially `digest_search_test.go`, with recognition that the proof surface was close but incomplete
- wrong turns:
  - 1 moderate wrong turn: Claude Code had to map the digest landscape itself before deciding whether search, worker workflows, or shared digest code was the best first place to look
- stale or misleading prose detours:
  - 0
- irrelevant files/docs opened before correct working area:
  - 0 significant irrelevant reads visible in the log
- time to first correct subsystem:
  - roughly moderate; it reached the right digest cluster quickly but still spent time inferring the taxonomy before choosing the integration point
- time to first correct subsystem plus executable check:
  - moderate; it identified the nearest proof surface but also identified a gap in it

### Stitching Burden

- did Claude Code have to stitch route, evidence, and warnings together itself?
  - yes
- notes:
  - Claude Code did well, but it had to derive the digest taxonomy and compare multiple surfaces before concluding that search integration was the best first place to look.

## Condition B: Strong Launcher

### Artifact Used

- file or prompt:
  - `01-digest-behavior-routing-launcher.md`

### Observations

- first subsystem chosen:
  - Temporal workflow divergence in `services/worker/workflows/`
- first executable check chosen:
  - the four digest-related e2e test families, treated as the nearest proof surface for daily, weekly, journal, and search behavior
- wrong turns:
  - 0 meaningful wrong turns
- stale or misleading prose detours:
  - 0
- irrelevant files/docs opened before correct working area:
  - 0
- time to first correct subsystem:
  - faster than condition A
- time to first correct subsystem plus executable check:
  - faster than condition A

### Stitching Burden

- did Claude Code have to stitch route, evidence, and warnings together itself?
  - partly
- notes:
  - The launcher substantially improved the first turn. Claude Code moved quickly into the workflow layer and explicitly separated daily, weekly, and journal behavior. It still had to synthesize the exact product-level distinction itself, but much less manual mapping was required.

## Condition C: Task Launch Pack

### Artifact Used

- file or prompt:
  - `01-digest-behavior-routing-launch-pack.md`

### Observations

- first subsystem chosen:
  - `pkg/digest/`, especially `gather.go` and `repository.go`, paired with the digest e2e test family as the comparative proof surface
- first executable check chosen:
  - the four digest e2e tests together, treated as the behavioral contract for daily, search, journal, and scheduled digest surfaces
- wrong turns:
  - 0 meaningful wrong turns
- stale or misleading prose detours:
  - 0
- irrelevant files/docs opened before correct working area:
  - 0
- time to first correct subsystem:
  - fastest or tied-fastest; Claude Code immediately treated `pkg/digest/` as the right starting point and did not spend time rediscovering the route
- time to first correct subsystem plus executable check:
  - fastest; the launch pack moved directly into the comparative proof surface and core shared implementation

### Stitching Burden

- did Claude Code have to stitch route, evidence, and warnings together itself?
  - only a little
- notes:
  - The launch pack changed the shape of the investigation. Claude Code started from the side-by-side e2e proof surface and the shared digest package, produced a clean comparison of data source, gather path, idempotency, and prompt template, and only then proposed expanding into workflow dispatch. That is less stitching than condition A and slightly less than condition B.

## Comparison

- which reached the correct subsystem fastest?
  - task launch pack, by a small margin
- which reached the executable check fastest?
  - task launch pack
- which caused fewer wrong turns?
  - strong launcher and task launch pack both avoided meaningful wrong turns, but the task launch pack required less self-assembly
- which reduced stitching burden most?
  - task launch pack
- was the launch pack materially better than normal Claude Code workflow?
  - yes
- was the launch pack materially better than the strong launcher?
  - yes, but narrowly; its value was in starting from the comparative proof surface and shared digest domain rather than routing through workflows first

## Qualitative Notes

- what helped most:
  - The launcher made the overloaded term "digest" less dangerous by explicitly naming digest domain code, search-related tests, journal tests, and scheduled tests up front.
- what still failed:
  - The launcher still did not fully pre-assemble the exact comparative working set or state the one most likely proof path; Claude Code still had to synthesize some of that itself. The launch pack improved this, but did not eliminate all follow-on reasoning because workflow dispatch still remained as the next layer to inspect.
- did the launch pack produce a user-visible difference for Claude Code?
  - yes; Claude Code answered with a tighter comparative model of the digest surfaces, a clearer proof hierarchy, and a more direct next step

## Decision

- outcome:
  - launch pack wins, but modestly
- why:
  - Normal Claude Code was competent, and the strong launcher already improved routing substantially. The launch pack still added visible value because it pre-assembled the exact comparative proof surface and the shared digest domain files, which reduced the need for Claude Code to infer the digest taxonomy and working set on its own.
