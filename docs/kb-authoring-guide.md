# KB Authoring Guide

How to bootstrap and maintain a knowledge base for an AI-agent-operated project using Context Palace.

This guide is for project agents setting up a KB from scratch — typically when a repo has a README, some architecture docs, and scattered markdown files, but no structured KB tree yet. It is also the reference for ongoing KB authoring conventions once the tree exists.

This is the second in a series of three guides:

1. **`kb-shard-architecture.md`** — Why KB shards work for agents. The theory: tiered retrieval, the spec→work→KB lifecycle, why shards beat flat files.
2. **This guide** — How to write and structure a KB. The practice: abstraction levels, article types, tree structure, bootstrap process, multi-agent authoring.
3. **`kb-maintenance-guide.md`** — How to maintain a KB over time. The automation: merge-time sync, nightly drift detection, canary testing, weekly triage.

## Prerequisites

- Context Palace CLI (`cxp`) installed and configured (see `setup.md`)
- A registered project with an ID prefix (see the `projects` table)
- At least a README.md in the repo

## What the KB Is For

The KB exists so that coding agents can load the right context for their current task without scanning the entire codebase. It answers the question: **what does this system do, and where does the code live?**

It does not replace code. It does not replace git history. It does not replace inline comments. It is a navigational and conceptual layer that sits above the code and below the agent's operating instructions.

### The retrieval hierarchy

```
Hot  (always loaded)  →  Law (AGENTS.md / CLAUDE.md) + Map (playbook)
Warm (on-demand nav)  →  Branch nodes in the KB tree
Cold (on-demand load) →  Leaf articles in the KB tree
Search (fallback)     →  cxp kb search / cxp recall
Code (last resort)    →  grep, glob, sub-agent exploration
```

Every KB article you write should make this hierarchy work better. If an article doesn't help an agent find what it needs faster than scanning the code, it shouldn't exist.

## What Belongs in the KB

### Yes — KB-worthy concepts

| Concept | Why | Example |
|---------|-----|---------|
| Subsystems / services | Agents need to know what exists and what it does | "Ingest Pipeline", "Auth Service", "Billing Webhook Handler" |
| Data models and entities | Core domain objects that appear across multiple files | "Source", "Project", "AutomationRule", "Tenant" |
| Workflows and processes | Multi-step operations that span files or services | "Content ingestion flow", "Deployment process", "Model routing" |
| Integration points | Where your system talks to external systems | "Stripe webhook handler", "S3 upload path", "gRPC service boundary" |
| Configuration systems | Where config lives and how it's structured | "Pipeline operational config", "AI routing rules", "Feature flags" |
| Architecture decisions | Why the system is shaped the way it is | "Why we use event sourcing for X", "Why auth is middleware not a service" |
| Operational knowledge | How to run, debug, and monitor the system | "How to restart the worker", "Where logs go", "Alert thresholds" |

### No — not KB-worthy

| Concept | Why not | Where it lives instead |
|---------|---------|----------------------|
| Individual function implementations | Changes too fast, duplicates code | Code + inline comments |
| Line numbers or exact code snippets | Stale within hours | Code |
| Git history / who changed what | `git log` / `git blame` is authoritative | Git |
| Test fixture details | Only relevant when editing tests | Test files themselves |
| One-off scripts or migrations | Ephemeral, not reused | Code |
| TODO lists or in-progress work | Belongs in work tracking | Task/design shards |

## Abstraction Level

This is the most important decision in KB authoring. Write at the wrong level and the article either duplicates the code (too low) or tells the agent nothing useful (too high).

### The right level: functional + navigational

A good KB article answers:
1. **What does this subsystem do?** (functional purpose)
2. **Where does the code live?** (entry points, key files)
3. **What are the important concepts?** (domain entities, state transitions, data flow)
4. **What are the constraints?** (invariants, edge cases, things that must not break)
5. **What proves it works?** (test locations, verification commands)

