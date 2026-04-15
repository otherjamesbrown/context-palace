# Penfold Run 01 Candidate Tasks

## Selection Criteria

These tasks are chosen to stress the kinds of failures that a launch pack is supposed to reduce:

- wrong subsystem first
- KB guidance that still requires stitching into code and tests
- unclear nearest executable evidence
- service/package/test surface ambiguity

Each task is intentionally short-form and investigation-oriented.

## Task 1: Digest Behavior Routing

### Task

```text
Investigate why digest behavior differs between search results, scheduled digests, and journal digest output.
```

### Why This Is Good

- "digest" can plausibly route into multiple areas
- there are likely several relevant services, packages, and tests
- the nearest executable proof surface may not be obvious immediately

### Likely Wrong First Turns

- entering search code first when the issue is scheduling or digest assembly
- entering worker scheduling first when the issue is query/retrieval behavior
- finding digest code but missing the closest e2e proof surface

### Likely Areas

- `pkg/digest/`
- `tests/e2e/digest_test.go`
- `tests/e2e/journal_digest_test.go`
- `tests/e2e/scheduled_digest_test.go`
- search-related packages and services

### Why It Matters

This is a strong test of whether the launch pack can disambiguate a familiar but overloaded term and connect it to the nearest executable evidence.

## Task 2: Model Routing Choice

### Task

```text
Investigate why the system chose a particular model or provider for a classification or enrichment path.
```

### Why This Is Good

- model routing can span config, services, AI coordination, and pipeline behavior
- the KB likely helps with architecture terms, but the proof path is probably distributed
- this tests whether the launch pack helps turn "AI & Models" guidance into a real code/test route

### Likely Wrong First Turns

- starting in generic AI client code instead of the actual routing/config path
- reading docs/specs before locating the runtime decision path
- finding the provider client but not the test or config surface that proves the selection

### Likely Areas

- `services/ai/`
- `pkg/ai/`
- `pkg/models/`
- config and routing-related tests
- e2e tests for model management and per-stage model behavior

### Why It Matters

This is a good validation of whether the launch pack helps on configuration-and-behavior questions that are easy to misroute.

## Task 3: MCP Surface Versus Backend Route

### Task

```text
Investigate how an MCP-facing capability maps back to the underlying Penfold service and proof surface.
```

### Why This Is Good

- the repo has an explicit MCP surface
- an agent could easily stay in `services/mcp/` too long without finding the real backend path
- this tests whether the launch pack can help distinguish interface surface from underlying implementation surface

### Likely Wrong First Turns

- staying in MCP toolset code when the behavior is actually in search, workflow, or gateway code
- treating tool schemas as proof instead of finding the closest executable test
- missing service boundaries between MCP and underlying runtime components

### Likely Areas

- `services/mcp/`
- `services/gateway/`
- shared packages under `pkg/`
- MCP toolset tests
- integration or e2e tests related to search or workflow behavior

### Why It Matters

This is a strong task for evaluating whether the launch pack helps Claude Code bridge from surface interface to actual proof path.

## Suggested Order

If only one task is run first, start with:

1. Task 1: Digest Behavior Routing

It seems most likely to produce:

- subsystem ambiguity
- proof-path ambiguity
- KB-to-code stitching burden

If two tasks are run:

1. Task 1: Digest Behavior Routing
2. Task 3: MCP Surface Versus Backend Route

That combination gives one behavior-routing task and one interface-boundary task, which should provide a strong first signal.
