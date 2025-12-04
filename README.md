# Context-Palace (cxp) 🏰

**A CLI tool that provides AI coding agents with persistent, retrievable project context.**

ContextPalace solves the problem of agents forgetting critical project knowledge during long sessions by providing small, focused memory files (memos) triggered by specific task types.

## The Problem 🤔

AI coding agents lose project knowledge during long sessions. Common symptoms include:

- ❌ Suggesting Go version downgrades when the project requires 1.22
- ❓ Asking "is there a CI workflow?" when CI/CD is documented
- 🔨 Forgetting to use `make build` instead of `go build`
- 📝 Missing deployment processes that are documented in markdown files

The root cause: context windows are finite, early context gets compacted, and agents don't re-retrieve knowledge at decision points.

## The Solution ✨

Small, focused memory files (called "memos") that agents retrieve before specific task types.

```bash
cxp memo ci-cd
```

Returns 10-15 lines of CI/CD knowledge. The agent reads this before any CI/CD work, preventing mistakes.

## Usage Philosophy 🧠

ContextPalace is designed with a **"Smart Agent, Dumb Tool"** philosophy:

*   **For Agents**: The CLI is a deterministic I/O tool. Agents use it programmatically (via JSON output and flags) to "save" their lessons learned. The Agent provides the intelligence; the CLI provides the structure.
*   **For Humans**: The CLI provides interactive wizards (like `cxp ingest`) to guide humans through creating structured context without needing to remember complex flags.

## Quick Start 🚀

### Installation 📦

```bash
# Build from source
make build

# Or install directly
go install ./cmd/cxp
```

### Initialize 🎯

```bash
cxp init
```

This creates the `.cxp/` directory structure with:
- 📄 `config.yaml` - Configuration file
- 📁 `memos/` - Directory for memo files
- 📊 `logs/` - Access and write logs

### Create Your First Memo ✏️

```bash
# Create a memo
cxp create build

# Edit the memo file
# .cxp/memos/build.yaml

# Add a trigger to CLAUDE.md
cxp add-trigger build "Build, test, or run commands"
```

### Use Memos 👀

```bash
# View a memo
cxp memo build

# View with all children
cxp memo ci-cd --depth all

# List all memos
cxp memos

# View as JSON (for agents)
cxp memo build --json
```

## Core Concepts 💡

### Memos 📋

A memo is a YAML file containing focused project knowledge for a specific task type.

```yaml
# .cxp/memos/build.yaml

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

### Nested Memos 🌳

Memos can have children for progressive disclosure:

```
ci-cd              → Overview (10 lines)
ci-cd.argocd       → ArgoCD detail (15 lines)
ci-cd.github       → GitHub Actions detail (15 lines)
ci-cd.docker       → Docker build detail (10 lines)
```

The agent loads the parent first. If more detail is needed, it loads the specific child.

### Source Links 🔗

Memos can link to source documents for deeper context:

```yaml
# .cxp/memos/ci-cd.yaml

source_doc: docs/platform/ci-cd.md

summary: |
  CI runs on GitHub Actions, deploys via ArgoCD
```

If the summary isn't sufficient, the agent reads the full source doc.

### Triggers ⚡

CLAUDE.md maps task types to memo commands:

```markdown
## Context Memos

| When | Command |
|------|---------|
| CI/CD changes | `cxp memo ci-cd` |
| Build/test/deploy | `cxp memo build` |
| Package versions | `cxp memo package-versions` |
```

The agent reads this at session start and knows to run commands before specific tasks.

## Commands 🛠️

### Core Commands

- `cxp init` - 🎯 Initialize `.cxp` directory
- `cxp memo <name>` - 👀 Show memo content
- `cxp memos` - 📋 List all memos (tree view)
- `cxp create <n>` - ✏️ Create memo file (template)
- `cxp ingest` - 💡 Create memo from mistake (guided flow)
- `cxp add-trigger <n> <desc>` - ⚡ Add trigger to CLAUDE.md
- `cxp lint` - ✅ Validate structure and links
- `cxp log` - 📊 Show recent access log

### Flags

Most commands support:
- `--json` - 📦 Output as JSON for programmatic consumption
- `--depth all` - 🌳 Include all children (for `cxp memo`)
- `--parent <p>` - 👨‍👩‍👧 Create child memo (for `cxp create`)
- `--edit` - ✏️ Open in $EDITOR after creation

See the [PRD](docs/prd/contextpalace-prd.md) for complete command documentation.

## Directory Structure 📁

```
.cxp/
├── config.yaml           # CXP configuration
├── memos/
│   ├── build.yaml
│   ├── ci-cd.yaml
│   ├── ci-cd.argocd.yaml
│   └── package-versions.yaml
└── logs/
    ├── access.jsonl      # Memo reads
    └── writes.jsonl      # Memo creates/updates
```

## Example Workflow 🔄

### Agent Makes Mistake ❌

```
User: "Deploy the changes"
Agent: *pushes to deploy branch*
Agent: "Container is stale. Want me to trigger a build, or is there CI?"
```

### Agent Creates Memory 💾

```bash
$ cxp create ci-cd
Created .cxp/memos/ci-cd.yaml

# Agent writes summary to ci-cd.yaml

$ cxp add-trigger ci-cd "CI/CD or deployment changes"
Added trigger to CLAUDE.md
```

### Agent Uses Memory ✅

```bash
$ cxp memo ci-cd

# Agent sees: "Deploy branch does NOT trigger builds. Push to main first."

# Agent now does it correctly
```

## Configuration ⚙️

Edit `.cxp/config.yaml` to customize:

```yaml
# Path to CLAUDE.md (relative to project root)
claude_md: "CLAUDE.md"

# Size warning thresholds
limits:
  memo_lines: 30            # Warn if memo exceeds this
  trigger_rows: 20          # Warn if CLAUDE.md table exceeds this

# Logging
logging:
  enabled: true
  access_log: "logs/access.jsonl"
  writes_log: "logs/writes.jsonl"
```

## Requirements 📋

- 🐹 Go 1.22 or later
- 📦 Single binary, no external dependencies (except `cobra` and `yaml.v3`)

## Development 🔧

```bash
# Build
make build

# Test
make test

# Run
./cxp --help
```

## Documentation 📚

- 📄 [Product Requirements Document](docs/prd/contextpalace-prd.md) - Complete specification

## License 📜

[Add your license here]
