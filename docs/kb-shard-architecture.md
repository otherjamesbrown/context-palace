# Knowledge Shard Architecture — Why It Works for Agents

Context Palace organises knowledge into a navigable tree of **shards** — small, focused documents with typed edges, trigger-based loading, and tiered access patterns. This architecture is grounded in academic research on how LLM agents manage context across sessions.

For engineering projects, the most effective operating model is usually not "put everything in the KB from day one." It is a staged lifecycle:

`Spec -> Design/WorkItem Shard -> KB Shard`

- **Specs** hold intended architecture and hard constraints while a subsystem is being designed and built
- **Design/task/bug shards** turn that intent into executable work, status, blockers, and proof boundaries
- **Knowledge shards** are written after implementation is real, tested, and worth retrieving as operational truth

This matters because build-time intent and post-implementation reality are not the same thing. Agents need both, but at different times.

The retrieval hierarchy that supports this lifecycle is:

`law -> map -> domain knowledge -> verified implementation`

- **Law** — repo-level constitution files such as `AGENTS.md`, `CLAUDE.md`, or equivalent
- **Map** — the root KB playbook that routes the agent to the right branch
- **Domain knowledge** — branch and leaf KB shards containing focused subsystem knowledge
- **Verified implementation** — code and tests, which win when documents drift

## The Problem

LLM agents face a fundamental tension: they need access to large amounts of project knowledge, but their context windows are finite and expensive. The naive approaches both fail:

- **Load everything at session start** — wastes tokens, buries signal in noise, and hits context limits on any non-trivial project.
- **Load nothing, search on demand** — agents don't know what they don't know. They can't search for knowledge they don't know exists.

Single-file configurations (CLAUDE.md, .cursorrules) work for small projects but break down as systems grow. An empirical study of 328 Claude Code projects found that 72.6% specify architecture in their configuration files, with a median of 7 sections per file (Agarwal et al., 2025). But a file with 7 sections becomes a file with 70 sections becomes an unnavigable wall of text.

## Academic Foundation

Context Palace's shard architecture draws heavily from **"Codified Context: Infrastructure for AI Agents in a Complex Codebase"** (Vasilopoulos, 2026), which describes building a 108,000-line C# distributed system across 283 development sessions using a three-tier memory architecture.

### The Three-Tier Model

The paper proposes organising agent knowledge by **access temperature** — how likely the agent is to need it in any given session:

| Tier | Temperature | Loading | Size | Purpose |
|------|-------------|---------|------|---------|
| 1 | Hot | Always loaded | ~660 lines | Conventions, routing tables, retrieval hooks |
| 2 | Warm | Loaded per-task | ~490 lines avg | Domain specialist knowledge |
| 3 | Cold | Loaded on demand | ~480 lines avg | Subsystem specifications |

**Key finding:** The hot tier must include **retrieval hooks** — not the knowledge itself, but pointers that tell agents where to look. The constitution (hot tier) contained trigger tables mapping file patterns and task types to the appropriate specialist or specification document. This is what makes the system self-navigating rather than relying on the human to load the right context.

**Scale:** The project maintained 54 context artifacts totalling ~26,200 lines — 24.2% of the codebase size. This is the cost of agent-navigable knowledge, but it paid for itself: 16,522 autonomous agent turns across 283 sessions with a team of 19 specialist agents.

### Why Specialisation Matters

The paper found that over 50% of each specialist agent's specification was **domain knowledge, not behavioural instructions**. Agents operating in complex domains produced significantly more errors without pre-loaded context. The practical heuristic: "if debugging a particular domain consumed an extended session without resolution, it was faster to create a specialized agent and restart."

This mirrors Context Palace's approach: shards aren't generic documentation — they're task-oriented knowledge packages that give an agent enough context to work effectively in a specific domain.

## How Context Palace Implements This

### Shards as Knowledge Units

A **shard** is the atomic unit of knowledge. Each shard has:

- **Content** — the knowledge itself (architecture docs, workflow procedures, decision records)
- **Type** — what kind of knowledge it is (reference, design, task, bug)
- **Edges** — typed relationships to other shards (child-of, blocked-by, implements)
- **Trigger metadata** — when an agent should load this shard

Shards are small by design. The paper's cold-tier documents averaged ~480 lines each. Context Palace encourages splitting anything over ~300 lines into a branch with children.

