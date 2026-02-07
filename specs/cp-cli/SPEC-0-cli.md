# SPEC-0: `cp` CLI & Configuration

**Status:** Draft
**Depends on:** Nothing
**Blocks:** All other specs

---

## Goal

Create a standalone CLI tool for Context Palace. Handles connection, authentication,
project selection, and agent identity. Migrates all Context Palace commands from `penf`.
This is the shell that all other specs build into.

## What Exists

- `palace` CLI — 5 commands: task get/claim/progress/close, artifact add
- `penf` CLI — has Context Palace commands (memory, backlog, session, message, context)
  baked in, coupled to Penfold-specific config
- Direct psql works but isn't a CLI

## What to Build

1. **Standalone Go binary** — `cp`, independent of `penf` and `palace`
2. **Project-scoped config** — `~/.cp/config.yaml` (global) + `.cp.yaml` (project)
3. **Direct DB connection** — PostgreSQL with SSL, no gateway dependency
4. **Agent identity** — configurable per-project
5. **Migrated commands** — all Context Palace commands from `penf`

## Configuration

### Global config: `~/.cp/config.yaml`

```yaml
connection:
  host: dev02.brown.chat
  database: contextpalace
  user: penfold
  sslmode: verify-full

agent: agent-penfold

embedding:
  provider: google
  model: text-embedding-004
```

### Project config: `.cp.yaml` (project root, committed to repo)

```yaml
project: penfold
agent: agent-penfold
```

### Environment variables (highest precedence)

```
CP_HOST          Override connection host
CP_DATABASE      Override database name
CP_USER          Override database user
CP_PROJECT       Override project name
CP_AGENT         Override agent identity
```

### Precedence

Environment variables > `.cp.yaml` (project) > `~/.cp/config.yaml` (global) > defaults.

## CLI Structure

```
cp
├── status              # Connection + project info
├── init                # Create .cp.yaml in current directory
├── version             # CLI version
│
├── memory              # Agent memory (from penf)
│   ├── add
│   ├── list
│   ├── search
│   ├── resolve
│   └── defer
│
├── backlog             # Dev backlog (from penf)
│   ├── add
│   ├── list
│   ├── show
│   ├── update
│   └── close
│
├── message             # Agent messaging (from penf)
│   ├── send
│   ├── inbox
│   ├── show
│   └── read
│
├── session             # Work sessions (from penf)
│   ├── start
│   ├── checkpoint
│   ├── show
│   └── end
│
├── context             # Project context (from penf)
│   ├── status
│   ├── history
│   ├── morning
│   └── project
│
├── task                # Task management (from palace)
│   ├── get
│   ├── claim
│   ├── progress
│   └── close
│
└── artifact            # Artifact tracking (from palace)
    └── add
```

### Global Flags

```
--project <name>        Override project from config
--agent <name>          Override agent identity
--output json|text|yaml Output format (default: text)
-o                      Short for --output
--limit <n>             Pagination limit
--debug                 Verbose logging
--config <path>         Override config file path
```

## Go Package Structure

```
cp/
├── main.go                     # Entry point
├── go.mod                      # Module: github.com/otherjamesbrown/context-palace/cp
├── cmd/
│   ├── root.go                 # Root command, config loading, DB connection
│   ├── status.go               # cp status
│   ├── init.go                 # cp init
│   ├── memory.go               # cp memory subcommands
│   ├── backlog.go              # cp backlog subcommands
│   ├── message.go              # cp message subcommands
│   ├── session.go              # cp session subcommands
│   ├── context.go              # cp context subcommands
│   ├── task.go                 # cp task subcommands (from palace)
│   └── artifact.go             # cp artifact subcommands (from palace)
└── internal/
    └── client/
        ├── client.go           # DB connection, config struct
        ├── shards.go           # Shard CRUD operations
        ├── messages.go         # Messaging operations
        ├── sessions.go         # Session operations
        └── format.go           # Output formatting (text/json/yaml)
```

### Shared Client Library

`internal/client/` is the core — all database operations live here. Commands in `cmd/`
are thin wrappers that parse arguments and call client functions.

This separation allows:
- `penf` to import the same client package (shared library, no duplication)
- Unit testing client functions without CLI overhead
- Future: other tools can use the client

## Success Criteria

1. **Install:** `go build -o cp ./cp` produces a standalone binary.
2. **Init:** `cp init` creates `.cp.yaml` with project name. Detects project from
   git remote or directory name if possible.
3. **Status:** `cp status` connects to DB and shows:
   ```
   Context Palace
     Host:     dev02.brown.chat
     Database: contextpalace
     Project:  penfold
     Agent:    agent-penfold
     Status:   connected
     Shards:   587 (41 open, 538 closed, 8 other)
   ```
4. **Config precedence:** Env var > .cp.yaml > ~/.cp/config.yaml > defaults.
5. **Agent identity:** All creates/updates use configured agent as creator/owner.
6. **Feature parity:** Migrated commands work identically to `penf` equivalents.
7. **No gateway dependency:** Works when Penfold services are down.
8. **JSON output:** Every command supports `-o json` with structured output.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| No `.cp.yaml` in project | Fall back to `~/.cp/config.yaml`. If no config at all: "Run `cp init` to configure." |
| No DB connection | "Cannot connect to Context Palace at dev02.brown.chat. Check config." Exit code 1. |
| Unknown project in DB | "Project 'foo' not found. Create it? (y/n)". Auto-create on yes. |
| Run outside any project dir | Uses global config. `--project` flag available. |
| SSL certificate missing | "SSL certificate not found at ~/.postgresql/. See setup.md." |
| Config file syntax error | "Error parsing ~/.cp/config.yaml: <yaml error>". |
| Both palace and cp installed | No conflict. Different binaries, different config paths. |