### Too low — code duplication

Bad:
```markdown
The `ClassifyProject` function at `services/worker/activities/classify.go:47`
takes a `ClassifyInput` struct with fields `SourceID int64`, `Content string`,
and `TenantID int64`. It calls `llmClient.Complete()` on line 82 with the
prompt template from `prompt_templates` where stage = 'classify_project'.
The response is parsed on line 94...
```

This is just the code in prose form. It will be wrong after the next refactor.

### Too high — useless abstraction

Bad:
```markdown
The classification system assigns projects to content using AI.
```

This tells the agent nothing it couldn't guess from the function name.

### Right level — functional + navigational

Good:
```markdown
## Project Classification

Assigns ingested content to projects using LLM-based classification,
replacing the earlier keyword-based `attribute_project` approach.

**How it works:**
- The worker activity `ClassifyProject` sends content + tenant context to an LLM
- Model selection is controlled by `ai_routing_rules` (task_type: `classify_project`)
- Results are written to `sources.classified_project_ids`
- Classification runs as a pipeline stage after extraction and before enrichment

**Key files:**
- Activity: `services/worker/activities/classify.go`
- Stage config: `prompt_templates` table, stage `classify_project`
- Routing: `ai_routing_rules` table, task_type `classify_project`

**Constraints:**
- Must handle multi-project classification (a source can match multiple projects)
- Tenant context is required — classification without tenant context produces garbage
- Fallback model must be configured in routing rules or the stage fails hard

**Tests:** `services/worker/activities/classify_test.go`
```

This tells the agent what it does, where to find things, what matters, and what to test — without reproducing the code.

## Anchors

Anchors are the machine-verifiable references in a KB article: file paths, function names, table names, config keys. They serve two purposes:

1. **Navigation** — agents use them to jump to the right code
2. **Factcheck** — the automated Layer 1 verification checks that anchors still exist

### Anchor rules

- **Include anchors for key entry points**, not every file the subsystem touches. 3-6 anchors per article is typical.
- **Use the most stable identifier available.** A package path is more stable than a file path. A table name is more stable than a column name. A function name is more stable than a line number.
- **Never include line numbers.** They change on every commit.
- **Group anchors in a "Key files" or "Where the code lives" section** so they're easy to find and update.
- **Include the anchor type implicitly.** "Activity: `services/worker/activities/classify.go`" is better than just a bare path — it tells both the agent and the factchecker what kind of thing it is.

### What the factchecker verifies

The automated Layer 1 (`kb-factcheck`) extracts and checks anchors in two phases — v1 (shipped) and v2 (planned).

**Verified today (v1):**

| Type | What it checks |
|------|---------------|
| File paths | `git ls-files` — does the file exist? |
| Function names | `git grep "func <name>("` — does the function exist? |

**Verified when v2 ships (cp-165854):**

| Type | What it checks |
|------|---------------|
| Type names | `grep "type <name> "` — does the type exist? |
| DB tables | `information_schema.tables` — does the table exist? |
| DB columns | `information_schema.columns` — does the column exist? |
| Config keys | DB lookup — does the config key exist? |
| Shard IDs | `cxp shard show` — does the shard exist? |

Write anchors that target the full claim set above — articles with DB table names, type names, and config keys will be fully verified once v2 lands, and the anchors age well in the meantime. If a file path or function gets renamed today, the v1 factchecker catches it and the drift scan flags it.

### What the factchecker does NOT verify

The factchecker validates structural anchors — things that exist or don't exist in code or schema. It does **not** verify:

- Connection strings, hostnames, or URLs
- Specific IDs (e.g., `source_system_id: 47`, `repository_id: 12`)
- Port numbers, credentials, or environment-specific paths
- Threshold values, timeout durations, or numeric config

These are the fastest-rotting content in any KB. A server migration, a config change, or a new environment invalidates them silently and no automated check catches it.