### The Playbook as Hot Memory

The **playbook** (loaded on every session start) is Context Palace's implementation of the paper's "constitution." It contains:

1. **Operating principles** — how the agent should behave
2. **Branch index with triggers** — a directory of knowledge domains, each with a description of when to load it
3. **Quick reference tables** — the most frequently needed lookups
4. **Retrieval hierarchy** — explicit instructions for how to find information (check hot context first, navigate the tree, search, then scan code as last resort)

The playbook is kept under ~200 lines. Everything else lives in the tree.

### Hot Tier = Law + Map

In real projects, the hot tier should usually be treated as a combination of two artifacts:

1. **Law** — the repo constitution (`AGENTS.md`, `CLAUDE.md`, or equivalent)
2. **Map** — the playbook shard at the root of the knowledge tree

These artifacts are both "always available," but they are not interchangeable.

**Law** exists to define:

- non-negotiable constraints
- source-of-truth precedence
- coding, testing, and workflow rules
- things the agent must obey even when other documentation says something different

**Map** exists to define:

- what knowledge branches exist
- when to load which branch
- which KB shards supersede old build-time specs for implemented systems
- how to route from a task to the right domain knowledge

Treating both as hot context avoids two failure modes:

- putting everything into the law file, which turns it into a bloated, stale flat document
- putting hard constraints only into the playbook, which makes them feel like optional routing hints instead of rules

The clean model is:

- **law** tells the agent what it must obey
- **map** tells the agent where to go next

### Tree Navigation with Triggers

The knowledge tree uses **trigger-based navigation** rather than requiring agents to understand a taxonomy upfront:

```
Playbook (always loaded, ~200 lines)
  ├── Architectural Principles (trigger: "hard constraints, config rules")
  ├── Ingest Pipeline (trigger: "content ingestion, pipeline stages")
  ├── Knowledge Graph (trigger: "entities, glossary, mentions")
  ├── Infrastructure (trigger: "servers, deployment, services")
  └── How We Work (trigger: "work tracking, delegation, workflows")
```

Each branch node is a lightweight index — it lists its children with triggers, not the full content. An agent encountering a pipeline question reads the playbook, sees the "Ingest Pipeline" trigger matches, loads that branch, finds children like "Stage Definitions" and "Classification Routing," and loads the specific leaf it needs.

This is the paper's retrieval hook pattern: the hot tier tells agents where to look, branch nodes narrow the search, and leaf nodes provide the actual knowledge.

The quality of those retrieval hooks matters. In practice, the `trigger` and `description` on each parent-child edge are the shard's discoverability interface:

- `trigger` answers: "Read this when you need to know about X, Y, or Z."
- `description` answers: "This shard covers A, B, and C."

Agents creating or updating KB shards should write these as retrieval-quality text, not placeholders:

- write from the agent's perspective
- use task language or question language
- include the terms likely to appear in a work item or debugging session
- avoid vague labels that could apply to many siblings
- revise the text when the shard's scope changes

Poor example:

```text
trigger: "auth"
description: "Authentication docs"
```

Better example:

```text
trigger: "Need token refresh flow, session expiry rules, or provider auth troubleshooting"
description: "Access token lifecycle, refresh behavior, auth failure modes, provider-specific caveats"
```

If this text is weak, the shard is effectively invisible except through search.

### Temperature in Practice

| Temperature | What | How loaded | Token cost |
|-------------|------|------------|------------|
| Hot | Law + playbook | SessionStart hook, automatic | ~200-400 lines per session |
| Warm | Branch nodes | Agent reads trigger, loads on match | ~50-100 lines per branch |
| Cold | Leaf articles | Agent navigates tree or searches | ~100-400 lines per article |
| Search | Any shard | `cxp kb search` (hybrid BM25 + vector) | Variable |

An agent working on pipeline classification might load: playbook (hot) + Ingest Pipeline branch (warm) + Classification Routing leaf (cold) = ~400 lines of focused context. Compare this to loading the entire knowledge base (~37,000 tokens across 23 open designs alone).

## Why Shards Beat Flat Files

### 1. Selective Loading

A 2,000-line CLAUDE.md burns 2,000 lines of context whether the agent needs it or not. Shards let agents load exactly the knowledge relevant to their current task. The paper found that agents invoked specialists selectively — the code reviewer was used 154 times, but most specialists were used far less frequently. Knowledge should follow the same pattern.

