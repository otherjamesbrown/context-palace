# SPEC-9: Test Infrastructure & Coverage

**Status:** Draft
**Depends on:** SPEC-0 (CLI skeleton, config)
**Blocks:** Nothing (but all future specs benefit from test infrastructure)

---

## Goal

Establish Go test infrastructure for the CP CLI and deliver unit test coverage across
all packages. Currently there are **zero `_test.go` files** — 49 source files, ~11k LOC,
0% coverage. The only test artifact is a SQL fixture (`tests/sql/test_create_shard.sql`)
with 7 PL/pgSQL tests for stored procedures.

This spec delivers: a test harness with mocks for external dependencies (PostgreSQL,
Google APIs), unit tests for all pure-logic packages, and integration test scaffolding
for the client layer. The goal is **≥80% line coverage on pure-logic packages** and
**≥60% on client packages** (excluding DB round-trips themselves).

## What Exists

- 49 Go source files across 7 packages (cp module) + 5 files (palace module)
- `go.mod` has no test dependencies (no testify, gomock, etc.)
- `tests/sql/test_create_shard.sql` — 7 PL/pgSQL tests for create_shard/create_impl_shard
- `embedding.Provider` interface (embed.go:10) — already mockable
- `generation.Generator` interface (generator.go:10) — already mockable
- `client.Client` struct with `Connect()` returning `*pgx.Conn` — not interface-backed, needs wrapper for mocking
- `format.go` — pure functions (FormatJSON, FormatYAML, Table, Truncate) — fully testable as-is
- `pointer/` — pure functions (ParseSubMemories, RenderWithBlock, AppendSubMemory, RemoveSubMemory, ReplaceSubMemories) — fully testable as-is
- `summary/` — pure functions (BuildSummaryPrompt, ParseSummaryResponse) — fully testable as-is
- `embedding/embed.go` — pure functions (BuildEmbeddingText, FormatDimensionError) — fully testable as-is

## What to Build

1. **Test infrastructure** — go test dependencies, mock implementations, test helpers
2. **Phase 1 tests: pure-logic packages** — `pointer/`, `summary/`, `embedding/` (pure functions), `client/format.go`
3. **Phase 2 tests: config & connection** — `client/client.go` (LoadConfig, ConnectionString, findProjectConfig)
4. **Phase 3 tests: client operations** — mock-backed tests for client methods in `shards.go`, `memory_hierarchy.go`, `messages.go`, etc.
5. **Phase 4 tests: SQL functions** — extend existing SQL fixture to cover more stored procedures

## Data Model

### Schema Changes

No schema changes.

### Storage Format

N/A — no new data stored.

### Data Flow

N/A — tests consume existing data flows, they don't create new ones.

### Concurrency

N/A — tests run sequentially via `go test`. No shared mutable state between tests.

## Test Infrastructure

### Dependencies to Add

```
go get github.com/stretchr/testify
```

Add to `go.mod`:
```
github.com/stretchr/testify v1.9.0
```

Testify provides `assert` and `require` — used universally in Go projects, reduces
boilerplate without introducing heavy frameworks. No code generation tools (gomock,
mockery) — hand-written mocks are sufficient given the small interface surface.

### Mock Implementations

#### `internal/embedding/mock_provider.go`

```go
package embedding

import "context"

// MockProvider implements Provider for testing.
type MockProvider struct {
    EmbedFunc      func(ctx context.Context, text string) ([]float32, error)
    DimensionsFunc func() int
    EmbedCalls     []string // records text arguments
}

func (m *MockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
    m.EmbedCalls = append(m.EmbedCalls, text)
    if m.EmbedFunc != nil {
        return m.EmbedFunc(ctx, text)
    }
    return make([]float32, m.Dimensions()), nil
}

func (m *MockProvider) Dimensions() int {
    if m.DimensionsFunc != nil {
        return m.DimensionsFunc()
    }
    return 768
}
```

#### `internal/generation/mock_generator.go`

```go
package generation

import "context"

// MockGenerator implements Generator for testing.
type MockGenerator struct {
    GenerateFunc func(ctx context.Context, prompt string) (string, error)
    GenerateCalls []string // records prompt arguments
}

func (m *MockGenerator) Generate(ctx context.Context, prompt string) (string, error) {
    m.GenerateCalls = append(m.GenerateCalls, prompt)
    if m.GenerateFunc != nil {
        return m.GenerateFunc(ctx, prompt)
    }
    return "{}", nil
}
```

#### Database mock strategy