**Rule: reference how to find the value, not the value itself.**

Bad:
```markdown
Database: `postgres://penfold:***@dev02.brown.chat:5432/penfold`
```

Good:
```markdown
Database URL: configured in `.env` as `DATABASE_URL`, or in the `connection` 
section of `.cxp.yaml`. See `infrastructure.md` for server details.
```

If a value must appear in a KB article (e.g., a well-known port that never changes), mark it explicitly as a hardcoded reference so reviewers and future authors know it's a drift risk.

## Entity Definitions

Entities are the core domain objects in your system — the nouns that appear in multiple places across the codebase. They deserve explicit definition in the KB because agents need a shared vocabulary.

### When to define an entity

- It has its own database table or persistent representation
- It appears in more than one subsystem
- Its meaning is not obvious from the name (or worse, its name is misleading)
- Agents have confused it with something else in the past

### What an entity definition should include

```markdown
## Source

A piece of ingested content — an article, document, or webpage.

**Identity:** `sources` table, primary key `id` (int64)
**Lifecycle:** created → classified → enriched → indexed
**Key fields:**
- `url` — the original URL (unique per tenant)
- `content_hash` — dedup key
- `classified_project_ids` — projects this source belongs to (set by classification)
- `tenant_id` — owning tenant (never null, never changes)

**Relationships:**
- belongs to a Tenant (1:many)
- classified into Projects (many:many via `classified_project_ids`)
- has Mentions (1:many, created during enrichment)

**Common confusion:**
- A Source is not a "document" in the search index sense — search documents
  are derived from Sources during indexing but have a different schema
```

### Where entity definitions live

There are two good options:

1. **Dedicated "Domain Model" branch** — a single branch with one leaf per major entity. Works well for projects with 5-15 core entities.
2. **Inline in subsystem articles** — define entities where they're most used. Works for projects where entities are strongly scoped to one subsystem.

Don't do both. Pick one and be consistent.

## Tree Structure

### The bootstrap tree

For a new project, start with this skeleton and adapt:

```
Playbook (root)
  ├── Architecture Overview
  │     What the system is, major components, how they connect
  ├── Domain Model
  │     Core entities, their relationships, lifecycle states
  ├── [Subsystem 1]
  │     What it does, key files, constraints, tests
  ├── [Subsystem 2]
  │     ...
  ├── Infrastructure
  │     Servers, deployment, config, monitoring
  └── How We Work
        Workflows, conventions, CI/CD, testing approach
```

### Labels

The tree structure organizes by subsystem, but some properties cut across subsystems — "this is operational," "this is volatile," "this is a maintenance artifact." Use labels for these.

Keep the label vocabulary small (5-10 labels) and document what each label means. Example set:

| Label | Meaning |
|-------|---------|
| `operations` | Operational runbooks and procedures |
| `reference` | Volatile reference shards (companion pattern) |
| `kb-maintenance` | Meta-KB shards (canaries, gaps, escalations) |
| `architecture` | Architecture decision records |

Labels are useful for:
- Filtering search results by category
- Identifying all operational content regardless of which branch it's under
- Tagging companion shards so agents know they're volatile
- Excluding maintenance shards from content searches

### Branch rules

- **3-10 top-level branches.** Fewer than 3 means the playbook isn't routing agents to specialised knowledge. More than 10 means the playbook is too long to scan quickly.
- **2-8 children per branch.** More than 8 means the branch is doing too much — split it.
- **Max depth: 3 levels** (playbook → branch → leaf). Deeper nesting means more navigation calls. If you need a 4th level, the branch node is probably too broad.
- **Leaf articles: 100-400 lines.** Under 100 is probably too thin to be worth a dedicated shard. Over 400 should usually be split — but if splitting would force agents to load 3+ shards to answer a single question, keep it together. Some subsystems (e.g., a pipeline with 6 stages that are only meaningful in comparison) are better as one large article than 6 thin ones.
- **Branch nodes: 50-100 lines.** These are indexes, not articles. They list children with triggers, plus a brief orientation paragraph.

### Trigger and description quality

Every parent-child edge needs a `trigger` and `description`. These are the shard's discoverability interface — without them, the shard is invisible to tree navigation.

**Trigger** answers: "Load this when you need to know about..."
**Description** answers: "This shard covers..."

Write them from the agent's perspective, using task language:

```bash
cxp knowledge children add <parent> <child> \
  --trigger "When working on content ingestion, pipeline stages, or stage ordering" \
  --description "The 6 ingest pipelines, stage definitions, execution order, retry behaviour"
