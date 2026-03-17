# Context Compiler for Large Legacy Codebases

## Summary

The next major capability Context Palace should add for AI-agent development in large legacy systems is a **Context Compiler**: a system that assembles a task-specific context pack from the KB, codebase, tests, recent work, and runtime evidence, while also scoring how trustworthy each knowledge shard is.

This turns Context Palace from a static knowledge store into an **implementation-aware retrieval and trust system**.

The core idea is simple:

- shards remain the unit of human-curated knowledge
- the codebase remains the ultimate source of truth
- Context Palace continuously connects the two
- when an agent starts work, `cxp` compiles the smallest high-value context set for that task
- stale or drifting shards are explicitly flagged instead of being loaded as authoritative truth

This is especially valuable in large legacy codebases, where the main problem is rarely "no documentation exists." The real problem is usually:

- too much documentation
- multiple competing truth surfaces
- unclear ownership boundaries
- architecture drift
- agents trusting stale docs with high confidence

## Why This Matters

Hot/warm/cold KB architecture is the right retrieval model for agent work, but by itself it is not enough.

A large legacy project still suffers from a fundamental failure mode:

**the agent does not know which documents are still trustworthy.**

When a codebase has evolved for years, there may be:

- old architecture docs
- partial migrations from one subsystem to another
- stale diagrams
- superseded design intent
- tests that reflect newer reality than the docs
- runtime behavior that differs from both

Without a trust layer, the agent can load the "right" shard topologically and still be wrong.

The Context Compiler solves that by making retrieval **evidence-aware**.

Instead of saying:

> "Here are the matching shards."

it says:

> "Here are the matching shards, ordered by likely usefulness and current trustworthiness. These two are probably stale. These three are backed by recent code and tests. Here are the files and tests most likely to prove the current behavior."

That is the missing layer for AI-agent work in legacy systems.

## Core Concept

The Context Compiler has two jobs:

1. **Compile task-specific context**
2. **Score shard trust and freshness**

### 1. Compile task-specific context

For an incoming task such as:

- "investigate digest bug"
- "add OAuth provider"
- "fix retry behavior in sync pipeline"
- "understand model selection path"

the compiler should assemble a focused context pack containing:

- hot context: law + playbook
- matching warm branch shards
- relevant cold leaf shards
- related code entrypoints
- relevant tests
- recent design/task/bug shards
- optional runtime evidence such as logs, traces, migrations, or config references

### 2. Score shard trust and freshness

Each shard should be evaluated against the implementation it claims to describe.

The compiler should be able to answer:

- has the referenced code changed since this shard was updated?
- have relevant tests changed?
- have related migrations changed the schema?
- have config tables or runtime boundaries changed?
- does the shard still look like a stable concept article or does it contain volatile live values?

This produces a trust/freshness signal that helps agents decide what to rely on.

## The Big Shift

The innovative step here is that Context Palace stops being just:

- a wiki
- a note graph
- a work tracker

and becomes:

- a **verified context engine** for AI agents

That means Context Palace is not only storing knowledge. It is actively helping agents decide:

- what to load
- what to ignore
- what to distrust
- what to verify in code

## Proposed User Experience

### New Command

```bash
cxp context compile "investigate digest bug"
```

Possible output shape:

```text
Task: investigate digest bug

Hot
- AGENTS.md
- Penfold Playbook (pf-34494b)

Warm
- Search & Retrieval (high match, fresh)
- Infrastructure (medium match, fresh)

Cold
- Digest & Journal Workflow (high match, fresh)
- Scheduling Infrastructure (medium match, warning: possibly stale)
- Pipeline/Prompts (medium match, warning: contains volatile details)

Implementation
- services/gateway/digestservice/
- services/worker/workflows/
- pkg/digest/
- tests/e2e/digest_test.go
- migrations/115_digests.sql
- migrations/134_digest_routing_and_prompts.sql

Recent Work
- pf-XXXXX Digest scheduling cleanup
- pf-YYYYY Prompt routing fix

Warnings
- pf-669ed3 changed domain files have diverged since shard update
- pf-01581c appears to contain live values likely to drift
```

### Shard-Level Trust Signal

`cxp shard show pf-XXXXXX` could display:

- trust score
- freshness score
- last verified against implementation
- referenced files/tests
- likely drift reasons

For example:

```text
Trust: 0.82
Freshness: medium
Verification: code and tests changed in 3 referenced paths since last shard update
Suggested action: load for architecture, verify runtime values in code/tests
```

### Search Results with Trust Ranking

`cxp kb search` should optionally rank or annotate results by:

- semantic/keyword match
- trust/freshness
- recency of access
- evidence coverage

This prevents highly similar but stale docs from outranking smaller, fresher shards.

## Data Model

The compiler needs richer metadata about what a shard is connected to.

### Proposed shard metadata

Each knowledge shard should be able to declare or accumulate:

- `knowledge_kind`
  - `map`
  - `branch`
  - `concept`
  - `workflow`
  - `reference`
  - `runtime_reference`
  - `decision`
  - `transient_note`

- `references.files`
  - file paths or glob patterns

