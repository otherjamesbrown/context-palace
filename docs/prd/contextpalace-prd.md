# ContextPalace (cxp) - Product Requirements Document

## Overview

ContextPalace is a CLI tool that provides AI coding agents with persistent, retrievable project context. It solves the problem of agents forgetting critical project knowledge mid-session by providing small, focused memory files triggered by specific action types.

## Problem Statement

AI coding agents lose project knowledge during long sessions. Symptoms include:

- Suggesting Go version downgrades when the project requires 1.22
- Asking "is there a CI workflow?" when CI/CD is documented
- Forgetting to use `make build` instead of `go build`
- Missing deployment processes that are documented in markdown files

The root cause: context windows are finite, early context gets compacted, and agents don't re-retrieve knowledge at decision points.

Existing solutions (extensive CLAUDE.md, docs folders) don't scale because:

- Large files swamp the context
- Agents don't know which docs to load for which tasks
- No trigger reminds agents to re-read at the right moment

## Solution

Small, focused memory files (called "actions") that agents retrieve before specific task types.

```bash
cxp action ci-cd
```

Returns 10-15 lines of CI/CD knowledge. Agent reads this before any CI/CD work, preventing mistakes.

## Core Concepts

### Actions

An action is a YAML file containing focused project knowledge for a specific task type.

```yaml
# .cxp/actions/build.yaml

commands:
  build: "make build"
  test: "make test"
  lint: "make lint"

rules:
  - Never use 'go build' directly
  - Never use 'go test' directly
  - Run 'make lint' before committing

footguns:
  - "'go run main.go' won't include build flags"
```

### Nested Actions

Actions can have children for progressive disclosure:

```
ci-cd              → Overview (10 lines)
ci-cd.argocd       → ArgoCD detail (15 lines)
ci-cd.github       → GitHub Actions detail (15 lines)
ci-cd.docker       → Docker build detail (10 lines)
```

Agent loads parent first. If more detail needed, loads specific child.

### Source Links

Actions can link to source documents for deeper context:

```yaml
# .cxp/actions/ci-cd.yaml

source_doc: docs/platform/ci-cd.md

summary:
  workflow:
    - "Push to main triggers GitHub Action"
    - "Push to deploy branch triggers ArgoCD"
```

If summary isn't sufficient, agent reads full source doc.

### Triggers

CLAUDE.md maps task types to action commands:

```markdown
## Context Management

| Action | Command |
|--------|---------|
| CI/CD changes | `cxp action ci-cd` |
| Build/test/deploy | `cxp action build` |
| Package versions | `cxp action package-versions` |
```

Agent reads this at session start, knows to run commands before specific tasks.

## Directory Structure

```
.cxp/
├── config.yaml           # CXP configuration
├── actions/
│   ├── build.yaml
│   ├── ci-cd.yaml
│   ├── ci-cd.argocd.yaml
│   ├── ci-cd.github.yaml
│   ├── ci-cd.docker.yaml
│   └── package-versions.yaml
└── logs/
    ├── access.jsonl      # Action reads
    └── writes.jsonl      # Action creates/updates
```

### config.yaml

```yaml
# CXP configuration

# Path to CLAUDE.md (relative to project root)
claude_md: "CLAUDE.md"

# Size warning thresholds
limits:
  action_lines: 30          # Warn if action exceeds this
  trigger_rows: 20          # Warn if CLAUDE.md table exceeds this

# Logging
logging:
  enabled: true
  access_log: "logs/access.jsonl"
  writes_log: "logs/writes.jsonl"
```

## Release Roadmap

### v1.0 - Core

Minimum viable tool to test the hypothesis: focused, retrievable context prevents agent mistakes.

**Features:**

- Initialize .cxp directory
- Create and show actions (with nested support)
- List all actions (tree view)
- Add triggers to CLAUDE.md
- Validate structure
- Log all access for analysis

**Commands:**