```

Bad triggers: single words ("auth"), vague labels ("documentation"), taxonomy terms ("subsystem-3").

Good triggers: task phrases ("Need to debug why classification is returning wrong projects"), question language ("How does model selection work for enrichment stages?"), action language ("Adding a new pipeline stage").

## Bootstrap Process

How to build a KB tree for a project that has none.

### Step 1 — Inventory existing docs

Read what exists:
- README.md
- architecture.md or equivalent
- Any docs/ directory contents
- AGENTS.md / CLAUDE.md
- Inline documentation in key files

Don't scan the entire codebase yet. Start from what's already written.

### Step 2 — Identify top-level domains

From the existing docs, extract the major subsystems or domains. These become your top-level branches. Look for:

- Section headings in architecture docs
- Distinct services or packages
- Things that have their own config, their own tests, their own deployment
- Concepts that get explained separately

### Step 3 — Create the playbook

The playbook is the root knowledge shard. It should be ~100-200 lines and contain:

1. **One paragraph** saying what the project is
2. **The branch index** — each branch with its trigger and description
3. **Quick reference** — the 3-5 things agents ask for most often (DB connection, build command, test command, deploy command)

```bash
cxp knowledge create --title "Playbook" --body "$(cat playbook.md)" --root
```

### Step 4 — Create branch nodes (thin)

For each top-level domain, create a branch node. Don't write the full articles yet — just the branch node with its orientation paragraph and placeholder children.

```bash
cxp knowledge create --title "Ingest Pipeline" \
  --body "## Ingest Pipeline\n\nContent ingestion from source to index.\n\n**Children coming:** stage definitions, routing, retry" \
  --parent <playbook-id>