### 2. Freshness at the Leaf Level

The paper identifies specification staleness as **"the primary failure mode"** — outdated specs caused agents to wire code through deprecated paths. With flat files, staleness is hard to detect because everything is one document. With shards, each leaf has its own `updated_at` timestamp and access log. Stale shards can be detected by comparing their last update against recent git activity in the files they reference.

Context Palace tracks `access_count` and `last_accessed` per shard, making it possible to identify knowledge that's frequently used (high-value, worth keeping current) versus knowledge that's never accessed (candidate for archival or restructuring).

### 3. Multi-Agent Coordination

When multiple agents work on the same codebase, flat files create a coordination problem: which agent updates the file? How do you avoid conflicts? Shards solve this naturally — each agent can own and update specific shards without touching others. The paper's 19 specialist agents each had their own specification documents that could be updated independently.

### 4. Typed Relationships

Edges between shards capture relationships that flat files lose:

- **child-of** — hierarchical organisation (branch → leaf)
- **blocked-by** — dependency tracking between work items
- **implements** — traceability from tasks to requirements
- **discovered-from** — provenance of knowledge

These relationships enable queries that flat files can't support: "what knowledge articles are affected by this design change?" or "what work is blocked by this unfinished task?"

### 5. Search as Fallback

Tree navigation handles known-unknowns (the agent knows where to look). But for unknown-unknowns, Context Palace provides hybrid search across all shards — BM25 for keyword matches plus vector similarity for semantic matches. The paper identified this gap: their keyword-matching retrieval service worked but missed semantic connections. Context Palace's hybrid search addresses this.

## The Engineering Lifecycle: Spec -> Work -> KB

This is the pattern that tends to work best for AI coding agents on real software projects.

### Phase 1: Spec

At the start of a subsystem, the most useful artifact is often a spec:

- intended architecture
- constraints and contracts
- acceptance intent
- boundaries between components

At this stage, the spec is the best available source of truth because the code either does not exist yet or is incomplete.

### Phase 2: Design and Work Shards

Once implementation begins, agents should not work directly from a large spec alone. The spec must be translated into executable work:

- a `design` shard for the implementation slice or phase
- child `task` shards for concrete work items
- `bug` shards for defects discovered during implementation or review

These shards do the operational work that specs do poorly:

- define exact scope
- track status and ownership
- record blockers and decisions
- attach proving tests and artifact links
- capture implementation drift as it happens

This is where the "AI coding loop" lives: define the slice, define the proof, do the work, validate it, close the shard.

### Phase 3: Knowledge Shard

After the code is implemented and verified, a KB shard can become the retrieval-first document for that subsystem.

A good KB shard for implemented software should include:

- what the system does now
- where the code lives
- what tests prove it
- known operational constraints
- accepted differences from the original spec
- triggers that tell future agents when to load it

Those triggers should be written with the same care as the article body. A technically correct shard with vague routing text is still a retrieval failure.

At this point, the spec is no longer the primary working context for day-to-day implementation. It becomes historical design intent. The KB shard becomes active memory because it reflects implemented reality.

### Why This Works Better Than Spec-Only

If agents keep loading old specs after a subsystem has evolved, they inherit stale assumptions:

- old interfaces
- outdated flow diagrams
- superseded persistence rules
- missing implementation caveats

That creates exactly the kind of drift-induced failure the architecture is trying to avoid.

The lifecycle solves this cleanly:

- **Spec** answers: what are we trying to build?
- **Work shards** answer: what exactly are we doing now, and how do we know it is done?
- **KB shard** answers: how does this implemented system actually work?

Within the retrieval hierarchy, that becomes:

- **Law** answers: what rules override everything else?
- **Map** answers: where should I go for the right context?
- **Domain knowledge** answers: what does this subsystem mean and how is it operated?
- **Verified implementation** answers: what is true right now?

### Why This Works Well for AI Coding Agents

AI coding agents perform best when the active context matches the actual phase of work:

- During build-out, they need constraints and proving tests more than polished reference docs
- During execution, they need bounded tasks and explicit acceptance criteria
- After implementation, they need concise retrieval-oriented operational knowledge, not the full history of design deliberation

This lifecycle keeps context aligned with the task:

| Phase | Primary artifact | What the agent needs most |
|------|------------------|---------------------------|
| Before implementation | Spec | intent, constraints, architecture |
| During implementation | Design/task/bug shards | scope, status, proof boundary, drift capture |
| After implementation | KB shard | implemented reality, code entry points, operating knowledge |

That improves agent performance in three ways:

1. **Less stale context.** Agents stop depending on design-time documents once implementation has diverged.
2. **Better task focus.** Work shards define a narrow slice with an explicit proof boundary.
3. **Better reuse.** Once verified, the KB shard becomes a reusable context package for future agents and future sessions.

### Promotion Rule: When Something Becomes KB

Not every work shard should become a knowledge shard. Promotion should happen only when:

- the code exists
- the proving tests exist
- the implementation is stable enough to be reused
- the knowledge is likely to matter again

In short: **no KB shard without proof**.

The proof is normally:

- code references
- passing unit/integration/e2e tests
- artifact links from the completed work shard

### What Happens to the Spec?

The spec should not necessarily be deleted. In most projects it becomes:

- historical design intent
- a reference for why the system started the way it did
- a useful comparison point when intentional drift is accepted

But it should stop being the default active retrieval target once the KB shard exists and reflects verified implementation.

This is the key distinction:

- **Specs are for building**
- **KB shards are for operating and extending**

And the higher-level decision order is:

- **Law -> Map -> Domain Knowledge -> Verified Implementation**

If these layers disagree, the resolution order should normally be:

1. Law beats map
2. Map routes to the relevant domain shard
3. Domain shard beats historical build-time docs for that subsystem
4. Verified implementation beats all documentation when reality has drifted

## Access-Count Promotion — Reducing Navigation Calls

The three-tier model assumes agents navigate from hot → warm → cold, making explicit tool calls at each step. But usage data reveals that certain warm/cold shards are accessed almost every session — these are effectively hot knowledge masquerading as cold.

Context Palace tracks `access_count` on every shard and exposes it in the `knowledge show` response. The playbook's children array includes each branch's access count:

```json
{
  "id": "pf-6b8072",
  "title": "Penfold CLI Reference",
  "trigger": "When looking up a penf command",
  "access_count": 47
}
```

**Dynamic promotion** uses this data to push frequently-accessed 2nd-layer articles directly into the session start context, eliminating the navigation call entirely. Instead of:

1. Agent reads playbook (hot) — sees trigger for CLI Reference
2. Agent calls `cxp shard show pf-6b8072` — loads branch
3. Agent finds the leaf it needs — another call

With promotion, high-access branches are injected at session start:

1. Agent reads playbook (hot) — CLI Reference content already present

This reduces the number of tool calls per session. Each eliminated call saves ~2-5 seconds of round-trip time and preserves the agent's "flow" — interrupting reasoning to make a retrieval call and then re-integrating the result is cognitively expensive for LLMs, just as context-switching is expensive for humans.

**Promotion criteria:** A shard qualifies for promotion when its access count crosses a threshold relative to the total session count. If a shard is accessed in >60% of sessions, it's effectively hot knowledge and should be loaded at session start. The `knowledge show` command already exposes the data needed for this decision — the session start hook can query for high-access children and inject them alongside the playbook.

**Budget constraint:** Promoted shards consume context window budget on every session, even when not needed. The playbook is ~200 lines; each promoted branch adds ~50-100 lines. A practical budget is 3-5 promoted branches before the hot tier becomes bloated. Access counts provide the signal for which branches earn that budget.

This is a feedback loop: access counts measure what agents actually need → promotion reduces friction for high-demand knowledge → agents work faster → the system learns from real usage rather than upfront taxonomy design.

## Relationship to llms.txt

The `/llms.txt` proposal (Howard, 2024) addresses a parallel problem: how do you make a body of knowledge accessible to LLMs without forcing them to process everything?

The proposal introduces a standardised file at a website's root that provides:
- A concise summary of what the site contains
- An index of available documents with URLs
- Machine-readable structure (Markdown with H2-delimited sections)
- Optional tiering: `llms.txt` (index) vs `llms-full.txt` (expanded content)

This maps directly to Context Palace's architecture:

| llms.txt Concept | Context Palace Equivalent |
|-----------------|--------------------------|
| `/llms.txt` — concise index | Playbook — branch index with triggers |
| `/llms-full.txt` — expanded content | Full KB tree — all shards loaded on demand |
| H2 sections with URLs | Branch nodes with child triggers and shard IDs |
| Markdown for machine readability | Shard content in Markdown, typed edges for structure |
| Optional sections for secondary content | Low-access-count shards kept cold |

Context Palace extends the llms.txt pattern in several ways:

1. **Dynamic rather than static.** llms.txt is a file that someone writes and maintains. Context Palace's index is generated from the live shard tree — adding a shard with a trigger automatically updates the navigable index.

2. **Access-count feedback.** llms.txt has no usage data. Context Palace tracks which shards agents actually load, enabling the promotion pattern described above.

3. **Typed relationships.** llms.txt is a flat list of links. Context Palace's edges (child-of, blocked-by, implements) encode structure that enables queries like "what's affected by this change?"

4. **Multi-agent scoping.** llms.txt serves one consumer. Context Palace can present different views of the same knowledge tree to different agents based on their domain (e.g., penfold sees pipeline knowledge, steve sees TUI knowledge).

The core insight is shared: **give the LLM an index it can navigate, not a dump it must parse.** Whether that's a `/llms.txt` file at a web root or a playbook loaded at session start, the pattern is the same — a small, always-available document that tells the agent what knowledge exists and how to access it.

## Design Principles

### Write for the Agent, Not the Human

Shard content should be optimised for agent consumption:

- **Lead with the decision or pattern**, not the history of how it was derived
- **Include file paths and code locations** — agents need to know where to look, not just what to look for
- **State triggers from the agent's perspective** — "When I am working on pipeline classification" not "Classification system documentation"

### Keep the Tree Shallow

The paper's three tiers map to: playbook → branch → leaf. Deeper nesting increases navigation cost. If you need a fourth level, consider whether the branch node is doing too much and should be split.

### Triggers Are the API

The trigger on each edge is effectively the shard's API — it's the contract between the navigation system and the knowledge content. A bad trigger means the shard never gets loaded when needed. Write triggers as task descriptions, not topic labels.

### Index Nodes Stay Thin

Branch nodes should be lightweight indexes (~50-100 lines) that list children with triggers. They should not contain detailed documentation — that goes in leaves. The playbook itself follows this pattern: it's an index of branches, not a knowledge dump.

### Staleness Is the Enemy

The paper reports 1-2 hours per week of maintenance to keep specifications current, including:
- 5 minutes per session when a specification is affected
- 30-45 minute biweekly review passes
- Automated drift detection via git hooks

Context Palace supports this with access logging (identifying stale shards), weekly grooming workflows, and the principle that spec reviews must include knowledge update criteria.

## Metrics from the Paper

| Metric | Value | Implication |
|--------|-------|-------------|
| Codebase size | 108,256 lines | Large enough that flat-file approaches fail |
| Context infrastructure | 26,200 lines (24.2%) | Significant investment, but enables autonomous work |
| Development sessions | 283 | Cross-session persistence is essential |
| Autonomous agent turns | 16,522 | Agents do most of the work; they need good context |
| Specialist agents | 19 | Domain knowledge matters more than generic instructions |
| Cold-tier documents | 34 | Manageable number of focused specifications |
| Human prompts under 100 words | >80% | With good context, humans give terse instructions |
| Maintenance overhead | 1-2 hours/week | The cost of keeping knowledge current |

## References

1. Vasilopoulos, A. (2026). "Codified Context: Infrastructure for AI Agents in a Complex Codebase." arXiv:2602.20478v1. — The primary foundation for Context Palace's tiered shard architecture.

2. Agarwal, S., Kang, S.Y., & Kim, D. (2025). "Decoding the Configuration of AI Coding Agents: An Empirical Study on Claude Code Projects." arXiv:2511.09268v1. — Empirical analysis of 328 CLAUDE.md files showing that architecture documentation (72.6%) and development guidelines (44.8%) are the most common configuration patterns.

3. Howard, J. (2024). "/llms.txt — A Proposal for Standardised LLM-Readable Website Content." llmstxt.org. — Standardised index files for making web content navigable by LLMs. Context Palace's playbook follows the same index-with-pointers pattern.

4. Paul, G. (2023). "Aider Repository Map." — Tree-sitter AST + PageRank approach to codebase navigation. Captures code structure but not operational semantics or design intent.