```bash
cxp init                            # Initialize .cxp directory
cxp init --claude-md <path>         # Initialize with custom CLAUDE.md location
cxp action <n>                      # Show action content
cxp action <n> --depth all          # Show action + all children
cxp action <n> --json               # Output as JSON
cxp actions                         # List all actions (tree view)
cxp actions --json                  # List as JSON array
cxp create <n>                      # Create action file (template)
cxp create <n> --parent <p>         # Create child action
cxp create <n> --edit               # Create and open in $EDITOR
cxp create <n> --json               # Output creation result as JSON
cxp add-trigger <n> <desc>          # Add trigger to CLAUDE.md
cxp add-trigger <n> <desc> --json   # Output result as JSON
cxp lint                            # Validate structure and links
cxp lint --json                     # Output errors as JSON
cxp log                             # Show recent access log
cxp log --tail                      # Follow log in real-time
cxp log --json                      # Output as JSON
```

All commands that produce output support `--json` for agent/programmatic consumption.

### v1.1 - Bootstrap & Stats

Reduce friction for new projects and understand usage patterns.

**Features:**

- Scan existing docs to suggest initial actions
- Usage statistics per action
- Identify high/low usage patterns

**Commands:**

```bash
cxp init --scan                     # Initialize + scan for memories
cxp stats                           # Usage statistics overview
cxp stats --action <name>           # Stats for specific action
```

### v1.2 - Lifecycle Management

Maintain action quality over time.

**Features:**

- Archive unused actions
- Staleness detection (source doc changed since action updated)
- Suggestions for split/merge/archive

**Commands:**

```bash
cxp archive <name>                  # Move to .cxp/archived/
cxp suggest                         # Archive/split/merge suggestions
cxp stale                           # List actions with changed source docs
```

## Technical Specifications

### v1.0 Specifications

#### Action File Format

```yaml
# Required for child actions
parent: <parent-name>               # e.g., "ci-cd" for ci-cd.argocd

# Optional source document
source_doc: <relative-path>         # e.g., "docs/platform/ci-cd.md"

# Content (flexible structure)
summary: |
  Free-form summary text
  
commands:
  <name>: <command>
  
rules:
  - Rule 1
  - Rule 2
  
footguns:
  - Common mistake 1
  - Common mistake 2

# For parent actions with children
children:
  - <child-name>: <description>
```

#### Log Format

Access log (.cxp/logs/access.jsonl):

```jsonl
{"ts": "2025-12-03T10:15:00Z", "action": "ci-cd", "depth": "single"}
{"ts": "2025-12-03T10:22:00Z", "action": "build", "depth": "single"}
{"ts": "2025-12-03T14:45:00Z", "action": "ci-cd", "depth": "all"}
```

Write log (.cxp/logs/writes.jsonl):

```jsonl
{"ts": "2025-12-03T09:00:00Z", "op": "create", "action": "ci-cd"}
{"ts": "2025-12-03T09:05:00Z", "op": "create", "action": "ci-cd.argocd", "parent": "ci-cd"}
{"ts": "2025-12-03T11:30:00Z", "op": "update", "action": "build"}
```

#### JSON Output Format

All commands support `--json` for programmatic consumption.

**cxp action ci-cd --json**

```json
{
  "name": "ci-cd",
  "path": ".cxp/actions/ci-cd.yaml",
  "parent": null,
  "source_doc": "docs/platform/ci-cd.md",
  "children": ["ci-cd.argocd", "ci-cd.github", "ci-cd.docker"],
  "content": {
    "summary": "CI runs on GitHub Actions, deploys via ArgoCD",
    "rules": ["Deploy branch does not trigger builds"],
    "footguns": ["Pushing to deploy before main = stale container"]
  }
}
```

**cxp actions --json**