The `client.Client` struct calls `pgx.Connect()` directly — it doesn't use an interface.
Rather than refactoring the connection layer (out of scope), tests for client methods that
hit the DB will use **build-tag-gated integration tests** that require a real database:

```go
//go:build integration

package client_test
```

Pure-logic helpers extracted from client methods (formatting, validation, query building)
should be tested without DB access. Where client methods mix logic and DB calls, extract
the logic into testable helpers during test writing.

### Test Helpers

#### `internal/testutil/testutil.go`

```go
package testutil

import (
    "os"
    "path/filepath"
    "testing"
)

// TempConfigDir creates a temp directory with a config file for testing LoadConfig.
// Returns cleanup function.
func TempConfigDir(t *testing.T, configYAML string) (dir string) {
    t.Helper()
    dir = t.TempDir()
    path := filepath.Join(dir, "config.yaml")
    if err := os.WriteFile(path, []byte(configYAML), 0644); err != nil {
        t.Fatal(err)
    }
    return dir
}

// TempProjectDir creates a temp directory with .cp.yaml for testing findProjectConfig.
func TempProjectDir(t *testing.T, projectYAML string) (dir string) {
    t.Helper()
    dir = t.TempDir()
    path := filepath.Join(dir, ".cp.yaml")
    if err := os.WriteFile(path, []byte(projectYAML), 0644); err != nil {
        t.Fatal(err)
    }
    return dir
}
```

### File Layout

```
cp/
├── internal/
│   ├── client/
│   │   ├── client_test.go          # Phase 2: config, connection string
│   │   └── format_test.go          # Phase 1: table, truncate, format
│   ├── embedding/
│   │   ├── embed_test.go           # Phase 1: BuildEmbeddingText, FormatDimensionError
│   │   ├── config_test.go          # Phase 2: NewProvider factory
│   │   └── mock_provider.go        # Mock implementation
│   ├── generation/
│   │   ├── generator_test.go       # Phase 2: NewGenerator factory
│   │   └── mock_generator.go       # Mock implementation
│   ├── pointer/
│   │   └── pointer_test.go         # Phase 1: parse, render, append, remove, replace
│   ├── summary/
│   │   └── summary_test.go         # Phase 1: BuildSummaryPrompt, ParseSummaryResponse
│   └── testutil/
│       └── testutil.go             # Shared test helpers
└── tests/
    └── sql/
        ├── test_create_shard.sql   # Existing (7 tests)
        └── test_memory_ops.sql     # Phase 4: memory SQL functions
```

## CLI Surface

N/A — no new commands. Tests are run via `go test ./...`.

### Running Tests

```bash
# Unit tests only (default)
cd cp && go test ./...

# With coverage report
cd cp && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

# Integration tests (requires DB)
cd cp && go test -tags=integration ./...

# Specific package
cd cp && go test ./internal/pointer/...
```

## Workflows

N/A — no multi-step user-facing workflows.

## SQL Functions

No new SQL functions. Phase 4 extends the existing test fixture.

## Go Implementation Notes

### Phase 1: Pure-Logic Tests

These packages have zero external dependencies — test them first.

#### `pointer/pointer_test.go`

```go
package pointer_test

import (
    "testing"

    "github.com/otherjamesbrown/context-palace/cp/internal/pointer"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**Test cases (all in this file):**

```
TEST: ParseSubMemories_NoBlock
  Given: content with no sub-memories markers
  When:  ParseSubMemories(content)
  Then:  returns original content, nil entries, nil error

TEST: ParseSubMemories_ValidBlock
  Given: content with valid JSON between markers
  When:  ParseSubMemories(content)
  Then:  returns main content (trimmed), parsed entries, nil error

TEST: ParseSubMemories_UnclosedBlock
  Given: content with start marker but no end marker
  When:  ParseSubMemories(content)
  Then:  returns original content, nil entries, error containing "without closing"

TEST: ParseSubMemories_InvalidJSON
  Given: content with markers wrapping malformed JSON
  When:  ParseSubMemories(content)
  Then:  returns main content, nil entries, error containing "invalid sub-memories JSON"

TEST: ParseSubMemories_EmptyJSON
  Given: content with markers wrapping "[]"
  When:  ParseSubMemories(content)
  Then:  returns main content, empty slice, nil error

TEST: RenderWithBlock_SingleEntry
  Given: main content string, one SubMemoryEntry
  When:  RenderWithBlock(main, entries)
  Then:  err == nil, output contains main content, start marker, valid JSON with entry, end marker

TEST: RenderWithBlock_MultipleEntries
  Given: main content, three SubMemoryEntries
  When:  RenderWithBlock(main, entries)
  Then:  err == nil, JSON block contains all three entries in order

