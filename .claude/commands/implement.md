---
description: "Implement a spec: read, plan, test-first, build, verify, deploy. Structured pipeline for CP CLI work."
---

# Implement — Structured Spec Pipeline

Single entry point for implementing specs against the CP CLI codebase.

## User Input

```text
$ARGUMENTS
```

Parse the input:
- **Spec file path** — e.g. `specs/cp-cli/SPEC-9-test-coverage.md` or just `SPEC-9`
- **Specific phase** — e.g. `SPEC-9 phase 2` to resume at a phase
- If no arguments, check Context Palace inbox for assigned specs

## Configuration

```yaml
AGENT_NAME: agent-cxp
PROJECT: penfold
DB_CONN: "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full"
REPO_ROOT: ~/github/otherjamesbrown/context-palace
```

## CRITICAL: You Write Code Directly

Unlike mycroft (who orchestrates sub-agents), you implement directly. You read the spec,
write the code, run the tests. No delegation. This means:

- You own every file you touch — no conflicts
- You must manage your own context budget — if a spec has 4 phases, don't spend 80% on phase 1
- If you're running low on context, checkpoint and stop. Do NOT rush to finish.

## Phase Flow

```
Phase 1:  READ      — Parse spec, identify deliverables, check what exists
Phase 2:  PLAN      — Break work into ordered deliverables, estimate scope per item
Phase 3:  TEST      — Write failing tests for current deliverable (test-first)
Phase 4:  BUILD     — Implement until tests pass
Phase 5:  VERIFY    — go test, go vet, go build, coverage check
Phase 6:  REPORT    — Checkpoint + message to penfold with results
Phase 7:  DEPLOY    — Build binary, install, verify (after ALL deliverables complete)
```

Phases 3-6 repeat for each deliverable. Do NOT implement everything then test everything.
**One deliverable at a time: test → build → verify → next.**

Phase 7 runs once, after the last deliverable.

## Phase 1: READ

1. Read the spec file completely
2. Read every source file referenced in the spec
3. Read `pf-rules` for project conventions:
   ```sql
   SELECT content FROM shards WHERE id = 'pf-rules';
   ```
4. Identify:
   - **Deliverables** — numbered items in "What to Build"
   - **Dependencies** — which deliverables must come first
   - **Success criteria** — what "done" looks like per deliverable
   - **Test cases** — specific tests listed in the spec
5. Check for existing work:
   ```bash
   git status
   git log --oneline -10
   ```

**Output:** Mental model of the work. No code written yet.

## Phase 2: PLAN

1. Order deliverables by dependency (spec usually defines this)
2. For each deliverable, list:
   - Files to create or modify
   - Test file and test function names
   - Estimated lines of code
3. Check total scope against context budget:
   - **< 500 LOC total:** proceed with all deliverables
   - **500-1000 LOC:** proceed but checkpoint between deliverables
   - **> 1000 LOC:** implement only the first N deliverables, checkpoint, tell penfold what remains
4. Send plan to penfold (non-blocking):
   ```sql
   SELECT send_message('penfold', 'agent-cxp', ARRAY['agent-penfold'],
     'Implementation plan: [SPEC-N]',
     'Deliverables: [count]. Estimated: [LOC]. Order: [list]. Starting with: [first].',
     NULL, NULL, NULL, NULL, 'review');
   ```

**CHECKPOINT after Phase 2.**

## Phase 3: TEST (per deliverable)

For the current deliverable:

1. Read the spec's test cases for this deliverable
2. Write the test file with all tests **that will fail** (functions exist but implementation is stub/missing)
3. Run the tests to confirm they fail:
   ```bash
   cd ~/github/otherjamesbrown/context-palace/cp && go test ./[package]/... -run "TestName" -v 2>&1 | head -50
   ```
4. If tests don't compile (missing types/functions), add minimal stubs to make them compile but fail

**Do NOT skip this phase.** Test-first catches spec misunderstandings before you write implementation.

## Phase 4: BUILD (per deliverable)

1. Implement the deliverable — add/modify source files
2. Run the specific tests frequently:
   ```bash
   cd ~/github/otherjamesbrown/context-palace/cp && go test ./[package]/... -run "TestName" -v
   ```
3. Fix until all tests for this deliverable pass
4. Run the full package tests to check for regressions:
   ```bash
   cd ~/github/otherjamesbrown/context-palace/cp && go test ./[package]/...
   ```

**API stability rule:** Do NOT change existing function signatures. If you need a different
interface, add a new function. Changing signatures cascades across the codebase.

## Phase 5: VERIFY (per deliverable)

Run all three checks:

```bash
# 1. All tests pass (not just this deliverable)
cd ~/github/otherjamesbrown/context-palace/cp && go test ./...

# 2. No vet issues
cd ~/github/otherjamesbrown/context-palace/cp && go vet ./...

# 3. Clean build
cd ~/github/otherjamesbrown/context-palace/cp && go build ./...
```