```

Set triggers on each branch immediately — they're not optional.

### Step 5 — Write leaf articles (depth-first)

Pick the subsystem that agents will work on most. Write its leaf articles first. Each leaf should follow the article template (see below).

Don't try to write all articles at once. Write them as they become relevant — when an agent needs to work in a subsystem and there's no KB article for it, that's the trigger to write one.

### Step 6 — Define core entities

Once you've written 3-4 subsystem articles, you'll have a clear picture of the core entities. Write entity definitions (either as a dedicated branch or inline) for any entity that appears in more than one article.

### Step 7 — Set up maintenance

Once the initial tree is populated:

1. Configure `kb-sync` in your CoBuild pipeline (if using CoBuild)
2. Seed canary questions for the most important articles
3. Enable the nightly drift scan

The maintenance system keeps the KB accurate after bootstrap. But it can only maintain articles that exist — it doesn't create new ones. Gap logging (`/kb-gaps`) and weekly triage handle the discovery of missing articles over time.

See `kb-maintenance-guide.md` for full details on the automated maintenance systems — kb-sync, drift scanning, canary testing, and weekly triage.

## Article Types

Not all KB articles serve the same purpose. The two main types have different structures, update patterns, and audiences.

### Reference articles (conceptual)

What a subsystem is, how it works, why it's shaped this way. These are the default — most KB articles are reference articles.

- **Update pattern:** changes when the code changes (caught by kb-sync at merge time)
- **Audience:** agents working on or with the subsystem
- **Staleness signal:** anchor rot (factchecker catches it)

Use the reference article template below.

### Companion pairs: stable + volatile

Some subsystems need both a stable reference (how things work) and a volatile reference (what the current values are). Split them into two sibling shards under the same branch. The stable shard links to the volatile one with a "for current values, see..." pointer. The volatile shard carries an explicit verification rule reminding agents not to trust its values blindly.

**When to use:** Any subsystem where agents regularly need current config values during design or review work, but those values change independently of the architecture. Typical examples: timeout keys, queue names, model assignments, concurrency limits, operational thresholds.

**Structure:**

The **stable shard** describes what the control surfaces are, why they exist, and how to reason about them. It changes rarely and is factcheckable. Example title: `Pipeline/Config & Operations`.

The **volatile companion** lists current values — the actual timeout keys, concurrency numbers, queue names. It changes frequently, is expected to drift, and must include an explicit verification rule:

```markdown
## Verification Rule
When exact runtime values matter, verify against:
- config tables
- current code
- service runtime state
- end-to-end tests where applicable
```

**Naming convention:** `[Subsystem]/Config & Operations` (stable) + `[Subsystem] Runtime Config Live Reference` (volatile).

**Labeling:** Tag volatile companions with a `reference` label so agents and maintenance workflows can identify them as high-drift content.

### Meta-KB operational shards

These shards track the health and coverage of the KB itself. They are not subject to the same authoring rules as reference articles — they are append-only logs or structured test fixtures.

**Required meta-KB shards:**

1. **Canary questions** — A set of retrieval test queries with expected facts and source article IDs. Used to verify that search returns the right articles. Re-seed after any renovation or major restructuring. Example format:
   ```yaml
   - q: "What model does the classify_project stage use?"
     expected_facts: ["gemini-2.5-flash", "classify_project", "ai_routing_rules"]
     source_kb: pf-861f0c
   ```

2. **Gap tracker** — Append-only log of KB failures. Categories: `hallucination` (factcheck caught a wrong claim), `omission` (judge found missing coverage), `drift-detected` (nightly scan found anchor rot), `retrieval-failure` (canary question returned wrong article), `coverage-hole` (agent couldn't find what it needed).

3. **Escalation queue** — Issues that require human review. Agents add entries; humans resolve them during weekly triage.

**Labeling:** Use a consistent label (e.g., `kb-maintenance`) so these shards are easy to find and exclude from content searches.

**Placement:** These shards should not be children of any content branch. Either make them top-level with a `kb-maintenance` label, or create a dedicated "KB Health" branch.

### Operational runbooks

How to deploy, troubleshoot, restart, or monitor a system. These contain procedures, not explanations.

- **Update pattern:** changes when infrastructure changes (often NOT caught by kb-sync, because infra changes may not come through code PRs)
- **Audience:** agents performing operational tasks
- **Staleness signal:** environment drift (factchecker often can't catch it — connection strings, hostnames, and paths are not in the verification table)

Runbooks should live on a separate branch (e.g., under "Infrastructure" or a dedicated "Operations" branch) rather than mixed into subsystem reference articles. This keeps the update patterns clean — a server migration should only require updating the operations branch, not touching every subsystem article that happened to mention a hostname.

**Template for operational runbooks:**

```markdown
## [Operation Name]

[What this runbook is for and when to use it]

### Prerequisites

[What must be true before starting — access, tools, permissions]

### Steps

1. [Step with command or action]
2. [Step with command or action]
3. [Verification step — how to confirm it worked]

### Where to find config values

- DB URL: configured in `.env` as `DATABASE_URL`
- Server details: see `infrastructure.md`
- Credentials: see `secrets/` directory

### Troubleshooting

[Common failure modes and how to diagnose them]

### Last verified

[Date this runbook was last manually tested]
```

Note the "Where to find config values" section — runbooks reference how to find values, not the values themselves (see the factchecker limitations in the Anchors section).

## Reference Article Template

Use this as a starting structure for reference (conceptual) leaf articles. Not all sections are required — omit what doesn't apply.

```markdown
## [Subsystem Name]

