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
2. For each article, extracts claims two ways:
   - Regex-based pass for file paths and function names (always on)
   - LLM-based pass via `claim_extraction_model` for all seven claim types (below) — skipped silently if the model is unavailable
3. Verifies each claim against the relevant source:
   - `file_path` via `git ls-files`
   - `function_name`, `type_name` via `git grep`
   - `db_table`, `db_column` via `information_schema` (requires `db_conn_str`)
   - `config_key` via a project-supplied query (`config_key_query` + `db_conn_str`; skipped if either is absent)
   - `shard_id` via `cxp shard show`
4. Broken claims are appended to the gaps shard with category `drift-detected`.
5. **Layer 2 semantic judge (rolling):** on every run, the N articles with the oldest `last_semantic_check_at` metadata go through a second pass. The judge LLM (different model family from the extractor) compares the article's current content against its previous version via `cxp knowledge diff` and classifies any drift as `ok`, `drift_interpretation`, `drift_coverage`, or `drift_scope`. Non-`ok` findings are appended to the gaps shard with category `semantic-drift`. Each reviewed article's `last_semantic_check_at` is updated so the rotation moves on.

### What the factchecker verifies

| Type | How it's checked |
|------|---------------|
| `file_path` | `git ls-files` |
| `function_name` | `git grep` for `func|fn|def <name>` |
| `type_name` | `git grep` for `type <name>` |
| `db_table` | `information_schema.tables` |
| `db_column` | `information_schema.columns` |
| `config_key` | Project-configured SQL query |
| `shard_id` | `cxp shard show` |

### What it catches

- File path rot — files moved, renamed, or deleted
- Function-name, type-name, DB table and column renames or deletions
- Config keys and shard IDs that no longer exist
- Conceptual staleness caught by the rolling semantic judge — incorrect interpretation, removed coverage, or drifted scope

### What it doesn't catch

- New subsystems with no article (there's nothing to scan)
- Environment-specific values (connection strings, hostnames — not in the verification table)
- Semantic issues in articles that haven't come up in the judge rotation yet

### Configuration

Drift scan is configured per-project when subscribing to KB maintenance:

```bash
cxp schedule create drift-scan --cron "0 3 * * *" \
  --config '{"repo_path": "/path/to/repo", "gaps_shard": "pf-kb-gaps"}'
```

| Config key | Default | Purpose |
|-----------|---------|---------|
| `repo_path` | (required) | Path to the project's git repo for file-based checks |
| `gaps_shard` | (required) | Shard ID where drift findings are logged |
| `claim_extraction_model` | `gemini/gemini-2.0-flash` | Model for LLM claim extraction (falls back to regex-only if unavailable) |
| `max_claims_per_article` | `50` | Cap on LLM-extracted claims per article |
| `db_conn_str` | (optional) | libpq connection string used for `db_table`, `db_column`, `config_key` checks |
| `config_key_query` | (optional) | Base SQL query for `config_key` verification; the value is appended as `= '<value>'` |
| `judge_model` | `claude/claude-haiku-4-5` | Model for the rolling semantic judge (must differ from claim_extraction_model family) |
| `judge_articles_per_run` | `5` | Articles to semantic-check per run; set to `-1` to disable Layer 2 |

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

### Seeding canary questions

When a project first subscribes to the canary workflow, the canary shard is empty. Seed it with the built-in generic starter pack:

```bash
cxp schedule seed-canaries --shard <canary-shard-id>
```

This merges 13 generic questions covering common KB article shapes (playbook lookup, architecture, subsystem retrieval, config lookup, operational runbooks) into the shard. The operation is idempotent — rerunning it won't duplicate questions.

After seeding, review and customise the questions for your project:

```bash
# List current questions with validation flags
cxp schedule canary list --shard <canary-shard-id>

# Add a project-specific question
cxp schedule canary add \
  --question "What model does the classify_project stage use?" \
  --expected "gemini-2.5-flash,classify_project,ai_routing_rules" \
  --source pf-861f0c \
  --shard <canary-shard-id>
```

The `--shard` flag is optional if a "canary" schedule exists for the project — the commands auto-detect `canary_shard` from the schedule config.

**Question format** (stored in the canary shard as a fenced YAML block):

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
- Set `source_kb` to the shard ID of the article being tested — `cxp schedule canary list` flags questions missing this
- Are updated after any KB renovation or major restructuring

A generic starter template is available at `templates/canary-questions.yaml`.

### Configuration

```bash
cxp schedule create canary --cron "0 6 * * *" \
  --config '{"canary_shard": "pf-kb-canaries", "gaps_shard": "pf-kb-gaps"}'
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
3. **Subscribe to scheduled maintenance** — one command registers all three workflows and creates any missing singleton shards:
   ```bash
   cxp schedule init --repo-path /path/to/repo
   ```
   Expected output:
   ```
   Created shards: <prefix>-kb-gaps, <prefix>-kb-canaries, <prefix>-kb-escalations
   Registered schedules:
     drift-scan  cron "0 3 * * *"   → <prefix>-kb-gaps
     canary      cron "0 6 * * *"   → <prefix>-kb-canaries
     triage      cron "0 5 * * MON" → <prefix>-kb-gaps, <prefix>-kb-escalations
   ```
   Running `cxp schedule init` again is a no-op — existing shards and schedules are detected and reused, not duplicated. Use `--dry-run` to preview the plan without writing.
4. **Start the daemon** — schedules only fire automatically when the daemon is running:
   ```bash
   cxp daemon start    # foreground; install the service template for persistence
   ```
5. **Seed canary questions** — `cxp schedule seed-canaries` adds a generic starter pack. Then add project-specific questions with `cxp schedule canary add`, or hand-write 20-30 questions covering your most important KB articles.

### For projects not using CoBuild

The scheduled workflows (drift scan, canary, triage) operate independently of CoBuild — they use `cxp` commands and don't require a CoBuild pipeline. You lose merge-time kb-sync, but you still get:

- Nightly detection of anchor rot and out-of-band drift
- Daily retrieval quality testing
- Weekly gap triage and escalation

Subscribe via `cxp schedule init --repo-path /path/to/repo`. Then start the daemon so schedules fire automatically:

```bash
cxp daemon start          # foreground — run inside tmux or as a service
cxp daemon status         # check running state and PID
```

See `docs/scheduler/` for launchd (macOS) and systemd (Linux) service templates.

**Without the daemon**, schedules can still be triggered manually:
```bash
cxp schedule run drift-scan
```

### Minimum viable maintenance

If you want to start with the smallest useful setup:

1. Enable kb-sync (if using CoBuild) — this is the highest-value, lowest-effort component
2. Create a gaps shard — so failures are tracked, not lost
3. Manually run a drift scan periodically — `cxp schedule run drift-scan` works without the daemon

Add canary testing and automated triage when your KB has enough articles (10+) to justify the infrastructure. Once schedules are in place, start `cxp daemon start` (or install the service template) so they fire automatically.

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