- `references.tests`
  - test files or test package paths

- `references.migrations`
  - migration files

- `references.tables`
  - database tables or config tables

- `references.services`
  - service names such as `gateway`, `worker`, `mcp`

- `references.commands`
  - CLI or operational commands that prove behavior

- `stability`
  - `stable`
  - `mixed`
  - `volatile`

- `verification_mode`
  - `concept_only`
  - `code_backed`
  - `code_and_tests`
  - `runtime_backed`

Not every field needs to be manually curated. Some can be user-authored, some can be inferred, and some can be attached over time.

## Drift Detection Model

The key to making this compelling is not perfect truth. It is **useful warning signals**.

The first version should be heuristic, not magical.

### Signals that a shard may have drifted

- referenced files changed after shard `updated_at`
- referenced tests changed after shard `updated_at`
- referenced migrations added or modified
- related files changed frequently but shard has low edit activity
- shard includes suspiciously volatile patterns such as:
  - exact model inventories
  - current timeout values
  - current schedule inventories
  - field-by-field schema dumps
  - environment-specific ports/hosts

### Signals that a shard is likely trustworthy

- recent updates after relevant code changes
- linked tests still present and passing
- referenced services/files still exist
- article is concept-oriented rather than value-dump-oriented
- high access rate plus periodic maintenance

### Output

This should not pretend to produce certainty. It should produce:

- `fresh`
- `possibly stale`
- `stale`
- `structurally volatile`

with reasons.

That is enough to steer agent behavior.

## How Context Compilation Would Work

### Inputs

- task text
- current repo
- current branch/worktree
- optional target files
- optional shard IDs already in context

### Retrieval pipeline

1. Load hot context
   - law files
   - root playbook

2. Match warm branches
   - trigger match
   - semantic match
   - file-pattern match if target files are known

3. Select cold leaf shards
   - based on semantic similarity
   - based on references to related files/tests/services
   - filtered by trust/freshness

4. Pull implementation anchors
   - files
   - tests
   - migrations
   - recent work shards

5. Rank and compress
   - include smallest set that maximizes likely value
   - annotate warnings for stale or volatile shards

6. Emit a context pack
   - structured JSON for agents
   - readable text output for humans

## Why This Is Accretive

This is a high-leverage feature because it compounds on top of the existing system instead of replacing it.

It does not require throwing away:

- shards
- typed edges
- triggers
- work items
- search

Instead it strengthens all of them.

### What it reuses

- shard graph
- KB tree
- edge metadata
- work item relationships
- search index
- access logs

### What it adds

- implementation references
- drift scoring
- trust annotations
- compiled context output

This makes it a natural next move for Context Palace rather than a side project.

## Why It Is Compelling for Legacy Codebases

Large legacy systems are where AI agents struggle most:

- architecture is split across eras
- code ownership is blurry
- docs contradict each other
- migrations tell one story, runtime behavior tells another
- agents waste time loading too much irrelevant context

The Context Compiler directly attacks these problems.

### Benefits

- lowers onboarding cost for new agents
- reduces over-trust in stale docs
- gives agents a practical starting point for bug work
- improves feature scoping by mapping task text to real subsystem boundaries
- encourages better KB hygiene because stale shards become visible
- creates a path from static documentation to evidence-backed operational memory

Most importantly, it changes the retrieval contract from:

> "Here is some documentation."

to:

> "Here is the most useful and most trustworthy working context we can assemble right now."

That is a much stronger product story.

## Suggested MVP

The MVP should be deliberately narrow and useful.

### MVP scope

1. Add optional shard metadata for:
   - files
   - tests
   - services
   - migrations
   - stability

2. Implement drift heuristics:
   - files changed since shard update
   - tests changed since shard update
   - migration changes since shard update

3. Add:

```bash
cxp context compile "<task>"
```

4. Return:
   - top matching shards
   - implementation anchors
   - trust/freshness warnings

5. Show trust annotations in:
   - `cxp shard show`
   - `cxp kb search`

This would already be genuinely useful.

## Example Future Extensions

- background indexing of file-to-shard relationships
- repo-specific compilers
- automated suggestions like:
  - "this shard is stale, update it?"
  - "this code area has no KB coverage"
- link passing test results to shard freshness
- runtime evidence connectors:
  - logs
  - traces
  - dashboards
  - migrations
- agent handoff packs:
  - compact context bundle for a worker agent

## Product Positioning

Context Palace can become the system that answers a problem most AI coding tools do not handle well:

**How do you let agents work safely and efficiently in a large, drifting, partially documented legacy codebase?**

Most tools offer:

- vector search
- code search
- long prompts
- wiki integration

The Context Compiler offers something more differentiated:

- **retrieval plus trust**
- **knowledge plus implementation awareness**
- **routing plus drift detection**

That is not just better memory. It is operational context infrastructure for software agents.

## Recommendation

If Context Palace wants one bold, high-leverage next step, it should build:

**`cxp context compile` backed by drift-scored implementation-aware shard retrieval.**

That is the feature most likely to make hot/warm/cold KB architecture truly work for AI-agent coding in large legacy systems.
