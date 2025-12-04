## `cxp ingest` Command Specification

Add this command to v1.0 alongside the other commands.

---

### Overview

`cxp ingest` is a guided workflow for creating memos from mistakes. When an agent makes an error and the user says "create a cxp so you don't do this again", the agent runs `cxp ingest` which walks through a structured process to analyze the failure and create a memo.

The trigger added to CLAUDE.md ensures the agent consults this memo before similar tasks in future sessions.

### Command

```bash
cxp ingest                          # Interactive guided flow
cxp ingest --category <name>        # Pre-fill category
cxp ingest --json                   # Output result as JSON
cxp ingest --dry-run                # Preview without writing
cxp ingest --no-confirm             # Skip confirmation prompt
```

### Flags for Non-Interactive Use

```bash
cxp ingest \
  --category build \
  --what "Built with wrong path, created archive not executable" \
  --cause "main.go at root not in cmd/ subdirectory" \
  --correct "Use 'go build -o bin/cli .' not './cmd/cli'" \
  --trigger "Building Go binaries in this project"
```

| Flag | Description |
|------|-------------|
| `--category` | Memo category (existing or new) |
| `--what` | The CLASS of mistake (general pattern, not specific instance) |
| `--cause` | Why this type of mistake happens |
| `--correct` | General rule to follow |
| `--trigger` | CLAUDE.md trigger description (optional; if omitted, no trigger added) |
| `--json` | Output as JSON |
| `--dry-run` | Show what would be written without making changes |
| `--no-confirm` | Skip confirmation, write immediately |

### Interactive Flow

> **Note**: Interactive mode is primarily for human use or debugging. Agents should use the flag-based non-interactive mode (all content flags + `--no-confirm`).

When run without flags, `cxp ingest` outputs prompts that the user responds to:

```
=== ContextPalace: Create Memo from Experience ===

Answer these questions to create a new memo:

1. CATEGORY: What type of task was this?
   (Existing: build, ci-cd, deploy | Or enter new category)
   > build

2. WHAT HAPPENED: Describe the CLASS of mistake, not just this instance.
   Think: what general pattern would catch this AND similar issues?
   (Bad: "go build ./cmd/cli failed" - too specific)
   (Good: "Assuming standard Go project layout without checking" - catches similar issues)
   > Assumed standard Go project layout (main.go in cmd/) without verifying actual structure

3. ROOT CAUSE: Why did this type of mistake happen?
   > Project uses non-standard layout with main.go at root. Standard assumptions don't apply.

4. CORRECT APPROACH: What's the general rule to follow?
   > Always check main.go location before running build commands. Build from the directory containing main.go.

5. TRIGGER: When should this memo be consulted?
   (This becomes the CLAUDE.md entry)
   > Building Go binaries in this project
```

### Preview and Confirmation

After gathering input, `cxp ingest` shows a preview:

```
=== Preview ===

Memo: build
File: .cxp/memos/build.yaml
Status: UPDATE (adding to existing memo)

New content to add:

  rules:
    - "Project uses non-standard layout with main.go at root. Standard assumptions don't apply."
    - "Always check main.go location before running build commands. Build from the directory containing main.go."

  footguns:
    - "Assumed standard Go project layout (main.go in cmd/) without verifying actual structure"

CLAUDE.md update:
  | Building Go binaries in this project | `cxp memo build` |

Create/update this memo? [y/n] > y

✓ Updated .cxp/memos/build.yaml
✓ Added trigger to CLAUDE.md
✓ Logged to .cxp/logs/writes.jsonl
```

### Behavior Details

1. **Category resolution**:
   - If category exists, memo is UPDATED (new content merged)
   - If category is new, memo is CREATED
   - Dot notation for children: `--category ci-cd.docker` creates child memo

2. **Merge semantics** (when updating existing memo):
   - **Arrays** (`rules`, `footguns`): Append new entries. No duplicate checking.
   - **Maps** (`commands`): Add new keys only. Existing keys are NOT overwritten; warn on conflict: "Command 'build' already exists, skipping"
   - **Strings** (`summary`): Keep existing. Warn: "Existing summary preserved. Edit manually if needed."
   - This is conservative—existing data is never lost.

3. **Memo structure**:
   Memos use a flexible structure. The fields `rules`, `footguns`, `commands`, and `summary` are conventions that `ingest` and `lint` understand, but custom fields are allowed.

4. **Content generation** (literal mapping):
   - `what` → appended to `footguns` array
   - `cause` → appended to `rules` array
   - `correct` → appended to `rules` array
   - `trigger` → becomes CLAUDE.md table row

5. **Duplicate detection**:
   - If CLAUDE.md already has trigger for this category, skip adding
   - Warn: "Trigger for 'build' already exists in CLAUDE.md"

6. **CLAUDE.md integration**:
   - Look for a markdown table with header containing "Command" (case-insensitive)
   - If found, append new row to that table
   - If no table exists, create new section at end of file:
     ```markdown
     ## Context Memos

     | When | Command |
     |------|---------|
     | <trigger description> | `cxp memo <category>` |
     ```
   - Expected header format: `| When | Command |` (or similar with "Command" column)

