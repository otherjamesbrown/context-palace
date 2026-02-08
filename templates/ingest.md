<!-- cp-template-version: 1 -->
# Ingest — Orchestrator Template

Template for agent work pipelines. Copy to your project's `.claude/commands/ingest.md`
and customize the placeholders.

**Placeholders to replace:**
- `[PROJECT]` — your project name (e.g., `penfold`)
- `[AGENT_NAME]` — the implementing agent (e.g., `agent-mycroft`)
- `[ORCHESTRATOR]` — the orchestrating agent (e.g., `agent-penfold`)
- `[DB_CONN]` — your Context Palace connection string
- `[PALACE_CLI]` — path to the palace CLI binary

---

## What This Pipeline Does

Single entry point for all implementation work. Pulls work items from Context Palace,
classifies them, investigates/analyzes, decomposes, writes tests, implements, verifies,
and deploys.

```
Phase 1:   /ingest.classify     — Pull & classify inbox (bugs vs requirements vs specs)
Phase 2:   /ingest.investigate   — Launch debuggers (bugs) and explorers (requirements). SPECs skip this.
Phase 3:   /ingest.triage        — Create impl shards, route by complexity, decompose HIGH
Phase 3.5: /ingest.test          — Write failing tests (all items, per-wave for HIGH)
Phase 4:   /ingest.implement     — Launch implementation agents
Phase 5:   /ingest.verify        — Verify builds, integration tests, cross-check, reply to [ORCHESTRATOR]
Phase 6+7: /ingest.deploy        — Commit, deploy, verify deployment, release
```

## Work Item Classification

| Type | Identified By | Phase 2 | Example |
|------|--------------|---------|---------|
| BUG | Symptom, error, "used to work", regression | Investigate (debugger) | "queue fails with timeout" |
| REQUIREMENT | New capability, enhancement, "add X" | Analyze (explorer) | "add --format json flag" |
| SPEC | Structured sections, acceptance criteria, data model, SQL | **Skip** (spec is the analysis) | Full spec with schema + tests |

## Complexity Routing

| Complexity | Layers | Approach |
|------------|--------|----------|
| LOW | 1 | Single agent, single pass |
| MEDIUM | 1-2 | Single agent, clear pattern |
| HIGH | 3+ | Decompose into layer sub-shards (DB → Service → CLI) |

## Sub-Shard Size Limits

Each sub-shard must fit in one agent's context window:

| Metric | Limit | If Exceeded |
|--------|-------|-------------|
| Files to modify/create | ≤15 | Split into sub-layers |
| Expected lines of change | ≤500 | Split by functional area |
| Acceptance criteria | ≤8 per sub-shard | Group into separate sub-shards |

## Parallel Session Coordination

When the user runs multiple sessions simultaneously:

1. **User assigns specific shards to each session** — sessions don't self-serve
2. **Check file claims before implementing** — two agents modifying the same file = broken code
3. **Claim files at Phase 3** (triage), not Phase 4 (implement) — gives other sessions visibility
4. **Only one session deploys at a time** — check for in-progress deploys before starting
5. **Use feature branches** when multiple sessions run simultaneously

## Definition of Done

Every resolution to [ORCHESTRATOR] must include:

**For bugs:**
- Root cause + fix description
- Regression test (fails without fix, passes with fix)
- All tests pass
- Deployed + version verified
- Sample output from the running system
- Before/after comparison (for pipeline/data changes)
- Reprocessed content output (for pipeline changes)

**For features:**
- Each acceptance criterion with pass/fail + test name
- All tests pass
- Deployed + version verified
- Example usage with actual output

**For specs:**
- All success criteria met (N/N)
- All test cases implemented
- Schema/CLI matches spec exactly
- Deployed + version verified
- Example output from each new command

## Key Principles

1. **Orchestrator never writes code** — always delegate to sub-agents
2. **Route by complexity** — LOW/MEDIUM single agent; HIGH decompose by layer
3. **Classify correctly** — BUGs investigate, REQs analyze, SPECs skip analysis
4. **No overlapping scopes** — each agent owns distinct files
5. **Layer ordering for HIGH** — DB → Service → CLI → Pipeline, sequential
6. **Tests are mandatory** — test-first for all complexity levels
7. **Feedback at boundaries** — progress updates to [ORCHESTRATOR] between phases
8. **Verify deployment** — confirm running binary matches expected commit
9. **Real-data verification** — for pipeline changes, reprocess and show before/after
10. **Sub-shard size limits** — prevent context exhaustion with scope limits

## Phase Files

Each phase is a separate slash command. Create these alongside this orchestrator:

| File | Description |
|------|-------------|
| `ingest.classify.md` | Pull inbox, classify BUG/REQ/SPEC, create shards, ack |
| `ingest.investigate.md` | Launch debugger + explorer agents, skip SPECs |
| `ingest.triage.md` | Create impl shards, route by complexity, decompose HIGH |
| `ingest.test.md` | Write failing tests before implementation |
| `ingest.implement.md` | Launch implementation agents (single or layer-by-layer) |
| `ingest.verify.md` | Build, test, integration test, real-data verify, review |
| `ingest.deploy.md` | Commit, deploy, version verify, smoke test, summarize |

See the penfold project for a reference implementation of each phase file.
