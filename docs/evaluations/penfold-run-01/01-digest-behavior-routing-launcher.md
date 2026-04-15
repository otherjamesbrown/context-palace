# Strong Launcher: Penfold Digest Behavior Routing

## Task

```text
Investigate why digest behavior differs between search results, scheduled digests, and journal digest output.
```

## Repo Rules And Source Of Truth

- rules:
  - follow Penfold repo law first
  - use Context Palace KB as the preferred architecture map when available
  - treat code, tests, and runtime behavior as the proof surfaces
- source-of-truth order:
  - `AGENTS.md`, `CLAUDE.md`, and repo law
  - KB playbook and relevant KB shards
  - code, tests, migrations, and runtime evidence
- workflow constraints:
  - this is a short-task investigation, not a broad architecture review

## Tiny Repo Map

- digest domain code:
  - `pkg/digest/`
  - likely relevant because digest gathering and repository behavior live here
- digest retrieval and behavior tests:
  - `tests/e2e/digest_test.go`
  - `tests/e2e/digest_search_test.go`
  - likely relevant because they prove different digest behaviors
- journal and scheduled digest tests:
  - `tests/e2e/journal_digest_test.go`
  - `tests/e2e/scheduled_digest_test.go`
  - likely relevant because the task explicitly spans journal and scheduled behavior

## Task Classification

- likely task type:
  - investigation / routing question
- likely subsystem candidates:
  - digest generation and retrieval
  - search over digests
  - scheduled workflow behavior
  - journal generation path
- main ambiguity:
  - "digest behavior" spans at least search, generation, retrieval, scheduling, and journal-specific behavior

## Likely Code Starting Points

- `pkg/digest/gather.go`
  - likely involved in digest content assembly
- `pkg/digest/repository.go`
  - likely involved in digest storage and retrieval
- digest-related services or CLI paths discovered through tests
  - likely relevant once the right behavior path is clearer

## Likely Tests Or Executable Checks

- `tests/e2e/digest_test.go`
  - daily digest generation, retrieval, listing, and idempotency
- `tests/e2e/digest_search_test.go`
  - digest search and search type filtering
- `tests/e2e/journal_digest_test.go`
  - journal generation and retrieval
- `tests/e2e/scheduled_digest_test.go`
  - scheduled generation behavior

## Relevant Prose Fallback

- `README.md`
  - confirms repo structure and retrieval order
- `ARCHITECTURE.md`
  - thin orientation only
- KB shards in Context Palace
  - likely needed for subsystem map if the task crosses search/retrieval and workflow boundaries

## Warnings

- "digest" is overloaded and likely to produce wrong subsystem turns
- tests are extensive, so the main risk is not lack of proof surface but choosing the wrong proof surface first
- Claude Code may do reasonably well here already, which is exactly why this is a good validation task

## Expected First Moves

1. inspect `pkg/digest/`
2. inspect `tests/e2e/digest_test.go`
3. inspect `tests/e2e/digest_search_test.go`, `journal_digest_test.go`, and `scheduled_digest_test.go`