TEST: RenderWithBlock_Roundtrip
  Given: entries rendered via RenderWithBlock
  When:  result parsed via ParseSubMemories
  Then:  err == nil on both calls, parsed entries match original

TEST: AppendSubMemory_ToEmpty
  Given: content with no existing block
  When:  AppendSubMemory(content, newEntry)
  Then:  err == nil, output has block with exactly one entry

TEST: AppendSubMemory_ToExisting
  Given: content with block containing 2 entries
  When:  AppendSubMemory(content, newEntry)
  Then:  err == nil, output has block with 3 entries, new entry last

TEST: AppendSubMemory_BrokenBlock
  Given: content with unclosed sub-memories marker (start but no end)
  When:  AppendSubMemory(content, newEntry)
  Then:  returns ("", err) — propagates parse error (unlike ReplaceSubMemories which recovers)

TEST: RemoveSubMemory_Exists
  Given: content with block containing entries [A, B, C]
  When:  RemoveSubMemory(content, "B")
  Then:  output has block with entries [A, C]

TEST: RemoveSubMemory_BrokenBlock
  Given: content with unclosed sub-memories marker
  When:  RemoveSubMemory(content, "A")
  Then:  returns ("", err) — propagates parse error (unlike ReplaceSubMemories which recovers)

TEST: RemoveSubMemory_LastEntry
  Given: content with block containing one entry
  When:  RemoveSubMemory(content, entry.ID)
  Then:  output is just mainContent (no block)

TEST: RemoveSubMemory_NotFound
  Given: content with block containing entries [A, B]
  When:  RemoveSubMemory(content, "Z")
  Then:  output unchanged (all entries preserved)

TEST: ReplaceSubMemories_WithEntries
  Given: content with existing block
  When:  ReplaceSubMemories(content, newEntries)
  Then:  old entries gone, new entries present

TEST: ReplaceSubMemories_EmptyEntries
  Given: content with existing block
  When:  ReplaceSubMemories(content, nil)
  Then:  output is just mainContent (block removed)

TEST: ReplaceSubMemories_BrokenBlock
  Given: content with malformed block (unclosed marker)
  When:  ReplaceSubMemories(content, newEntries)
  Then:  uses raw content (including dangling marker) as mainContent, appends new block
  Note:  This produces content with a dangling <!-- sub-memories --> in the prose.
         AppendSubMemory and RemoveSubMemory do NOT have this recovery — they fail.
         This asymmetry is intentional: Replace is a reconciliation tool.
```

#### `client/format_test.go`

```
TEST: Truncate_ShortString
  Given: "hello" with maxLen 10
  When:  Truncate(s, 10)
  Then:  returns "hello" unchanged

TEST: Truncate_ExactLength
  Given: "hello" with maxLen 5
  When:  Truncate(s, 5)
  Then:  returns "hello" unchanged

TEST: Truncate_LongString
  Given: "hello world" with maxLen 8
  When:  Truncate(s, 8)
  Then:  returns "hello..."

TEST: Truncate_VerySmallMax
  Given: "hello" with maxLen 2
  When:  Truncate(s, 2)
  Then:  returns "he" (no room for "...")

TEST: Truncate_MaxThree
  Given: "hello" with maxLen 3
  When:  Truncate(s, 3)
  Then:  returns "hel" (edge: maxLen <= 3 returns raw truncation)

TEST: NewTable_EmptyRows
  Given: table with headers ["ID", "NAME"]
  When:  String()
  Then:  returns "" (no rows = no output)

TEST: NewTable_SingleRow
  Given: table with headers ["ID", "NAME"], one row ["1", "Alice"]
  When:  String()
  Then:  output has header line and one data line, columns aligned

TEST: NewTable_ColumnAlignment
  Given: table with varying cell widths
  When:  String()
  Then:  columns padded to max width, last column not padded

TEST: NewTable_FewerCellsThanHeaders
  Given: AddRow with fewer cells than headers
  When:  String()
  Then:  missing cells treated as empty string, no panic

TEST: NewTable_ZeroHeaders
  Given: NewTable() with no arguments
  When:  String()
  Then:  returns "" (no panic)

TEST: Truncate_ZeroMax
  Given: s="hello", maxLen=0
  When:  Truncate(s, 0)
  Then:  returns ""

TEST: Truncate_EmptyString
  Given: s="", maxLen=5
  When:  Truncate(s, 5)
  Then:  returns ""

TEST: FormatJSON_SimpleStruct
  Given: struct{Name string}{Name: "test"}
  When:  FormatJSON(data)
  Then:  returns indented JSON with "name": "test"