[1-3 sentences: what this subsystem does and why it exists]

### How it works

[Describe the flow, process, or behaviour. Use numbered steps for
sequential processes, bullets for parallel concerns. Stay at the
functional level — what happens, not how each line of code works.]

### Key files

- [Role]: `path/to/file.go`
- [Role]: `path/to/other.go`
- [Config]: `table_name` table, key `config_key`

### Data model

[If this subsystem owns entities or tables, describe them briefly.
Link to the Domain Model branch if entity definitions live there.]

### Constraints

[Invariants, edge cases, things that must not break. These are the
facts that save an agent from introducing a regression.]

### Tests

[Where the tests live and how to run them.]

- Unit: `path/to/test_file.go`
- Integration: `path/to/integration_test.go`
- Run: `go test ./path/to/...`
```

## What NOT to Do

- **Don't replicate the code.** If an agent needs to know what a function does at the line level, it should read the function. The KB article explains what the subsystem does.
- **Don't write articles for code being rewritten.** KB articles for code that's actively being replaced or refactored create more drift than value. But do write articles for code being built for the first time once it passes review — the article should be accurate at the point of delivery and becomes the stable reference going forward.
- **Don't front-load the entire KB.** Write articles as they become needed. The gap logging system (`/kb-gaps`) surfaces what's missing. A thin KB with accurate articles is better than a comprehensive KB with stale ones.
- **Don't store ephemeral state.** Current sprint goals, in-progress work status, temporary workarounds — these belong in work tracking shards or memories, not KB articles.
- **Don't over-nest.** Three levels maximum. If you're creating a 4th level, step back and restructure.
- **Don't write articles without anchors.** An article with no file paths, table names, or function names gives the agent nothing to navigate to and gives the factchecker nothing to verify. Every article should have at least 2-3 anchors.
- **Don't skip triggers.** A shard without a trigger on its parent edge is invisible to tree navigation. Every shard needs a trigger the moment it's linked into the tree.
- **Don't hardcode environment-specific values.** Connection strings, hostnames, ports, credentials, specific IDs (repository IDs, source system IDs), and paths that vary between environments are the fastest-rotting content in any KB. Reference where to find the value, not the value itself. See the factchecker limitations section under Anchors.

## Version History

When you update a knowledge article via `cxp knowledge update`, the old content is preserved as a closed version linked via a `previous-version` edge. This forms a change history:

```
pf-861f0c   Content Processing Pipeline (current, v3)
  ├── previous-version → pf-861f0c-v2 (closed)
  └── previous-version → pf-861f0c-v1 (closed)