```json
{
  "actions": [
    {"name": "build", "parent": null, "children": []},
    {"name": "ci-cd", "parent": null, "children": ["ci-cd.argocd", "ci-cd.docker", "ci-cd.github"]},
    {"name": "ci-cd.argocd", "parent": "ci-cd", "children": []},
    {"name": "ci-cd.docker", "parent": "ci-cd", "children": []},
    {"name": "ci-cd.github", "parent": "ci-cd", "children": []},
    {"name": "package-versions", "parent": null, "children": []}
  ]
}
```

**cxp create ci-cd --json**

```json
{
  "created": true,
  "name": "ci-cd",
  "path": ".cxp/actions/ci-cd.yaml"
}
```

**cxp lint --json**

```json
{
  "valid": false,
  "errors": [
    {"action": "ci-cd.argocd", "field": "parent", "error": "parent 'ci-cd' not found", "severity": "error"},
    {"action": "build", "field": "source_doc", "error": "file 'docs/build.md' not found", "severity": "warning"}
  ],
  "warnings": [
    {"action": "architecture", "warning": "action exceeds 30 lines (42 lines)"}
  ]
}
```

**cxp log --json**

```json
{
  "entries": [
    {"ts": "2025-12-03T10:15:00Z", "action": "ci-cd", "depth": "single"},
    {"ts": "2025-12-03T10:22:00Z", "action": "build", "depth": "single"}
  ]
}
```

#### Size Warnings

The CLI warns when content exceeds recommended limits:

- **Action files**: Warn if > 30 lines (configurable via `config.yaml`)
- **CLAUDE.md table**: Warn if > 20 trigger rows (configurable via `config.yaml`)

Warnings appear on relevant commands (`cxp lint`, `cxp action`, `cxp add-trigger`) but do not prevent operation.

```bash
$ cxp action ci-cd
Warning: Action 'ci-cd' exceeds recommended size (42 lines > 30 lines).
Consider splitting into child actions.

summary: |
  CI runs on GitHub Actions...
```

```bash
$ cxp add-trigger security "Security-related changes"
Warning: CLAUDE.md context table has 21 rows (recommended max: 20).
Consider consolidating triggers or using parent actions.

Added trigger to CLAUDE.md
```

#### CLI Behavior

**cxp init**

- Creates .cxp directory structure
- Creates empty config.yaml
- Creates logs directory
- Idempotent (safe to run multiple times)

**cxp action \<name\>**

- Reads .cxp/actions/<name>.yaml
- Supports dot notation: `ci-cd.argocd` reads `ci-cd.argocd.yaml`
- With `--depth all`: concatenates parent + all children
- Logs access to access.jsonl
- Exits with error if action doesn't exist

**cxp actions**

- Lists all actions in tree format:
  ```
  build
  ci-cd
  ├── ci-cd.argocd
  ├── ci-cd.docker
  └── ci-cd.github
  package-versions
  ```
- Sorted alphabetically, children under parents

**cxp create \<name\>**

- Creates .cxp/actions/<name>.yaml with template
- With `--parent`: adds parent field, validates parent exists
- Opens file path for agent to write content
- Logs to writes.jsonl

**cxp add-trigger \<name\> \<description\>**

- Appends row to CLAUDE.md context management table
- Creates table if doesn't exist
- Format: `| <description> | \`cxp action <name>\` |`

**cxp lint**

- Validates all action files parse correctly
- Checks parent references exist
- Checks source_doc paths exist
- Checks children listed in parent exist
- Reports errors, exits non-zero if any

**cxp log**

- Shows last 20 entries from access.jsonl
- With `--tail`: follows log (like tail -f)
- With `--writes`: shows writes.jsonl instead

#### Error Handling

- Action not found: exit 1, message "Action '<name>' not found. Run 'cxp actions' to list available."
- Invalid YAML: exit 1, message "Failed to parse <file>: <error>"
- Parent not found: exit 1, message "Parent action '<parent>' not found"
- Source doc not found: warning (not error), message "Warning: source_doc '<path>' not found"