7. **Logging**:
   - Logs to .cxp/logs/writes.jsonl: `{"ts": "...", "op": "ingest", "memo": "build", "source": "interactive"}`

### JSON Output

```bash
$ cxp ingest --category build --what "..." --cause "..." --correct "..." --trigger "..." --no-confirm --json
```

```json
{
  "memo": "build",
  "path": ".cxp/memos/build.yaml",
  "operation": "update",
  "added": {
    "rules": ["main.go is at project root, not in cmd/"],
    "footguns": ["go build -o bin/cli ./cmd/cli creates ar archive"],
    "commands": {"build": "go build -o bin/myapp ."}
  },
  "trigger_added": true,
  "trigger_description": "Building Go binaries in this project"
}
```

### Error Cases

| Condition | Behavior |
|-----------|----------|
| `--no-confirm` without all required flags | Exit 1: "Missing required flags for non-interactive mode" |
| Parent doesn't exist for child category | Exit 1: "Parent memo 'ci-cd' not found" |
| CLAUDE.md not found | Warn and skip trigger: "Warning: CLAUDE.md not found, skipping trigger" |
| Memo file not writable | Exit 1: "Cannot write to .cxp/memos/build.yaml: permission denied" |

### Integration with Agent Workflow

Typical usage after a mistake:

```
User: "Create a cxp so you don't do this again"

Agent thinks:
- I was trying to build the Go binary
- I used the wrong build command
- The fix was to build from root

Agent runs:
$ cxp ingest --category build \
    --what "Used go build ./cmd/cli which created archive not executable" \
    --cause "main.go is at project root not cmd/ subdirectory" \
    --correct "Build with 'go build -o bin/cli .' from project root" \
    --trigger "Building Go binaries"

Agent confirms: "Created build memo. Next time I build Go binaries, I'll run
'cxp memo build' first to check the correct command."
```

### Example: Full Interactive Session

```
$ cxp ingest

=== ContextPalace: Create Memo from Experience ===

1. CATEGORY: build

2. WHAT HAPPENED: Tried "go build -o bin/cli ./cmd/cli" repeatedly,
   each attempt created an ar archive instead of executable binary.
   File command showed "current ar archive" not "ELF executable".

3. ROOT CAUSE: Assumed standard Go project layout with main.go in 
   cmd/cli/ directory. This project has main.go at repository root.
   The ./cmd/cli path doesn't contain main.go so go build created
   a library archive.

4. CORRECT APPROACH: Build from repository root using "go build -o bin/cli ."
   The dot means current directory which contains main.go.
   Can verify with "file bin/cli" which should show ELF executable.

5. TRIGGER: Building Go binaries in this project

=== Preview ===

Memo: build
File: .cxp/memos/build.yaml
Status: CREATE (new memo)

Content:

rules:
  - "Assumed standard Go project layout with main.go in cmd/cli/ directory. This project has main.go at repository root. The ./cmd/cli path doesn't contain main.go so go build created a library archive."
  - "Build from repository root using 'go build -o bin/cli .' The dot means current directory which contains main.go. Can verify with 'file bin/cli' which should show ELF executable."

footguns:
  - "Tried 'go build -o bin/cli ./cmd/cli' repeatedly, each attempt created an ar archive instead of executable binary. File command showed 'current ar archive' not 'ELF executable'."

CLAUDE.md addition:
  | Building Go binaries in this project | `cxp memo build` |

Create this memo? [y/n] > y

✓ Created .cxp/memos/build.yaml
✓ Added trigger to CLAUDE.md
✓ Logged to .cxp/logs/writes.jsonl

Done. Run 'cxp memo build' before building to see this context.
```

---

## Add to Commands Section

In the v1.0 Commands list, add:

```bash
cxp ingest                          # Guided flow: create memo from mistake
cxp ingest --category <n>           # Pre-fill category
cxp ingest --json                   # Output as JSON
cxp ingest --dry-run                # Preview without writing
cxp ingest --no-confirm             # Skip confirmation
```

## Add to Implementation Guide

### cxp ingest

```go
// Flags
--category string   // Pre-fill category
--what string       // Class of mistake
--cause string      // Why this type happens
--correct string    // General rule to follow
--trigger string    // CLAUDE.md trigger text
--json bool         // Output as JSON
--dry-run bool      // Preview without writing
--no-confirm bool   // Skip confirmation

// Behavior
1. If required content flags provided (what, cause, correct):
   - Non-interactive mode
   - Require --category
   - --trigger is optional (if omitted, skip CLAUDE.md update)
2. Else:
   - Interactive mode: print prompts, read from stdin
3. Parse existing memo if category exists (for merge)
4. Generate new content from inputs (literal mapping)
5. If --dry-run:
   - Show preview and exit (no writes)
6. Show preview (unless --no-confirm)
7. On confirm:
   - Write/update memo YAML
   - Update CLAUDE.md (if trigger provided and not duplicate)
   - Log to .cxp/logs/writes.jsonl
8. Output confirmation or JSON

// Exit codes
0: Success
1: Missing flags, parent not found, write error
```