TEST: FormatYAML_SimpleStruct
  Given: map[string]string{"key": "val"}
  When:  FormatYAML(data)
  Then:  returns valid YAML "key: val\n"

TEST: FormatOutput_JSON
  Given: data, format="json"
  When:  FormatOutput(data, "json")
  Then:  returns JSON string

TEST: FormatOutput_YAML
  Given: data, format="yaml"
  When:  FormatOutput(data, "yaml")
  Then:  returns YAML string matching FormatYAML output

TEST: FormatOutput_Unknown
  Given: data, format="text"
  When:  FormatOutput(data, "text")
  Then:  returns fmt.Sprintf("%v", data)
```

#### `embedding/embed_test.go`

```
TEST: BuildEmbeddingText_AllFields
  Given: type="note", title="My Title", content="Body text"
  When:  BuildEmbeddingText(type, title, content)
  Then:  returns "note: My Title\n\nBody text"

TEST: BuildEmbeddingText_NoType
  Given: type="", title="My Title", content="Body text"
  When:  BuildEmbeddingText(type, title, content)
  Then:  returns "My Title\n\nBody text"

TEST: BuildEmbeddingText_NoContent
  Given: type="note", title="My Title", content=""
  When:  BuildEmbeddingText(type, title, content)
  Then:  returns "note: My Title" (no trailing newlines)

TEST: BuildEmbeddingText_WhitespaceContent
  Given: type="note", title="My Title", content="  \n  "
  When:  BuildEmbeddingText(type, title, content)
  Then:  returns "note: My Title" (whitespace-only content treated as empty)

TEST: BuildEmbeddingText_Truncation
  Given: content longer than 32000 chars
  When:  BuildEmbeddingText("", "T", longContent)
  Then:  result length is exactly 32000

TEST: BuildEmbeddingText_TypeOnly
  Given: type="note", title="", content=""
  When:  BuildEmbeddingText("note", "", "")
  Then:  returns "note: " (trailing space after colon — documents current behavior)

TEST: BuildEmbeddingText_Empty
  Given: all empty strings
  When:  BuildEmbeddingText("", "", "")
  Then:  returns ""

TEST: FormatDimensionError_Message
  Given: expected=768, got=384
  When:  FormatDimensionError(768, 384)
  Then:  error message contains "expected 768" and "returns 384"
```

#### `summary/summary_test.go`

```
TEST: ParseSummaryResponse_ValidJSON
  Given: '{"summary":"trigger text","parent_needs_update":false,"parent_edits":null}'
  When:  ParseSummaryResponse(response)
  Then:  returns SummaryResult with correct fields

TEST: ParseSummaryResponse_WithCodeFence
  Given: response wrapped in ```json ... ```
  When:  ParseSummaryResponse(response)
  Then:  strips fences, parses JSON correctly

TEST: ParseSummaryResponse_WithPlainFence
  Given: response wrapped in ``` ... ``` (no language tag)
  When:  ParseSummaryResponse(response)
  Then:  strips fences, parses JSON correctly

TEST: ParseSummaryResponse_EmptySummary
  Given: '{"summary":"","parent_needs_update":false}'
  When:  ParseSummaryResponse(response)
  Then:  returns error containing "AI returned empty summary"

TEST: ParseSummaryResponse_InvalidJSON
  Given: "not json at all"
  When:  ParseSummaryResponse(response)
  Then:  returns error containing "failed to parse AI response as JSON"

TEST: ParseSummaryResponse_WithParentEdits
  Given: JSON with parent_needs_update:true and parent_edits string
  When:  ParseSummaryResponse(response)
  Then:  ParentNeedsUpdate is true, ParentEdits is non-nil with correct value

TEST: ParseSummaryResponse_NestedFences
  Given: "```json\n{\"summary\":\"has ``` inside\",\"parent_needs_update\":false}\n```"
  When:  ParseSummaryResponse(input)
  Then:  strips outer fences, inner ``` preserved, parses correctly

TEST: ParseSummaryResponse_MinimalFence
  Given: "```json\n```" (only 2 lines, no content between fences)
  When:  ParseSummaryResponse(input)
  Then:  returns error (empty/non-JSON content after stripping)

TEST: BuildSummaryPrompt_StripsSubMemories
  Given: parentContent with a sub-memories block
  When:  BuildSummaryPrompt(parentID, parentContent, childTitle, childContent)
  Then:  prompt contains main parent content but NOT "<!-- sub-memories -->" markers

TEST: BuildSummaryPrompt_ContainsAllFields
  Given: parentID="abc", childTitle="My Child"
  When:  BuildSummaryPrompt(parentID, parentContent, childTitle, childContent)
  Then:  prompt contains "PARENT MEMORY (ID: abc)", "NEW CHILD MEMORY (title: My Child)",
         parent content, child content, and "---" delimiters
```