### Non-Functional Requirements

- Single binary, no dependencies
- Fast startup (<50ms)
- Works offline
- Git-friendly (all files are text, merge-safe)

## Success Criteria

### v1.0

1. Agent stops making previously-seen mistakes (Go version, CI/CD process, build commands)
2. Actions are actually being read (visible in logs)
3. Creating new actions takes <2 minutes
4. Action retrieval adds <100ms to agent workflow

### v1.1

1. New project bootstrap takes <10 minutes
2. Stats reveal which actions are valuable vs unused

### v1.2

1. Stale actions are identified before they cause problems
2. Action count stays manageable (archive removes cruft)

## Implementation Guide

This section provides the technical details needed to implement ContextPalace in Go.

### Project Structure

```
context-palace/
├── cmd/
│   └── cxp/
│       └── main.go                 # CLI entry point, command routing
├── internal/
│   ├── action/
│   │   ├── action.go               # Action struct, YAML parsing
│   │   ├── show.go                 # cxp action command
│   │   ├── list.go                 # cxp actions command
│   │   └── create.go               # cxp create command
│   ├── config/
│   │   └── config.go               # Config loading, .cxp directory handling
│   ├── trigger/
│   │   └── trigger.go              # cxp add-trigger command, CLAUDE.md parsing
│   ├── lint/
│   │   └── lint.go                 # cxp lint command
│   ├── logging/
│   │   └── logging.go              # Access/write logging, cxp log command
│   └── output/
│       └── output.go               # JSON/text output formatting
├── go.mod
├── go.sum
└── README.md
```

### Dependencies

```go
// go.mod
module github.com/otherjamesbrown/context-palace

go 1.22

require (
    github.com/spf13/cobra v1.8.0      // CLI framework
    gopkg.in/yaml.v3 v3.0.1            // YAML parsing
)
```

Use only these dependencies. No other external packages.

### Core Types

```go
// internal/action/action.go

package action

import "time"

// Action represents a parsed action YAML file
type Action struct {
    Name      string                 `yaml:"-" json:"name"`
    Path      string                 `yaml:"-" json:"path"`
    Parent    string                 `yaml:"parent,omitempty" json:"parent"`
    SourceDoc string                 `yaml:"source_doc,omitempty" json:"source_doc"`
    Children  []string               `yaml:"-" json:"children"`
    Content   map[string]interface{} `yaml:",inline" json:"content"`
}

// ActionMeta is used for listing actions without full content
type ActionMeta struct {
    Name     string   `json:"name"`
    Parent   *string  `json:"parent"`
    Children []string `json:"children"`
}
```

```go
// internal/config/config.go

package config

// Config represents .cxp/config.yaml
type Config struct {
    ClaudeMD string       `yaml:"claude_md"`
    Limits   LimitsConfig `yaml:"limits"`
    Logging  LogConfig    `yaml:"logging"`
}

type LimitsConfig struct {
    ActionLines int `yaml:"action_lines"`
    TriggerRows int `yaml:"trigger_rows"`
}

type LogConfig struct {
    Enabled   bool   `yaml:"enabled"`
    AccessLog string `yaml:"access_log"`
    WritesLog string `yaml:"writes_log"`
}

// DefaultConfig returns config with default values
func DefaultConfig() Config {
    return Config{
        ClaudeMD: "CLAUDE.md",
        Limits: LimitsConfig{
            ActionLines: 30,
            TriggerRows: 20,
        },
        Logging: LogConfig{
            Enabled:   true,
            AccessLog: "logs/access.jsonl",
            WritesLog: "logs/writes.jsonl",
        },
    }
}
```

