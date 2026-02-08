<!-- cp-template-version: 1 -->
# Ways of Working — Template

How the orchestrating agent raises, tracks, and verifies work done by implementing agents.

Copy to your project's `docs/ways-of-working.md` and customize.

**Placeholders:**
- `[PROJECT]` — project name
- `[ORCHESTRATOR]` — orchestrating agent (raises work, verifies results)
- `[IMPLEMENTER]` — implementing agent (builds, tests, deploys)
- `[MAINTAINER]` — infrastructure/platform agent (optional)

---

## Agents & Roles

| Agent | Role | Owns |
|-------|------|------|
| **User** | Product owner. Sets direction, approves specs, makes design decisions. | Everything |
| **[ORCHESTRATOR]** | Orchestrator. Finds bugs, defines features, writes specs, verifies results. | Work item quality, verification, escalation |
| **[IMPLEMENTER]** | Developer. Implements bugs/features/specs via `/ingest` pipeline. | Codebase, deployment, tests |
| **[MAINTAINER]** | Platform maintainer. | Infrastructure, shared tooling |

---

## How Work Flows

```
[ORCHESTRATOR] finds problem/need
    │
    ├─ Bug?     → Fill BUG template     → send (kind:bug)
    ├─ Feature? → Fill FEATURE template  → send (kind:requirement)
    └─ Spec?    → Write full spec        → send (kind:spec)
                                              │
                                    [IMPLEMENTER] runs /ingest
                                              │
                                    classify → investigate → triage →
                                    test → implement → verify → deploy
                                              │
                                    [IMPLEMENTER] sends resolution
                                              │
                                    [ORCHESTRATOR] verifies ← QUALITY GATE
                                              │
                                    Verified? YES → close / NO → escalate
```

---

## Parallel Sessions

When running multiple [IMPLEMENTER] sessions simultaneously:

1. **[ORCHESTRATOR] advises what can parallelize** — analyzes file overlap between items,
   recommends grouping, flags multi-day work that needs a feature branch
2. **User assigns shards to sessions based on [ORCHESTRATOR]'s advice**
3. **All sessions work on main** — stage only own files, never `git add -A`
4. **File claims prevent two sessions modifying the same file**
5. **Only one session deploys at a time** — later sessions pull + re-verify first
6. **Feature branches only for multi-day specs** — [ORCHESTRATOR] advises when needed
7. **Context exhaustion → close shard with progress notes, start new session to continue**

---

## Bug Reports

### Template

```markdown
## Bug: [short title]

**Component:** [component list for your project]
**Severity:** [P0 blocking | P1 high | P2 medium | P3 low]
**Version:** [version or commit]

### Symptom
[Exact error, unexpected output, missing data. Be specific.]

### Steps to Reproduce
1. [command or action]
2. [what happened]
3. [what should have happened]

### Expected Behavior
[What should happen. Example output if possible.]

### Evidence
[Paste actual output, errors, query results.]
```

### Definition of Done

- [ ] Root cause identified and explained
- [ ] Regression test: fails without fix, passes with fix
- [ ] All tests pass
- [ ] Deployed + version verified (not a ghost deploy)
- [ ] Sample output from the running system
- [ ] Before/after comparison (for pipeline/data changes)
- [ ] Reprocessed content (for pipeline changes)

---

## Feature Requests

### Template

```markdown
## Feature: [short title]

**Component:** [component]
**Priority:** [P1 high | P2 normal | P3 low]

### What
[One paragraph: what, who, why]

### Behavior
[Expected behavior. Command syntax + output for CLI. Request/response for API.]

### Acceptance Criteria
[Numbered. Each must be testable.]
1. [criterion]
2. [criterion]

### Scope Boundaries
[What this does NOT include.]

### Test Cases
[Concrete scenarios to implement as tests.]
```

### Definition of Done

- [ ] Each acceptance criterion met with a named test
- [ ] All tests pass
- [ ] Deployed + version verified
- [ ] Example usage with actual output from running system
- [ ] Scope respected — nothing built outside stated boundaries

---

## Specs

### Template

Use `SPEC-TEMPLATE.md` for full spec format. Covers: Goal, Data Model, CLI Surface,
SQL Functions, Go Implementation, Success Criteria, Edge Cases, Test Cases.

### Before Sending

1. Complete the pre-submission checklist
2. If >300 lines, run a sub-agent review
3. Get user approval on design decisions

### Definition of Done

- [ ] All success criteria met (N/N — no partial)
- [ ] All test cases implemented
- [ ] Schema/CLI matches spec exactly
- [ ] Deployed + version verified
- [ ] Example output from each new command

---

## Escalation

### Level 1: Evidence-based rejection

"Not verified. Here's what I checked, what I expected, what I got."

### Level 2: Deployment investigation

"Still broken after second attempt. Verify the correct binary is running."

### Level 3: Direct investigation

"I checked the server myself. Here's what I found. Here's exactly what to fix."

---

## Quality Rules

1. No bug closed without a regression test
2. No feature closed without acceptance tests mapping to criteria
3. No spec closed without all success criteria met
4. No deploy accepted without version verification
5. No resolution accepted without sample output from the running system
6. Reprocessing required for pipeline changes — show before/after
7. Scope respected — ask before building outside scope
8. Test-first — failing test before fix/feature

---

## Quality Metrics

Define measurable bars for your project's domain. Examples:

| Domain | Metric | Target | How to Check |
|--------|--------|--------|-------------|
| [area] | [what to measure] | [threshold] | [query or command] |

"Show me output" is not verification. Queries with thresholds are verification.

---

## /ingest Pipeline Mapping

| Work Item | Phase 1 Prefix | Phase 2 | Phase 3 Route |
|-----------|---------------|---------|---------------|
| Bug (kind:bug) | `investigate:` | Debugger agent | `fix:` shard |
| Feature (kind:requirement) | `analyze:` | Explorer agent | `feat:` shard |
| Spec (kind:spec) | `spec:` | **Skipped** | `feat:` → decompose if HIGH |

---

## cp init

This template is scaffolded by `cp init` which creates:
- `docs/ways-of-working.md` — this file (customized)
- `.claude/commands/ingest.md` — pipeline orchestrator
- `.claude/commands/ingest.*.md` — pipeline phase files
- `CLAUDE.md` — agent identity and Context Palace connection
- `context-palace.md` — full CP usage guide