```

Each version edge carries metadata:
```json
{
  "changed_at": "2026-03-06 11:00:15+00",
  "changed_by": "agent-penfold",
  "change_summary": "Add classify_project stage, replaces legacy keyword matching"
}
```

### Versioning rules

- **Change summaries matter.** Write them like commit messages — what changed and why. "Updated content" is useless. "Add classify_project stage, replaces legacy keyword matching" tells an agent whether the change is relevant to their current work.
- **Closed versions are still accessible.** `cxp shard show <version-id>` works. This is useful when investigating regressions — "what did this article say before the last update?"
- **Don't version for typo fixes.** Minor corrections (spelling, formatting, punctuation) don't need a version trail. Version when the conceptual content changes.
- **Version history supports debugging.** When an agent suspects a KB article led it astray, it can check `cxp knowledge history <id>` and `cxp knowledge diff <id>` to see what changed and when.

## Cross-References Between Articles

When one article references another, use a consistent format that includes the title, shard ID, and parent branch:

```markdown
See: **Model Selection Architecture** (pf-bc537d, under AI & Models)
```

This gives agents three ways to find the target: by title (human-readable), by ID (machine-navigable), and by branch (contextual placement).

## Managing the Unsorted Backlog

In active projects, KB articles are often created faster than they can be properly parented. This is fine — capturing knowledge quickly is more important than filing it correctly in the moment.

**Accept the unsorted bucket.** Create an explicit "Unsorted" branch. New articles that don't have an obvious home go here rather than being force-fitted into the wrong branch or left as top-level orphans.

**Triage weekly.** During the weekly triage cadence:
1. Review all shards in Unsorted
2. For each: move to the correct branch, merge into an existing article, or close if redundant
3. Check for top-level singletons (0 children, depth 0) — these are usually leaves that should be under a branch

**Watch the ratio.** If Unsorted holds more than ~20% of your total shards, your branch structure probably doesn't match your actual work patterns. Restructure branches before triaging individual articles.

**Distinguish branches from leaves.** A shard with 0 children at depth 0 is a leaf pretending to be a branch. Either give it children (it's actually a branch that hasn't been fleshed out) or move it under the right parent.

## Multi-Agent KB Authoring

When multiple agents write or maintain the KB, establish clear ownership to avoid conflicting updates and stale cached knowledge.

### Author vs. maintainer roles

- The **author** creates articles and makes substantive content changes — new sections, revised explanations, updated architecture descriptions.
- The **maintainer** updates anchors, fixes drift, and flags articles for rewrite — but does not change the narrative without escalation.

### Rules

- **Track attribution in metadata.** Every update should record `last_changed_by` and `last_change_summary`. This lets any agent distinguish between "this article was deliberately rewritten" and "an automated scan updated a file path."
- **Maintenance updates should be conservative.** When a maintainer finds a stale anchor, the correct action is usually to update the anchor and note the change — not to rewrite the surrounding explanation. If the surrounding content is also wrong, escalate to the author or flag it in the escalation queue.
- **Notify on substantive changes.** If a maintenance agent rewrites more than just anchors, it should create a notification (e.g., a gap log entry or a direct flag) so the author can review.
- **Beware cached knowledge.** If one agent updates an article while another agent has the old version loaded in its session context, the second agent may make decisions on stale data. Agents performing long-running work should reload KB articles at the start of each major task, not rely on content loaded earlier in the session.

## KB Renovation

The bootstrap process and the maintenance system handle two ends of the spectrum: building from zero and keeping things current incrementally. But there's a third situation: **the KB was accurate a month ago and is now comprehensively stale** due to infrastructure changes, new features, or architectural shifts.

This is not the same as incremental drift. Incremental drift is a renamed function here, a new column there — the nightly scan catches it. Bulk staleness is "the database moved to a different server, 4 new adapters exist with no shards, and the deployment process changed entirely." The drift scan catches anchor rot but not missing coverage.

### When to renovate vs. update incrementally

**Update incrementally** when:
- A few articles have stale anchors (functions renamed, files moved)
- The tree structure is still correct — branches map to real subsystems
- New features slot into existing branches

**Renovate** when:
- More than ~30% of leaf articles would fail a manual accuracy read
- New subsystems exist with no KB coverage at all
- The tree structure no longer reflects the system architecture (branches map to subsystems that were merged, split, or replaced)
- Infrastructure changed in ways that affect operational runbooks across the board

### Renovation process

1. **Audit scope.** Run the drift scan manually against all articles (`kb-drift-scan`). Separately, review the tree structure against the current codebase — list subsystems that exist in code but have no branch, and branches that reference subsystems that no longer exist.

2. **Triage articles into buckets:**
   - **Still accurate** — leave alone
   - **Anchor rot only** — update anchors, keep the narrative (kb-sync can often handle this)
   - **Conceptually stale** — the article describes something that works differently now. Rewrite the article from the current code, not from the old article.
   - **Orphaned** — the subsystem was removed or replaced. Close the shard (see Retiring Shards below).
   - **Missing** — subsystem exists with no coverage. Write a new article.

3. **Restructure the tree if needed.** If branches need to be added, merged, or renamed, do that first — before rewriting leaf articles. Update the playbook's branch index and triggers.

4. **Rewrite stale articles depth-first.** Pick the most-used branch (check access counts), rewrite its leaves, then move to the next branch. Don't try to renovate everything at once — the same "don't front-load" principle applies.

5. **Re-seed canaries.** After renovation, review the canary questions in `pf-kb-canaries`. Questions that reference removed or changed concepts need to be updated or replaced.

### What renovation is NOT

Renovation is not "delete everything and start over." The tree structure, shard IDs, edge metadata, and access counts are valuable. Preserve what's still accurate. Close what's dead. Rewrite what's wrong. Fill what's missing.

## Retiring Shards

KB shards have a lifecycle. When the subsystem they describe is removed or replaced, the shard should be closed — not left open as zombie knowledge that misleads agents.

### When to close a shard

- The code it describes has been deleted
- The subsystem was replaced by a new one (and a new shard covers the replacement)
- The article was split into multiple more focused shards (the original is now redundant)
- The article was merged into a broader shard

### How to close

```bash
cxp shard close <shard-id> --reason "Subsystem replaced by X — see <new-shard-id>"
```

The close reason is important — it tells any agent that finds a reference to this shard (e.g., in an old design or handoff) where to look instead.

### What happens to closed shards

- They no longer appear in `cxp kb tree` or `cxp kb search` results
- They are still accessible via `cxp shard show <id>` for historical reference
- Edges pointing to them remain (with the closed status visible)
- The nightly drift scan skips them

### Don't delete, close

Deleting a shard removes it permanently. Closing preserves the history and the redirect reason. Always close, never delete — unless the content was created in error and has no historical value.

## Ongoing Authoring

Once the KB exists, new articles are created in response to:

1. **New subsystems** — when code is built that doesn't map to any existing article
2. **Gap logging** — when an agent logs `/kb-gaps` because it couldn't find what it needed
3. **Weekly triage** — the `kb-triage` workflow proposes new articles based on accumulated gaps
4. **Promotion from work shards** — when a design/task is complete and the implemented knowledge is worth persisting (see the Spec → Work → KB lifecycle in `kb-shard-architecture.md`)

Existing articles are updated by:

1. **kb-sync at merge time** — automated, verified by Layer 1 (factcheck) and Layer 2 (judge)
2. **Nightly drift scan** — catches out-of-band changes
3. **Manual update** — when an agent notices something wrong during a working session

The escalation path for things no agent can resolve: `pf-kb-escalations` shard (reviewed weekly by a human).

### Monitor access counts during triage

Articles with zero accesses after 4+ weeks of active development in their subsystem are candidates for review — either the trigger isn't matching, the article isn't at the right abstraction level, or the subsystem genuinely hasn't been touched.

**Pair access counts with gap logs.** If agents are logging gaps in an area that has KB coverage, the existing articles aren't working. High gap log entries + existing articles = rewrite needed, not new articles.

**What access counts DON'T tell you:**
- Zero accesses doesn't mean the article is bad — it might cover a subsystem nobody has worked on recently
- High accesses doesn't mean the article is good — agents might be loading it repeatedly because it doesn't answer their question and they keep retrying

## Quick Reference

```bash
# Create a knowledge article
cxp knowledge create --title "Title" --body "Content..." --parent <parent-id>

# Set trigger on parent-child edge
cxp knowledge children add <parent> <child> \
  --trigger "When..." --description "Covers..."

# Update an article (creates new version)
cxp knowledge update <id> --body "New content..."

# View the tree
cxp kb tree

# Search the KB
cxp kb search "query"

# Log a gap
/kb-gaps "description of what was missing"

# Check article health
cxp knowledge show <id>   # shows access count, version, edges
cxp knowledge history <id> # version history
cxp knowledge diff <id>    # diff between versions
```