```go
// internal/logging/logging.go

package logging

import "time"

// AccessEntry represents a single access log entry
type AccessEntry struct {
    Timestamp time.Time `json:"ts"`
    Action    string    `json:"action"`
    Depth     string    `json:"depth"` // "single" or "all"
}

// WriteEntry represents a single write log entry
type WriteEntry struct {
    Timestamp time.Time `json:"ts"`
    Operation string    `json:"op"`     // "create" or "update"
    Action    string    `json:"action"`
    Parent    string    `json:"parent,omitempty"`
}
```

```go
// internal/lint/lint.go

package lint

// LintResult represents the result of linting all actions
type LintResult struct {
    Valid    bool          `json:"valid"`
    Errors   []LintError   `json:"errors"`
    Warnings []LintWarning `json:"warnings"`
}

type LintError struct {
    Action   string `json:"action"`
    Field    string `json:"field,omitempty"`
    Error    string `json:"error"`
    Severity string `json:"severity"` // "error" or "warning"
}

type LintWarning struct {
    Action  string `json:"action"`
    Warning string `json:"warning"`
}
```

### Command Implementation Details

#### cxp init

```go
// Flags
--claude-md string    // Custom CLAUDE.md path (default: "CLAUDE.md")

// Behavior
1. Check if .cxp/ already exists
   - If exists and not empty: print "Already initialized" and exit 0
2. Create directory structure:
   - .cxp/
   - .cxp/actions/
   - .cxp/logs/
3. Create .cxp/config.yaml with defaults (or custom claude_md if flag provided)
4. Print "Initialized ContextPalace in .cxp/"

// Exit codes
0: Success (including already initialized)
1: Error (permission denied, etc.)
```

#### cxp action \<name\>

```go
// Flags
--depth string   // "single" (default) or "all"
--json bool      // Output as JSON

// Behavior
1. Parse action name (handle dot notation: "ci-cd.argocd")
2. Construct path: .cxp/actions/{name}.yaml
3. Check file exists, exit 1 if not
4. Parse YAML
5. If --depth all:
   - Find all children (files matching {name}.*.yaml)
   - Parse and concatenate
6. Log access to access.jsonl (if logging enabled)
7. Check line count, warn if exceeds limit
8. Output as YAML (default) or JSON (--json)

// Dot notation
"ci-cd" -> .cxp/actions/ci-cd.yaml
"ci-cd.argocd" -> .cxp/actions/ci-cd.argocd.yaml

// Exit codes
0: Success
1: Action not found or parse error
```

#### cxp actions

```go
// Flags
--json bool      // Output as JSON

// Behavior
1. List all .yaml files in .cxp/actions/
2. Parse each to extract parent field
3. Build tree structure
4. Output as tree (default) or JSON (--json)

// Tree output format
build
ci-cd
├── ci-cd.argocd
├── ci-cd.docker
└── ci-cd.github
package-versions

// Exit codes
0: Success
1: Error reading directory
```

#### cxp create \<name\>

```go
// Flags
--parent string  // Parent action name
--edit bool      // Open in $EDITOR after creation
--json bool      // Output as JSON

// Behavior
1. Validate name (alphanumeric, hyphens, dots only)
2. If --parent provided:
   - Validate parent exists
   - Validate name starts with parent prefix (e.g., parent="ci-cd" name must be "ci-cd.something")
3. Construct path: .cxp/actions/{name}.yaml
4. Check file doesn't already exist
5. Create file with template:
   
   # Action: {name}
   {parent: {parent}  # only if --parent provided}
   
   summary: |
     TODO: Add summary
   
   rules:
     - TODO: Add rules
   
   footguns:
     - TODO: Add common mistakes

6. Log write to writes.jsonl
7. If --edit: open $EDITOR (respect $EDITOR env var, default to "vi")
8. Output confirmation or JSON

// Exit codes
0: Success
1: Validation error, file exists, or parent not found
```

#### cxp add-trigger \<name\> \<description\>