### Phase 2: Config & Factory Tests

#### `client/client_test.go`

```
TEST: ConnectionString_Defaults
  Given: Config with host, database, user, no sslmode
  When:  ConnectionString()
  Then:  returns string with sslmode=verify-full (default)

TEST: ConnectionString_CustomSSL
  Given: Config with sslmode="disable"
  When:  ConnectionString()
  Then:  returns string with sslmode=disable

TEST: LoadConfig_EnvOverrides
  Given: config file with host=filehost, env CP_HOST=envhost
  When:  LoadConfig(configPath)
  Then:  config.Connection.Host == "envhost"

TEST: LoadConfig_MissingUser
  Given: config with no user set anywhere
  When:  LoadConfig(configPath)
  Then:  returns error containing "database user is required"

TEST: LoadConfig_MissingAgent
  Given: config with user but no agent
  When:  LoadConfig(configPath)
  Then:  returns error containing "agent identity is required"

TEST: LoadConfig_ProjectOverrides
  Given: global config with agent=A, .cp.yaml with agent=B
  When:  LoadConfig (from dir with .cp.yaml)
  Then:  config.Agent == "B"

TEST: LoadConfig_AllEnvVars
  Given: CP_HOST, CP_DATABASE, CP_USER, CP_PROJECT, CP_AGENT all set
  When:  LoadConfig("")
  Then:  all config fields match env var values
```

#### `embedding/config_test.go`

```
TEST: NewProvider_NilConfig
  Given: cfg = nil
  When:  NewProvider(nil)
  Then:  returns (nil, nil)

TEST: NewProvider_UnsupportedProvider
  Given: cfg with Provider="openai"
  When:  NewProvider(cfg)
  Then:  returns error containing "unsupported"

TEST: NewProvider_MissingAPIKey
  Given: cfg with Provider="google", APIKeyEnv="NONEXISTENT_VAR"
  When:  NewProvider(cfg)
  Then:  returns error containing "API key not found"

TEST: NewProvider_GoogleValid
  Given: cfg with Provider="google", t.Setenv(cfg.APIKeyEnv, "dummy-key")
  When:  NewProvider(cfg)
  Then:  returns non-nil provider, nil error (no network call during construction)
```

#### `generation/generator_test.go`

```
TEST: NewGenerator_NilConfig
  Given: cfg = nil
  When:  NewGenerator(nil)
  Then:  returns (nil, nil)

TEST: NewGenerator_UnsupportedProvider
  Given: cfg with Provider="openai"
  When:  NewGenerator(cfg)
  Then:  returns error containing "unsupported"

TEST: NewGenerator_MissingAPIKey
  Given: cfg with Provider="google", APIKeyEnv="NONEXISTENT_VAR"
  When:  NewGenerator(cfg)
  Then:  returns error containing "API key not found"

TEST: NewGenerator_GoogleValid
  Given: cfg with Provider="google", t.Setenv(cfg.APIKeyEnv, "dummy-key")
  When:  NewGenerator(cfg)
  Then:  returns non-nil generator, nil error (no network call during construction)
```

**Environment variable safety:** All tests that call `os.Setenv` must use `t.Setenv()`
(Go 1.17+) which automatically restores the original value on test cleanup. For
`LoadConfig_ProjectOverrides`, use the `configOverride` parameter to bypass
`findProjectConfig()` — do not `os.Chdir` in tests as it affects the whole process.

### Phase 3: Client Operation Tests

These test the logic in client methods **without hitting a real database**. The strategy
is to extract testable logic (query building, result formatting, validation) where possible,
and use build-tagged integration tests for DB round-trips.

Focus on the highest-value files first:

| File | LOC | Test Priority | Strategy |
|------|-----|---------------|----------|
| memory_hierarchy.go | 894 | HIGH | Extract tree-building logic, mock DB rows |
| shards.go | 582 | HIGH | Extract query builders, validate formatting |
| requirement.go | 507 | MEDIUM | Extract status transitions, validate output |
| knowledge.go | 368 | MEDIUM | Extract diff logic, validate rendering |
| edges.go | 267 | MEDIUM | Validate edge type parsing, formatting |
| metadata.go | 234 | MEDIUM | Validate JSONB marshaling/unmarshaling |
| lifecycle.go | 211 | LOW | Mostly DB orchestration |
| search.go | 201 | LOW | Mostly DB orchestration |
| epic.go | 166 | LOW | Mostly DB orchestration |
| sessions.go | 115 | LOW | Mostly DB orchestration |
| messages.go | 114 | LOW | Mostly DB orchestration |
| focus.go | 77 | LOW | Mostly DB orchestration |
| labels.go | 72 | LOW | Mostly DB orchestration |
| memory_telemetry.go | 21 | LOW | Trivial, defer |

