# Task Launch Pack: Penfold Digest Behavior Routing

## Task

```text
Investigate why digest behavior differs between search results, scheduled digests, and journal digest output.
```

## Primary Objective

- get to:
  - the first correct subsystem plus executable check for the digest behavior difference without spending the first turns in the wrong digest path

## Why This Pack Exists

- dominant risk:
  - wrong first turn into one digest surface when the behavior difference is actually between multiple digest surfaces
- why a thin launcher is incomplete here:
  - a launcher can identify digest-related code and tests, but still leave Claude Code to assemble how search, journal, scheduled generation, and core digest behavior relate

## Source Of Truth

- proof surfaces:
  - `pkg/digest/gather.go`
  - `pkg/digest/repository.go`
  - `tests/e2e/digest_test.go`
  - `tests/e2e/digest_search_test.go`
  - `tests/e2e/journal_digest_test.go`
  - `tests/e2e/scheduled_digest_test.go`
- prose role:
  - route into the right Penfold KB branch if needed, but do not treat prose as the answer

## First Credible Route

- likely subsystem:
  - digest domain plus e2e proof surfaces that split search, journal, and scheduled behavior
- likely code entrypoints:
  - `pkg/digest/gather.go`
  - `pkg/digest/repository.go`
- nearest executable evidence:
  - `tests/e2e/digest_test.go`
  - `tests/e2e/digest_search_test.go`
  - `tests/e2e/journal_digest_test.go`
  - `tests/e2e/scheduled_digest_test.go`

## Disambiguation

- ambiguity:
  - does the task concern search indexing of digests, digest generation workflow, journal-specific synthesis, or scheduled orchestration?
- how to disambiguate quickly:
  - compare the four e2e digest test families first, before diving too deep into implementation

## Trust Guidance

- likely current:
  - repo law, digest package code, and the digest-related e2e tests
- use with caution:
  - thin architecture prose and any broad digest design notes
- why:
  - this task is likely answered fastest from proof surfaces and only secondarily from prose

## Minimal Supporting Prose

- `README.md`
  - only for top-level repo orientation
- relevant KB shards
  - only if the task needs subsystem disambiguation beyond what the tests already show

## First Three Moves

1. inspect `tests/e2e/digest_test.go`, `digest_search_test.go`, `journal_digest_test.go`, and `scheduled_digest_test.go` together to map the behavior split
2. inspect `pkg/digest/gather.go` and `pkg/digest/repository.go` to see which behavior is shared versus specialized
3. only then expand into services, CLI paths, or KB routing if the difference still is not clear

## Stop Conditions

- if the behavior split is already clear from the e2e tests:
  - treat the issue as a test/proof-path routing problem first, not a general architecture problem
- if the shared digest package does not explain the difference:
  - pivot to the service/workflow path implied by the relevant test family

## Why This Should Beat The Launcher

- pre-assembled route:
  - it starts from the actual proof split, not just from the overloaded term "digest"
- earlier proof linkage:
  - it points Claude Code at the exact e2e surfaces most likely to disambiguate the task
- earlier trust calibration:
  - it reduces the chance of spending the first turns in broad KB or digest prose instead of executable evidence