```go
// Flags
--json bool      // Output as JSON

// Behavior
1. Validate action exists
2. Read CLAUDE.md (path from config)
3. Find "## Context Management" section
   - If not found, append new section
4. Find markdown table in that section
   - If not found, create new table with header
5. Check row count, warn if exceeds limit
6. Append new row: | {description} | `cxp action {name}` |
7. Write CLAUDE.md
8. Output confirmation or JSON

// Table format to find/create
| Action | Command |
|--------|---------|
| Existing trigger | `cxp action existing` |

// Exit codes
0: Success
1: Action not found, CLAUDE.md not found, or write error
```

#### cxp lint

```go
// Flags
--json bool      // Output as JSON

// Behavior
1. List all actions
2. For each action:
   - Validate YAML parses correctly
   - If parent field set, validate parent file exists
   - If source_doc field set, validate file exists (warning, not error)
   - If children field set, validate each child file exists
   - Check line count against limit
3. Check CLAUDE.md table row count against limit
4. Collect all errors and warnings
5. Output human-readable (default) or JSON (--json)
6. Exit 1 if any errors, exit 0 if only warnings

// Exit codes
0: All valid (may have warnings)
1: Has errors
```

#### cxp log

```go
// Flags
--tail bool      // Follow log in real-time
--writes bool    // Show writes.jsonl instead of access.jsonl
--json bool      // Output as JSON
-n int           // Number of entries (default 20)

// Behavior
1. Determine log file (access.jsonl or writes.jsonl)
2. If --tail:
   - Open file, seek to end
   - Watch for changes, print new lines
3. Else:
   - Read last N entries
   - Output formatted (default) or JSON (--json)

// Default output format (access)
2025-12-03 10:15:00  ci-cd          (single)
2025-12-03 10:22:00  build          (single)
2025-12-03 14:45:00  ci-cd          (all)

// Exit codes
0: Success
1: Log file not found or read error
```

### Output Formatting

```go
// internal/output/output.go

package output

import (
    "encoding/json"
    "fmt"
    "os"
)

// Print outputs data as JSON or formatted text
func Print(data interface{}, asJSON bool) error {
    if asJSON {
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(data)
    }
    // Type switch for text formatting
    switch v := data.(type) {
    case *action.Action:
        return printActionYAML(v)
    case []ActionMeta:
        return printActionTree(v)
    // ... etc
    }
    return nil
}

// PrintWarning prints a warning to stderr
func PrintWarning(format string, args ...interface{}) {
    fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}

// PrintError prints an error to stderr
func PrintError(format string, args ...interface{}) {
    fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
```

### Error Messages

Use consistent error message format:

```
Error: Action 'ci-cd' not found. Run 'cxp actions' to list available.
Error: Parent action 'ci-cd' not found.
Error: Failed to parse .cxp/actions/build.yaml: yaml: line 5: mapping values are not allowed here
Error: CLAUDE.md not found at expected path: CLAUDE.md
Warning: Action 'ci-cd' exceeds recommended size (42 lines > 30 lines). Consider splitting into child actions.
Warning: source_doc 'docs/build.md' not found for action 'build'.
Warning: CLAUDE.md context table has 21 rows (recommended max: 20). Consider consolidating triggers.
```

### Testing Strategy

```
internal/
├── action/
│   ├── action_test.go      # Unit tests for Action parsing
│   ├── show_test.go        # Unit tests for show command
│   └── testdata/           # Test YAML files
│       ├── valid.yaml
│       ├── invalid.yaml
│       └── nested/
├── config/
│   └── config_test.go
└── ...

cmd/cxp/
└── integration_test.go     # Integration tests for full CLI
```

Test cases to cover:

1. **init**: Fresh init, already initialized, custom claude-md path
2. **action**: Valid action, missing action, nested action, --depth all, --json
3. **actions**: Empty, with hierarchy, --json
4. **create**: New action, with parent, invalid name, already exists, --edit
5. **add-trigger**: New table, existing table, action not found, table too large
6. **lint**: All valid, missing parent, missing source_doc, oversized actions
7. **log**: Normal output, --tail, --writes, --json, empty log