For each file, the implementer should:

1. Read the source and identify pure-logic sections (struct building, formatting, validation, state transitions)
2. If logic is inline in a method that also does DB calls, extract it to a testable helper
3. Write tests for the extracted helpers
4. Mark DB-dependent tests with `//go:build integration`

**Do NOT refactor Client to use a DB interface.** That's a larger change. This spec is about
getting coverage on what's testable today with minimal structural changes.

### Phase 4: SQL Function Tests

Extend `tests/sql/` with fixtures for additional stored procedures. Check which SQL
functions exist in the database and add test fixtures for any that lack coverage.

```bash
# Find all custom functions
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" \
  -c "SELECT routine_name FROM information_schema.routines WHERE routine_schema = 'public' ORDER BY routine_name;"
```

Write tests in the same pattern as `test_create_shard.sql`:
- Each test in a `DO $$ ... END $$;` block
- Wrapped in `BEGIN; ... ROLLBACK;`
- Uses `RAISE EXCEPTION` for failures, `RAISE NOTICE` for passes

## Success Criteria

1. `go test ./...` runs from `cp/` directory and passes with zero failures
2. `testify` is the only new dependency added
3. `pointer/` package has ≥90% line coverage (pure logic, no excuses)
4. `summary/` package has ≥90% line coverage (pure logic)
5. `embedding/embed.go` functions have ≥90% line coverage
6. `client/format.go` has ≥90% line coverage
7. `client/client.go` LoadConfig + ConnectionString have ≥80% line coverage
8. `embedding/config.go` and `generation/generator.go` factory functions have ≥80% line coverage
9. Mock implementations exist for `embedding.Provider` and `generation.Generator`
10. All tests run without network access or database connection (except `//go:build integration` tagged tests)
11. `go test -coverprofile=coverage.out ./...` produces a valid coverage report
12. Phase 3 client tests deliver ≥5 extracted helper functions with tests from the top-3 files (memory_hierarchy, shards, requirement)
13. No existing code behavior changes — tests are additive only
14. SQL test fixtures cover ≥3 additional stored procedures beyond create_shard/create_impl_shard

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| ParseSubMemories with multiple blocks in same content | Only first block parsed (matches current implementation) |
| ParseSummaryResponse with nested code fences | Outer fences stripped, inner content preserved |
| BuildEmbeddingText with exactly 32000 chars | Returns unchanged (boundary: `len(text) > maxChars`) |
| BuildEmbeddingText with 32001 chars | Truncated to exactly 32000 |
| Truncate with maxLen=0 | Returns "" (boundary) |
| Truncate with empty string | Returns "" |
| Table with zero headers | No panic, returns "" |
| LoadConfig with no home directory | Falls through gracefully (err from UserHomeDir) |
| LoadConfig with unreadable config file | Silently skipped (loadYAML returns on error) |
| RemoveSubMemory from empty entries | Returns mainContent (no block) |
| ReplaceSubMemories with broken existing block | Uses raw content (including dangling marker) as main, appends new block. Note: AppendSubMemory/RemoveSubMemory fail instead — no recovery. |
| AppendSubMemory with broken existing block | Returns error (no recovery, unlike ReplaceSubMemories) |
| ParseSummaryResponse with only fence lines | Strips fences, tries to parse empty string, returns error |
| BuildEmbeddingText type-only (no title/content) | Returns "note: " — trailing space, not empty |
| NewProvider with empty APIKeyEnv string | Reads os.Getenv("") which returns "", triggers error |
| ConnectionString with empty host/database | Returns malformed string (matches current behavior — no validation) |

---

## Test Cases

### Go Unit Tests — Phase 1

