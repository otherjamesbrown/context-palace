# KB Maintenance Guide

How to keep a knowledge base accurate over time using Context Palace's automated maintenance systems.

This is the third in a series of three guides:

1. **`kb-shard-architecture.md`** — Why KB shards work for agents. The theory: tiered retrieval, the spec→work→KB lifecycle, why shards beat flat files.
2. **`kb-authoring-guide.md`** — How to write and structure a KB. The practice: abstraction levels, article types, tree structure, bootstrap process, multi-agent authoring.
3. **This guide** — How to maintain a KB over time. The automation: merge-time sync, nightly drift detection, canary testing, weekly triage.

Read `kb-authoring-guide.md` first. The maintenance systems described here keep articles healthy — but the articles must exist and be well-structured for the automation to work.

## The Problem

KB articles go stale in three ways:

1. **Missed updates** — code changes merge without the KB being updated. A function is renamed, a column dropped, a stage replaced — the code moves, the article doesn't.
2. **Bad updates** — the KB is updated, but the update is inaccurate or incomplete. The writer agent hallucinates facts, or strips information that should still be true.
3. **Out-of-band changes** — some changes bypass the normal merge flow entirely. Direct DB edits, emergency hotfixes, config tweaks via psql, infrastructure migrations. No code PR means no merge-time trigger.

Manual review would catch some of these, but it doesn't scale. The maintenance system addresses all three failure modes with layered automation.

## System Overview

```
                    MERGE TIME                        SCHEDULED
                    ──────────                        ─────────
PR merged
  → kb-sync                                    03:00 daily
    → Writer proposes update                     → Drift scan
      → Layer 1: factcheck (deterministic)         re-checks all anchors
      → Layer 2: judge (semantic, cross-model)     against current state
        → pass → commit update
        → fail → rollback → log gap            06:00 daily
                                                 → Canary test
                                                   retrieval quality
                                                   end-to-end

                                               Monday 05:00
                                                 → Triage
                                                   reviews all gaps
                                                   proposes fixes
                                                   escalates recurring
                                                        ↓
                                               pf-kb-escalations
                                               (human reviews weekly)
```

All failure modes feed into a central **gaps shard**. The weekly triage reviews gaps and proposes actions. Unresolvable issues escalate to a human — the only manual touchpoint in the system.

## Merge-Time: kb-sync

