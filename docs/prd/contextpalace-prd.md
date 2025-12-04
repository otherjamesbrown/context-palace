## `cxp ingest` Command Specification

Add this command to v1.0 alongside the other commands.

---

### Overview

`cxp ingest` is a guided workflow for creating actions from mistakes. When an agent makes an error and the user says "create a cxp so you don't do this again", the agent runs `cxp ingest` which walks through a structured process to analyze the failure and create an action.

### Command

```bash
cxp ingest                          # Interactive guided flow
cxp ingest --category <name>        # Pre-fill category
cxp ingest --json                   # Output result as JSON
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
| `--category` | Action category (existing or new) |
| `--what` | What happened / the mistake |
| `--cause` | Root cause analysis |
| `--correct` | Correct approach |
| `--trigger` | CLAUDE.md trigger description |
| `--json` | Output as JSON |
| `--no-confirm` | Skip confirmation, write immediately |

### Interactive Flow

When run without flags, `cxp ingest` outputs prompts that the agent reads and responds to:

```
=== ContextPalace: Create Action from Experience ===

Answer these questions to create a new action:

1. CATEGORY: What type of task was this?
   (Existing: build, ci-cd, deploy | Or enter new category)
   > build

2. WHAT HAPPENED: Brief description of the mistake
   > Ran "go build -o bin/cli ./cmd/cli" but got archive file instead of executable

3. ROOT CAUSE: Why did it go wrong?
   > main.go is at project root, not in cmd/ subdirectory. Standard layout assumption was wrong.

4. CORRECT APPROACH: What should be done instead?
   > Use "go build -o bin/cli ." to build from current directory

5. TRIGGER: When should this action be consulted?
   (This becomes the CLAUDE.md entry)
   > Building Go binaries in this project
```

### Preview and Confirmation

After gathering input, `cxp ingest` shows a preview:

```
=== Preview ===

Action: build
File: .cxp/actions/build.yaml
Status: UPDATE (adding to existing action)

New content to add:

  rules:
    - main.go is at project root, not in cmd/
    - Always build with "go build -o bin/<name> ." not "./cmd/..."
    
  footguns:
    - "go build -o bin/cli ./cmd/cli" creates ar archive, not executable
    - Check with "file bin/cli" - should say "ELF executable", not "ar archive"

  commands:
    build: "go build -o bin/ai-aas-cli ."
    build_verbose: "go build -v -o bin/ai-aas-cli ."

CLAUDE.md update:
  | Building Go binaries in this project | `cxp action build` |

Create/update this action? [y/n] > y

✓ Updated .cxp/actions/build.yaml
✓ Added trigger to CLAUDE.md
✓ Logged to writes.jsonl
```

### Behavior Details

1. **Category resolution**:
   - If category exists, action is UPDATED (new content merged)
   - If category is new, action is CREATED
   - Dot notation for children: `--category ci-cd.docker` creates child action

2. **Content generation**:
   - `what` → becomes a `footguns` entry
   - `cause` → becomes a `rules` entry explaining why
   - `correct` → becomes `commands` and/or `rules` entries
   - `trigger` → becomes CLAUDE.md table row

3. **Duplicate detection**:
   - If CLAUDE.md already has trigger for this category, skip adding
   - Warn: "Trigger for 'build' already exists in CLAUDE.md"

4. **Logging**:
   - Logs to writes.jsonl: `{"ts": "...", "op": "ingest", "action": "build", "source": "interactive"}`

### JSON Output

```bash
$ cxp ingest --category build --what "..." --cause "..." --correct "..." --trigger "..." --no-confirm --json
```

```json
{
  "action": "build",
  "path": ".cxp/actions/build.yaml",
  "operation": "update",
  "added": {
    "rules": ["main.go is at project root, not in cmd/"],
    "footguns": ["go build -o bin/cli ./cmd/cli creates ar archive"],
    "commands": {"build": "go build -o bin/ai-aas-cli ."}
  },
  "trigger_added": true,
  "trigger_description": "Building Go binaries in this project"
}
```

### Error Cases

| Condition | Behavior |
|-----------|----------|
| `--no-confirm` without all required flags | Exit 1: "Missing required flags for non-interactive mode" |
| Parent doesn't exist for child category | Exit 1: "Parent action 'ci-cd' not found" |
| CLAUDE.md not found | Warn and skip trigger: "Warning: CLAUDE.md not found, skipping trigger" |
| Action file not writable | Exit 1: "Cannot write to .cxp/actions/build.yaml: permission denied" |

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

Agent confirms: "Created build action. Next time I build Go binaries, I'll run 
'cxp action build' first to check the correct command."
```

### Example: Full Interactive Session

```
$ cxp ingest

=== ContextPalace: Create Action from Experience ===

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

Action: build
File: .cxp/actions/build.yaml
Status: CREATE (new action)

Content:

summary: |
  Go build configuration for this project. Binary builds from
  project root, not cmd/ subdirectory.

commands:
  build: "go build -o bin/ai-aas-cli ."
  build_verbose: "go build -v -o bin/ai-aas-cli ."
  verify: "file bin/ai-aas-cli"

rules:
  - main.go is at project root, not in cmd/ subdirectory
  - Always build with "go build -o bin/<name> ." (dot = current dir)
  - Verify binary type with "file" command after build

footguns:
  - "go build -o bin/cli ./cmd/cli" creates ar archive, not executable
  - Repeated build attempts won't fix wrong path - check main.go location
  - If "file" shows "ar archive", the build path is wrong

CLAUDE.md addition:
  | Building Go binaries in this project | `cxp action build` |

Create this action? [y/n] > y

✓ Created .cxp/actions/build.yaml
✓ Added trigger to CLAUDE.md
✓ Logged to writes.jsonl

Done. Run 'cxp action build' before building to see this context.
```

---

## Add to Commands Section

In the v1.0 Commands list, add:

```bash
cxp ingest                          # Guided flow: create action from mistake
cxp ingest --category <n>           # Pre-fill category
cxp ingest --json                   # Output as JSON
cxp ingest --no-confirm             # Skip confirmation
```

## Add to Implementation Guide

### cxp ingest

```go
// Flags
--category string   // Pre-fill category
--what string       // What happened
--cause string      // Root cause
--correct string    // Correct approach
--trigger string    // CLAUDE.md trigger text
--json bool         // Output as JSON
--no-confirm bool   // Skip confirmation

// Behavior
1. If all content flags provided (what, cause, correct, trigger):
   - Non-interactive mode
   - Require --category
2. Else:
   - Interactive mode: print prompts, read from stdin
3. Parse existing action if category exists (for merge)
4. Generate new content from inputs
5. Show preview (unless --no-confirm)
6. On confirm:
   - Write/update action YAML
   - Update CLAUDE.md (if trigger provided and not duplicate)
   - Log to writes.jsonl
7. Output confirmation or JSON

// Exit codes
0: Success
1: Missing flags, parent not found, write error
```