If the spec has coverage targets:
```bash
cd ~/github/otherjamesbrown/context-palace/cp && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

**If ANY check fails:** fix it before proceeding. Do NOT move to the next deliverable
with a broken build.

## Phase 6: REPORT (per deliverable)

1. Write a checkpoint:
   ```bash
   cxp session checkpoint "$(cat <<'CKPT'
   ## Deliverable [N] complete: [name]

   **Files created/modified:** [list]
   **Tests:** [count] passing
   **Coverage:** [if applicable]
   **Next:** deliverable [N+1] — [name]
   CKPT
   )"
   ```
2. If more deliverables remain → repeat Phases 3-6 for the next one
3. If this is the last deliverable → proceed to Phase 7 (DEPLOY)

## Phase 7: DEPLOY (once, after all deliverables)

All deliverables are verified. Now build, install, and confirm the binary works.

### Step 1: Final full verification

```bash
cd ~/github/otherjamesbrown/context-palace/cp && go test ./... && go vet ./... && go build ./...
```

If anything fails, fix it. Do NOT deploy a broken build.

### Step 2: Build and install

```bash
cd ~/github/otherjamesbrown/context-palace/cp && go build -o /Users/dev/penf-cli/cxp .
```

The binary lives at `/Users/dev/penf-cli/cxp` — this is on PATH and used by all agents
(penfold, mycroft, and you). Getting this wrong breaks everyone.

### Step 3: Verify the installed binary

```bash
cxp status
```

If `cxp status` fails, the binary is broken. **Revert immediately:**
```bash
cd ~/github/otherjamesbrown/context-palace && git checkout cp/ && go build -o /Users/dev/penf-cli/cxp ./cp/
```
Then investigate what went wrong.

### Step 4: Commit

```bash
cd ~/github/otherjamesbrown/context-palace && git add [specific files] && git commit -m "[SPEC-N] description"
```

**Stage specific files only.** Never `git add -A` or `git add .`.

### Step 5: Push and report

```bash
cd ~/github/otherjamesbrown/context-palace && git push
```

Send resolution to penfold:
```sql
SELECT send_message('penfold', 'agent-cxp', ARRAY['agent-penfold'],
  'Resolved: [SPEC-N title]',
  E'## Results\n\n- Files: [count] created, [count] modified\n- Tests: [count] passing\n- Coverage: [stats]\n- Binary: installed at /Users/dev/penf-cli/cxp\n- Commit: [hash]\n\n## Evidence\n\n[paste test output + cxp status output]\n\n## Remaining\n\n[any deferred items, or "None"]',
  NULL, NULL, NULL, NULL, 'done');
```

**CHECKPOINT after Phase 7.**

## Mandatory Checkpoint Format

After every phase (not just deliverables), write a checkpoint:

```bash
cxp session checkpoint "Phase [N] ([name]) complete. [1-2 sentence summary]. Next: [what]."
```

This is non-negotiable. If your session dies, the next session reads checkpoints to resume.

## Context Budget Management

You are a single agent — no sub-agents to delegate to. Manage your context:

| Budget Used | Action |
|------------|--------|
| < 50% | Normal operation |
| 50-75% | Checkpoint more frequently, skip verbose output |
| 75-90% | Finish current deliverable, checkpoint, stop |
| > 90% | STOP immediately. Checkpoint what's done. Tell penfold what remains. |

**Signs you're running low:**
- You're forgetting earlier parts of the spec
- You're re-reading files you already read
- Your responses are getting shorter or less precise

When stopping early, always:
1. Ensure current code compiles (`go build ./...`)
2. Ensure existing tests still pass (`go test ./...`)
3. Checkpoint with explicit "remaining work" list
4. Message penfold with what's done and what's left

## Error Handling

| Failure | Action |
|---------|--------|
| Test won't compile | Add minimal stubs, don't skip the test |
| Test fails unexpectedly | Re-read spec — your understanding may be wrong |
| go vet warning | Fix it immediately, don't defer |
| Import cycle | Restructure — don't ignore |
| Existing tests break | You changed behavior — revert and reconsider approach |
| Spec is ambiguous | Message penfold asking for clarification. Don't guess. |
| Spec is wrong | Message penfold with evidence. Don't silently deviate. |

## Definition of Done

A deliverable is done when:
1. All spec'd tests for it pass
2. No existing tests broken
3. `go vet ./...` clean
4. `go build ./...` clean
5. Coverage targets met (if specified)
6. Checkpoint written

The full spec is done when:
1. All deliverables complete (or explicitly deferred with penfold's agreement)
2. Full `go test ./...` passes
3. Coverage report generated
4. Binary built and installed at `/Users/dev/penf-cli/cxp`
5. `cxp status` confirms binary works
6. Changes committed and pushed
7. Resolution message sent to penfold with test output + `cxp status` evidence

## Key Principles

1. **Test-first** — write failing tests before implementation. Always.
2. **One deliverable at a time** — don't scatter across the spec
3. **Verify continuously** — `go test` after every meaningful change
4. **Checkpoint obsessively** — your session can die at any time
5. **Don't guess** — if the spec is unclear, ask penfold
6. **Don't gold-plate** — implement what the spec says, nothing more
7. **Don't change signatures** — add new functions, don't modify existing ones
8. **Evidence over claims** — paste test output, don't say "tests pass"
9. **Stop clean** — if you must stop, leave the codebase in a buildable state
10. **Read pf-rules** — project conventions exist for a reason