## Migration from `palace`

The existing `palace` binary has `task` and `artifact` commands. These are absorbed
into `cp` with identical behavior. `palace` can be deprecated after `cp` is stable.

| `palace` command | `cp` equivalent | Notes |
|------------------|----------------|-------|
| `palace task get <id>` | `cp task get <id>` | Identical |
| `palace task claim <id>` | `cp task claim <id>` | Identical |
| `palace task progress <id> <note>` | `cp task progress <id> <note>` | Identical |
| `palace task close <id> <summary>` | `cp task close <id> <summary>` | Identical |
| `palace artifact add <id> <type> <ref> <desc>` | `cp artifact add <id> <type> <ref> <desc>` | Identical |

Config migration: `~/.palace.yaml` → `~/.cp/config.yaml`. Different format but
same connection info. `cp init --migrate-palace` reads old config.

---

## Test Cases

### Unit Tests: Config Loading

```
TEST: LoadConfig with no files returns defaults
  Given: No ~/.cp/config.yaml, no .cp.yaml, no env vars
  When:  LoadConfig() is called
  Then:  Host = "dev02.brown.chat", Database = "contextpalace", Agent = ""
         Error: agent is required

TEST: LoadConfig reads global config
  Given: ~/.cp/config.yaml with host=testhost, agent=test-agent
  When:  LoadConfig() is called
  Then:  Host = "testhost", Agent = "test-agent"

TEST: LoadConfig project overrides global
  Given: ~/.cp/config.yaml with agent=global-agent
         .cp.yaml with agent=project-agent
  When:  LoadConfig() is called
  Then:  Agent = "project-agent"

TEST: LoadConfig env overrides all
  Given: ~/.cp/config.yaml with host=confighost
         CP_HOST=envhost
  When:  LoadConfig() is called
  Then:  Host = "envhost"

TEST: LoadConfig walks up to find .cp.yaml
  Given: .cp.yaml exists in parent directory
         CWD is a subdirectory
  When:  LoadConfig() is called
  Then:  Project config found and loaded

TEST: LoadConfig with invalid YAML
  Given: ~/.cp/config.yaml contains invalid YAML
  When:  LoadConfig() is called
  Then:  Returns descriptive error with file path and line number

TEST: LoadConfig migrate-palace
  Given: ~/.palace.yaml with host/database/user/project/agent
  When:  LoadConfig with --migrate-palace flag
  Then:  Reads palace config, maps to cp config format
```

### Unit Tests: Output Formatting

```
TEST: FormatText renders shard table
  Given: List of 3 shards with id, title, type, status
  When:  FormatText(shards) is called
  Then:  Returns aligned table with headers and rows

TEST: FormatJSON renders valid JSON
  Given: List of shards
  When:  FormatJSON(shards) is called
  Then:  Returns valid JSON array, parseable by jq

TEST: FormatJSON handles empty list
  Given: Empty shard list
  When:  FormatJSON(shards) is called
  Then:  Returns "[]", not null or error

TEST: FormatText handles long titles
  Given: Shard with 200-char title
  When:  FormatText(shard) is called
  Then:  Title truncated to column width with "..."
```

### Unit Tests: Connection String

```
TEST: ConnectionString with verify-full
  Given: Config with host=h, database=d, user=u, sslmode=verify-full
  When:  ConnectionString() is called
  Then:  Returns "host=h dbname=d user=u sslmode=verify-full"

TEST: ConnectionString with custom cert paths
  Given: Config with cert_path and key_path set
  When:  ConnectionString() is called
  Then:  Includes sslcert and sslkey parameters
```

### Integration Tests: Status Command

```
TEST: cp status shows connection info
  Given: Valid config pointing to test database
  When:  `cp status` is run
  Then:  Output includes host, database, project, agent, "connected"
         Exit code 0

TEST: cp status with bad connection
  Given: Config pointing to non-existent host
  When:  `cp status` is run
  Then:  Output includes "Cannot connect"
         Exit code 1

TEST: cp status shows shard counts
  Given: Test database with 5 open, 10 closed shards
  When:  `cp status` is run
  Then:  Output includes "15" total, "5 open", "10 closed"
```

### Integration Tests: Init Command

```
TEST: cp init creates .cp.yaml
  Given: Empty directory, no .cp.yaml
  When:  `cp init --project test-project --agent test-agent` is run
  Then:  .cp.yaml created with project=test-project, agent=test-agent

TEST: cp init detects git project name
  Given: Git repo with remote origin=github.com/org/my-project.git
  When:  `cp init` is run (no --project flag)
  Then:  .cp.yaml created with project=my-project

TEST: cp init refuses to overwrite
  Given: .cp.yaml already exists
  When:  `cp init` is run
  Then:  "Config already exists. Use --force to overwrite."
         Exit code 1
```

### Integration Tests: Migrated Commands

```
TEST: cp message send works
  Given: Valid config
  When:  `cp message send agent-test "Test Subject" --body "Test body"` is run
  Then:  Message shard created, ID returned

TEST: cp message inbox works
  Given: Message sent to current agent
  When:  `cp message inbox` is run
  Then:  Message appears in output with subject, sender, date

TEST: cp memory add works
  Given: Valid config
  When:  `cp memory add "Test memory"` is run
  Then:  Memory shard created, ID returned

TEST: cp memory list works
  Given: Memory shards exist
  When:  `cp memory list` is run
  Then:  Lists memory shards with ID, date, content preview

TEST: cp backlog add works
  Given: Valid config
  When:  `cp backlog add "Test item" --priority high` is run
  Then:  Backlog shard created with priority=1

TEST: cp task get works (palace parity)
  Given: Task shard exists with known ID
  When:  `cp task get <id>` is run
  Then:  Shows task details, matches palace output format
```