```
TEST: ParseSubMemories_NoBlock
  Given: "Hello world\nSome content"
  When:  ParseSubMemories(input)
  Then:  mainContent == input, entries == nil, err == nil

TEST: ParseSubMemories_ValidBlock
  Given: "Main\n\n<!-- sub-memories -->\n[{\"id\":\"a\",\"title\":\"T\",\"summary\":\"S\"}]\n<!-- /sub-memories -->\n"
  When:  ParseSubMemories(input)
  Then:  mainContent == "Main", len(entries) == 1, entries[0].ID == "a"

TEST: ParseSubMemories_UnclosedBlock
  Given: "Main\n<!-- sub-memories -->\n[...]"
  When:  ParseSubMemories(input)
  Then:  err != nil, err contains "without closing"

TEST: ParseSubMemories_InvalidJSON
  Given: "Main\n<!-- sub-memories -->\nnot json\n<!-- /sub-memories -->"
  When:  ParseSubMemories(input)
  Then:  err != nil, err contains "invalid sub-memories JSON"

TEST: RenderWithBlock_Roundtrip
  Given: entries = [{ID:"x", Title:"T", Summary:"S"}]
  When:  rendered := RenderWithBlock("Main", entries); ParseSubMemories(rendered)
  Then:  err == nil on both calls, parsed entries match original

TEST: AppendSubMemory_ToEmpty
  Given: content = "Hello"
  When:  AppendSubMemory(content, {ID:"a", Title:"T", Summary:"S"})
  Then:  err == nil, result contains markers, ParseSubMemories returns 1 entry

TEST: AppendSubMemory_BrokenBlock
  Given: content with unclosed sub-memories marker
  When:  AppendSubMemory(content, entry)
  Then:  returns ("", err) — fails (no error recovery)

TEST: RemoveSubMemory_BrokenBlock
  Given: content with unclosed sub-memories marker
  When:  RemoveSubMemory(content, "A")
  Then:  returns ("", err) — fails (no error recovery)

TEST: RemoveSubMemory_LastEntry
  Given: content with 1 entry (ID="a")
  When:  RemoveSubMemory(content, "a")
  Then:  result has no markers, just main content

TEST: RemoveSubMemory_NotFound
  Given: content with entries [A, B]
  When:  RemoveSubMemory(content, "Z")
  Then:  result still has 2 entries

TEST: ReplaceSubMemories_EmptyEntries
  Given: content with existing block
  When:  ReplaceSubMemories(content, nil)
  Then:  result is just mainContent, no markers

TEST: ReplaceSubMemories_BrokenBlock
  Given: content with unclosed marker
  When:  ReplaceSubMemories(content, newEntries)
  Then:  uses raw content (including dangling marker) as main, appends new valid block

TEST: Truncate_ZeroMax
  Given: s="hello", maxLen=0
  When:  Truncate(s, 0)
  Then:  returns ""

TEST: Truncate_EmptyString
  Given: s="", maxLen=5
  When:  Truncate(s, 5)
  Then:  returns ""

TEST: Truncate_ShortString
  Given: s="hello", maxLen=10
  When:  Truncate(s, 10)
  Then:  returns "hello"

TEST: Truncate_LongString
  Given: s="hello world", maxLen=8
  When:  Truncate(s, 8)
  Then:  returns "hello..."

TEST: Truncate_MaxThree
  Given: s="hello", maxLen=3
  When:  Truncate(s, 3)
  Then:  returns "hel"

TEST: Table_SingleRow
  Given: headers=["ID","NAME"], row=["1","Alice"]
  When:  table.String()
  Then:  contains "ID" and "Alice", columns aligned with spacing

TEST: Table_EmptyRows
  Given: headers=["ID","NAME"], no rows
  When:  table.String()
  Then:  returns ""

TEST: Table_FewerCellsThanHeaders
  Given: headers=["ID","NAME","STATUS"], row=["1","Alice"]
  When:  table.String()
  Then:  third column is empty, no panic

TEST: Table_ZeroHeaders
  Given: NewTable() with no arguments
  When:  table.String()
  Then:  returns ""

TEST: FormatOutput_YAML
  Given: data, format="yaml"
  When:  FormatOutput(data, "yaml")
  Then:  returns YAML string

TEST: BuildEmbeddingText_AllFields
  Given: type="note", title="T", content="C"
  When:  BuildEmbeddingText("note", "T", "C")
  Then:  returns "note: T\n\nC"

TEST: BuildEmbeddingText_Truncation
  Given: content of 40000 chars
  When:  BuildEmbeddingText("", "T", content)
  Then:  len(result) == 32000

TEST: BuildEmbeddingText_TypeOnly
  Given: type="note", title="", content=""
  When:  BuildEmbeddingText("note", "", "")
  Then:  returns "note: "

TEST: BuildEmbeddingText_Empty
  Given: all empty
  When:  BuildEmbeddingText("", "", "")
  Then:  returns ""

TEST: ParseSummaryResponse_ValidJSON
  Given: valid JSON string
  When:  ParseSummaryResponse(json)
  Then:  returns parsed SummaryResult

TEST: ParseSummaryResponse_WithCodeFence
  Given: "```json\n{...}\n```"
  When:  ParseSummaryResponse(input)
  Then:  strips fences, parses correctly

TEST: ParseSummaryResponse_EmptySummary
  Given: '{"summary":""}'
  When:  ParseSummaryResponse(input)
  Then:  error containing "AI returned empty summary"

TEST: ParseSummaryResponse_InvalidJSON
  Given: "not json"
  When:  ParseSummaryResponse(input)
  Then:  error containing "failed to parse AI response as JSON"

TEST: ParseSummaryResponse_NestedFences
  Given: "```json\n{\"summary\":\"has ``` inside\",...}\n```"
  When:  ParseSummaryResponse(input)
  Then:  outer fences stripped, parses correctly

TEST: ParseSummaryResponse_MinimalFence
  Given: "```json\n```"
  When:  ParseSummaryResponse(input)
  Then:  error (no valid JSON between fences)

TEST: BuildSummaryPrompt_StripsSubMemories
  Given: parentContent with sub-memories block
  When:  BuildSummaryPrompt(...)
  Then:  result does NOT contain "<!-- sub-memories -->"
```

### Go Unit Tests — Phase 2

```
TEST: ConnectionString_Defaults
  Given: Config{Host:"h", Database:"d", User:"u", SSLMode:""}
  When:  client.ConnectionString()
  Then:  returns "host=h dbname=d user=u sslmode=verify-full"

TEST: ConnectionString_CustomSSL
  Given: Config with SSLMode:"disable"
  When:  client.ConnectionString()
  Then:  contains "sslmode=disable"

TEST: LoadConfig_EnvOverrides
  Given: config file + CP_HOST env var
  When:  LoadConfig(path)
  Then:  Host matches env var, not file

TEST: LoadConfig_MissingUser
  Given: config with no user
  When:  LoadConfig(path)
  Then:  error "database user is required"

TEST: LoadConfig_MissingAgent
  Given: config with user but no agent
  When:  LoadConfig(path)
  Then:  error "agent identity is required"

TEST: NewProvider_NilConfig
  Given: nil
  When:  NewProvider(nil)
  Then:  (nil, nil)

TEST: NewProvider_UnsupportedProvider
  Given: cfg.Provider = "openai"
  When:  NewProvider(cfg)
  Then:  error "unsupported"

TEST: NewProvider_GoogleValid
  Given: cfg.Provider = "google", t.Setenv(APIKeyEnv, "dummy")
  When:  NewProvider(cfg)
  Then:  non-nil provider, nil error

TEST: NewGenerator_NilConfig
  Given: nil
  When:  NewGenerator(nil)
  Then:  (nil, nil)

TEST: NewGenerator_GoogleValid
  Given: cfg.Provider = "google", t.Setenv(APIKeyEnv, "dummy")
  When:  NewGenerator(cfg)
  Then:  non-nil generator, nil error
```

### SQL Tests — Phase 4

```
TEST: memory_hot — returns memories sorted by recency
  Given: 3 memories with different touched_at
  When:  SELECT * FROM memory_hot('project', 10)
  Then:  results ordered by touched_at DESC

TEST: memory_search — vector similarity returns relevant results
  Given: memories with embeddings
  When:  SELECT * FROM memory_search('project', embedding, 5)
  Then:  returns ≤5 results with similarity scores

TEST: shard_close — marks shard as closed
  Given: open shard
  When:  SELECT shard_close(id)
  Then:  status = 'closed', closed_at is set

TEST: create_shard — duplicate labels deduplicated
  Given: labels = ['a', 'b', 'a']
  When:  create_shard with duplicate labels
  Then:  stored labels have no duplicates (verify actual behavior)
```

---

## Pre-Submission Checklist

- [x] Every item in "What to Build" has: CLI section + SQL + success criterion + tests
- [x] Every data flow answers all 7 questions — N/A (no new data flows)
- [x] Every command has: syntax + example + output + atomic steps + JSON schema — N/A (no new commands)
- [x] Every workflow has: flowchart + all branches + error recovery — N/A (no workflows)
- [x] Every success criterion has at least one test case
- [x] Concurrency is addressed — N/A (tests run sequentially)
- [x] No feature is "mentioned but not specced"
- [x] Edge cases cover: invalid input, empty state, conflicts, boundaries, cross-feature, failure recovery
- [x] Existing spec interactions documented — no behavior changes, tests are additive only
- [x] Sub-agent review completed — 20 findings (3 HIGH, 10 MEDIUM, 7 LOW), all HIGH + key MEDIUM fixed