### Build and Release

```makefile
# Makefile

BINARY=cxp
VERSION?=0.1.0

.PHONY: build test clean install

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/cxp

test:
	go test -v ./...

clean:
	rm -f $(BINARY)

install: build
	mv $(BINARY) $(GOPATH)/bin/
```

### Example Session

```bash
# Initialize
$ cxp init
Initialized ContextPalace in .cxp/

# Create an action
$ cxp create build
Created .cxp/actions/build.yaml

# Edit the action (manually or agent writes content)
$ cat .cxp/actions/build.yaml
summary: |
  Build commands for the project

commands:
  build: "make build"
  test: "make test"

rules:
  - Never use 'go build' directly

# Add trigger to CLAUDE.md
$ cxp add-trigger build "Build, test, or run commands"
Added trigger to CLAUDE.md

# View action
$ cxp action build
summary: |
  Build commands for the project
...

# View as JSON
$ cxp action build --json
{
  "name": "build",
  "path": ".cxp/actions/build.yaml",
  ...
}

# List all actions
$ cxp actions
build
ci-cd
├── ci-cd.argocd
└── ci-cd.github

# Lint
$ cxp lint
✓ All actions valid

# View access log
$ cxp log
2025-12-03 10:15:00  build  (single)
```

## Design Decisions

1. **CLAUDE.md location**: Assumes project root. Override with `cxp init --claude-md <path>`.

2. **Action size limits**: Warn if action exceeds 30 lines. Warn if CLAUDE.md context table exceeds 20 rows. Configurable in config.yaml.

3. **Monorepo support**: Single .cxp at project root. Service-specific knowledge uses named actions (e.g., `user-management`, `billing`). Nested .cxp directories out of scope for v1.

4. **Editor integration**: `cxp create` creates template file. Use `--edit` flag to open in $EDITOR.

5. **JSON output**: All commands support `--json` flag for agent/programmatic consumption.

## Open Questions

1. **Conflict with existing tools**: Verify `cxp` command name is available on target systems.

2. **Size thresholds**: Are 30 lines (action) and 20 rows (CLAUDE.md table) the right limits?

## Appendix: Example Workflow

### Agent Makes Mistake

```
User: "Deploy the changes"
Agent: *pushes to deploy branch*
Agent: "Container is stale. Want me to trigger a build, or is there CI?"

User: "Create me a cxp so you remember the CI/CD process"
```

### Agent Creates Memory

```
Agent: I'll create a CI/CD action from your documentation.

$ cxp create ci-cd
Created .cxp/actions/ci-cd.yaml

*Agent reads docs/platform/ci-cd.md*
*Agent writes summary to ci-cd.yaml*

$ cxp add-trigger ci-cd "CI/CD or deployment changes"
Added trigger to CLAUDE.md

Done. Next time I'll run 'cxp action ci-cd' before deployment work.
```

### Agent Uses Memory

```
User: "Deploy the new feature"

Agent: Let me check the CI/CD process first.

$ cxp action ci-cd

*sees: "Deploy branch does NOT trigger builds. Push to main first."*

I'll push to main first to trigger the image build, wait for it to complete,
then push to the deploy branch for ArgoCD to sync. 

*does it correctly*
```

## Appendix: CLAUDE.md Integration

Recommended CLAUDE.md section:

```markdown
## Context Management

This project uses ContextPalace for persistent memory.

Before taking these actions, run the corresponding command:

| Action | Command |
|--------|---------|
| CI/CD or deployment changes | `cxp action ci-cd` |
| Build, test, or run commands | `cxp action build` |
| Changing package versions | `cxp action package-versions` |
| ArgoCD configuration | `cxp action ci-cd.argocd` |

Run `cxp actions` to see all available context.

When you make a mistake due to missing context, offer to create a new action
with `cxp create <name>` to prevent the same mistake in future.
```