When a PR merges through [CoBuild](https://github.com/otherjamesbrown/cobuild), the `kb-sync` phase runs automatically between `review` and `done`. It finds KB articles affected by the change and updates them through a 3-layer verification pipeline.

For detailed setup and configuration, see the [CoBuild kb-sync guide](https://github.com/otherjamesbrown/cobuild/blob/main/docs/guides/kb-sync.md).

### How it works

1. **Concept extraction** — extracts key concepts from the PR diff: file paths, function names, stage names, table names, config keys.
2. **Article matching** — finds affected KB articles via two signals:
   - Semantic search: `cxp kb search` with extracted concepts
   - Code-anchor scan: full-text match of file paths and function names against article content
3. **Writer (Layer 0)** — an LLM reads the PR diff, task spec, and current article content, then proposes a narrow, patch-style update. The writer targets specific sections — it does not rewrite the whole article.
4. **Factcheck (Layer 1)** — deterministic verification of machine-verifiable claims extracted from the proposed update. Checks file paths exist, function signatures match, DB tables and columns are real, config keys are present.
5. **Judge (Layer 2)** — a different-model-family LLM reviews the proposed update for accuracy, completeness, and gaps against the PR diff and old article content.
6. **Commit or rollback** — both layers pass: update committed via `cxp kb update`. Either layer fails: rollback, log details to gaps shard.

### What it catches

- Renamed functions, moved files, dropped columns, changed stage names (Layer 1)
- Inaccurate descriptions of what the code does (Layer 2)
- Information removed from the article that should still be true (Layer 2)
- Gaps where the PR introduced something the article doesn't mention (Layer 2)

### What it doesn't catch

- Changes that don't come through a CoBuild merge (see Drift Scan below)
- Entirely new subsystems with no existing article (kb-sync updates, it doesn't create)
- Environment-specific values like connection strings and hostnames (not in the factcheck verification table — see `kb-authoring-guide.md`, Anchors section)

### Cross-model verification

The writer and judge must use different model families (e.g., writer = Gemini, judge = Claude). Same-model review is blind to its own errors — the judge would share the writer's biases. If both routing rules point to the same family, kb-sync refuses to run.

### The gate is non-blocking

Even if all articles fail verification, the pipeline still advances to `done`. KB quality is tracked, not enforced. This is deliberate — a broken KB update should not block shipping code. The gap is logged, and the triage system handles it.

### Enabling kb-sync

Add to your project's `.cobuild/pipeline.yaml`:

```yaml
kb_sync:
    enabled: true
    root_article: pf-5ae167    # optional — scope to children of this root
```

See the [CoBuild kb-sync guide](https://github.com/otherjamesbrown/cobuild/blob/main/docs/guides/kb-sync.md) for full configuration, scoping options, CLI usage, and troubleshooting.

## Nightly: Drift Scan

**Schedule:** daily at 03:00

The drift scan re-verifies all KB articles against the current state of the system. It catches changes that bypassed the normal merge flow.

### How it works

1. Lists all open knowledge shards for the project
2. For each article, re-runs Layer 1 (factcheck) against the current codebase and database
3. Any claim that was valid when the article was written but is now broken → drift detected
4. Logs each drift to the gaps shard with category `drift-detected`

Additionally, on a rolling basis, it runs Layer 2 (semantic judge) on a small batch of articles per night (default: 5). This compares the article's current content against its previous version — catching slow interpretation drift that anchor checks alone miss.

### What it catches

- Out-of-band code changes that didn't go through CoBuild
- Direct DB edits (table/column changes)
- Config tweaks via psql
- Force-pushes to main that bypass the pipeline
- Silent renames during refactors
- Infrastructure changes that break file path references

### What it doesn't catch

- New subsystems with no article (there's nothing to scan)
- Environment-specific values (connection strings, hostnames — not in the factcheck table)
- Conceptual staleness where the anchors are fine but the explanation is wrong (partially caught by the rolling semantic check, but only for articles that come up in the rotation)

### Configuration

Drift scan is configured per-project when subscribing to KB maintenance:

```bash
cxp schedule create drift-scan --cron "0 3 * * *" \
  --config '{"repo_path": "/path/to/repo", "judge_articles_per_run": 5}'
```

| Config key | Default | Purpose |
|-----------|---------|---------|
| `repo_path` | (required) | Path to the project's git repo for file-based checks |
| `factcheck_model` | `gemini/gemini-2.0-flash` | Model for claim extraction |
| `judge_model` | `claude/claude-haiku-4-5` | Model for semantic review (must differ from factcheck family) |
| `judge_articles_per_run` | 5 | Articles to semantic-check per nightly run |

## Daily: Canary Testing

**Schedule:** daily at 06:00

Canary testing verifies retrieval quality end-to-end. It answers the question: can an agent actually find the right knowledge using the KB tools?

### How it works

1. Loads canary questions from the project's canary shard (e.g., `pf-kb-canaries`)
2. For each question, dispatches a fresh agent with no prior context
3. The agent has access to only two tools: `cxp kb search` and `cxp kb show`
4. The agent answers the question using only KB retrieval
5. A verifier checks the response contains the expected facts (substring match)
6. Failures are logged to the gaps shard with category `retrieval-failure`

### What a failure means

A canary failure tells you one of three things:

| Failure type | What's wrong | Fix |
|-------------|-------------|-----|
| KB has the info but search can't surface it | Search tuning issue — embeddings, keywords, or article structure | Revise article title, content, or trigger text to improve searchability |
| KB is missing the info entirely | Content gap | Write a new article |
| KB has the info but the agent can't parse it | Structure issue — article too dense, poorly organized | Restructure the article, split if needed |

All three are actionable. The gap entry includes the failing question, the agent's actual response, and the expected facts — enough to diagnose which category the failure falls into.

### Writing canary questions

Canary questions live in a dedicated shard (see meta-KB shards in `kb-authoring-guide.md`). Format:

```yaml
- q: "What model does the classify_project stage use?"
  expected_facts: ["gemini-2.5-flash", "classify_project", "ai_routing_rules"]
  source_kb: pf-861f0c

- q: "Why does classify_project run for all content regardless of triage skip decisions?"
  expected_facts: ["skip_when_low", "PERSONAL", "project attribution"]
  source_kb: pf-d7b678
```

Each question targets a specific article. Good canary questions:

- Cover the most important articles (high access count, critical subsystems)
- Test both keyword and semantic retrieval (some questions should use the exact terms in the article, others should use natural language)
- Include 2-4 expected facts per question — enough to verify the right article was found without being so specific that minor rewording causes a false failure
- Are updated after any KB renovation or major restructuring

### Configuration

```bash
cxp schedule create canary --cron "0 6 * * *" \
  --config '{"canary_shard": "pf-kb-canaries", "agent_model": "gemini/gemini-2.5-flash"}'
```

## Weekly: Triage

**Schedule:** Monday at 05:00

Triage is the intelligence layer — it reads all accumulated gaps, identifies patterns, and proposes actions. This is how the maintenance system learns from its own failures.

### How it works

1. Reads the gaps shard and filters to entries since the last triage run
2. Groups by category:
   - `hallucination` — Layer 1 caught a wrong claim in a proposed update
   - `omission` — Layer 2 caught missing information in a proposed update
   - `drift-detected` — nightly scan found an anchor that no longer exists
   - `retrieval-failure` — canary question returned the wrong article or missed expected facts
   - `coverage-hole` — an agent logged `/kb-gaps` because it couldn't find what it needed
3. For each group, proposes actions:
   - Coverage holes → propose new KB article with an outline
   - Drift patterns → investigate if a class of changes needs new kb-sync triggers
   - Retrieval failures → propose search tuning or article restructure
   - Repeated hallucinations in same area → suggest the writer agent needs better source material
4. Opens tasks via `cxp task create` for each proposed action
5. Checks for recurring gaps (3+ occurrences of the same issue) → escalates to the escalation shard
6. Creates a `KB Triage Report YYYY-MM-DD` shard as a record

### Escalation

When a gap is logged 3+ times without resolution, it moves to the escalation shard — the **only human touchpoint** in the entire system.

Escalation criteria:
- Same gap logged 3+ times without agent resolution
- Layer 1 repeatedly failing on the same claim type across multiple articles (suggests the extraction prompt is wrong, not the articles)
- Canary retrieval failing after multiple tuning attempts
- Triage agent explicitly marks a gap as "can't resolve"

The escalation shard is reviewed weekly by a human, who decides:
- Fix the underlying issue manually
- Update the maintenance system (e.g., fix the factcheck prompt)
- Close as won't-fix
- Escalate to a design if the issue is systemic

### Configuration

```bash
cxp schedule create triage --cron "0 5 * * MON" \
  --config '{"gaps_shard": "pf-kb-gaps", "escalations_shard": "pf-kb-escalations"}'
```

## The Gaps Shard

The gaps shard is the central nervous system of the maintenance system. Every failure mode feeds into it:

| Source | Category | Example |
|--------|----------|---------|
| kb-sync Layer 1 | `hallucination` | Proposed update referenced `services/worker/classify.go` but file doesn't exist |
| kb-sync Layer 2 | `omission` | Proposed update removed the retry behavior section that's still valid |
| Drift scan | `drift-detected` | Article says `attribute_project` function exists but it was renamed to `classify_project` |
| Canary test | `retrieval-failure` | Q: "How does model selection work?" — expected article pf-861f0c but agent found pf-bc537d |
| Agent feedback | `coverage-hole` | Agent logged `/kb-gaps "no article covering the new webhook adapter"` |

Entry format:
```
date | source-work-item | category | description
```

The gaps shard is append-only — entries are never removed, only processed by triage. This preserves the full history of KB failures, which is valuable data for understanding where agent-maintained knowledge systems break down.

## Getting Started

### For projects using CoBuild

If your project already uses CoBuild for pipeline automation:

1. **Enable kb-sync** — add `kb_sync: { enabled: true }` to `.cobuild/pipeline.yaml`. This gives you merge-time verification immediately.
2. **Create the gaps shard** — `cxp shard create --type knowledge --title "KB Gaps Tracker" --body "# KB Gaps"`. Without this, verification failures are logged to stdout only.
3. **Subscribe to scheduled maintenance** — when available via CP scheduler:
   ```bash
   cxp schedule init --repo-path /path/to/repo
   ```
   This creates drift scan, canary, and triage schedules with sensible defaults, and creates the gaps/canaries/escalations shards if they don't exist.
4. **Seed canary questions** — write 20-30 questions covering your most important KB articles.

### For projects not using CoBuild

The scheduled workflows (drift scan, canary, triage) operate independently of CoBuild — they use `cxp` commands and don't require a CoBuild pipeline. You lose merge-time kb-sync, but you still get:

- Nightly detection of anchor rot and out-of-band drift
- Daily retrieval quality testing
- Weekly gap triage and escalation

Subscribe via `cxp schedule init` once the CP scheduler is available.

### Minimum viable maintenance

If you want to start with the smallest useful setup:

1. Enable kb-sync (if using CoBuild) — this is the highest-value, lowest-effort component
2. Create a gaps shard — so failures are tracked, not lost
3. Manually run a drift scan periodically — `cxp schedule run drift-scan` works without the daemon

Add canary testing and automated triage when your KB has enough articles (10+) to justify the infrastructure.

## What the Maintenance System Does NOT Do

- **Create new articles.** The system maintains existing articles. New article creation is triggered by gap logging and triage — the triage agent proposes new articles, but a human or author agent writes them.
- **Enforce KB quality as a merge gate.** kb-sync verdicts are non-blocking. A failed KB update does not prevent code from shipping.
- **Catch environment-specific drift.** Connection strings, hostnames, port numbers, and specific IDs are not in the factcheck verification table. Use the companion shard pattern (see `kb-authoring-guide.md`) to manage volatile operational values.
- **Replace human judgment on article structure.** The system verifies facts and retrieval quality, not whether an article is well-organized or at the right abstraction level.
- **Monitor articles that don't exist.** If a subsystem has no KB coverage, there's nothing to drift-scan or canary-test. Gap logging (`/kb-gaps`) and triage handle the discovery of missing coverage over time.

## Relationship to Other Guides

| Guide | Question it answers |
|-------|-------------------|
| `kb-shard-architecture.md` | Why use KB shards? What's the theory? |
| `kb-authoring-guide.md` | How do I write and structure KB articles? |
| This guide | How do I keep them accurate over time? |

The authoring guide tells you how to write articles that age well — good abstraction levels, stable anchors, proper tree structure. This guide describes the automated systems that catch the inevitable drift. The architecture doc explains the theoretical foundation underlying both.
