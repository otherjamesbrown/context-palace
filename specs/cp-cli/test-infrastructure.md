# Test Infrastructure

## Current State

Zero test files in the codebase. No test framework, no test database, no CI.

## Test Strategy

### Layer 1: SQL Function Tests

Context Palace is SQL-first — most logic lives in PostgreSQL functions. Test these
directly with SQL test scripts that assert expected results.

**Framework:** Plain SQL scripts with assertions via `DO $$ ... $$` blocks and
`RAISE EXCEPTION` on failure. No external framework needed (pgTAP is overkill for
our scale).

**Test database:** `contextpalace_test` — separate database, same schema. Created
fresh per test run.

**Pattern:**

```sql
-- test_semantic_search.sql
BEGIN;

-- Setup
INSERT INTO shards (id, project, title, content, type, status, creator)
VALUES ('test-001', 'test-project', 'Test shard', 'Content about timeouts', 'memory', 'open', 'test-agent');

-- Test: semantic_search returns results (requires embedding)
-- ... (embedding populated by test harness)

-- Assert
DO $$
DECLARE
    result_count INT;
BEGIN
    SELECT count(*) INTO result_count FROM shards WHERE project = 'test-project';
    IF result_count != 1 THEN
        RAISE EXCEPTION 'Expected 1 shard, got %', result_count;
    END IF;
END $$;

-- Cleanup
ROLLBACK;
```

**Runner:** `scripts/test-sql.sh`

```bash
#!/bin/bash
# Run all SQL tests against test database
set -euo pipefail

DB_NAME="contextpalace_test"
DB_HOST="${PALACE_HOST:-dev02.brown.chat}"
DB_USER="${PALACE_USER:-penfold}"
CONN="host=$DB_HOST dbname=$DB_NAME user=$DB_USER sslmode=verify-full"

# Create test database (idempotent)
psql "host=$DB_HOST dbname=postgres user=$DB_USER sslmode=verify-full" \
    -c "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1 || \
    psql "host=$DB_HOST dbname=postgres user=$DB_USER sslmode=verify-full" \
    -c "CREATE DATABASE $DB_NAME"

# Apply schema
psql "$CONN" -f migrations/schema.sql

# Run test files
PASS=0
FAIL=0
for test_file in tests/sql/*.sql; do
    echo -n "  $(basename $test_file)... "
    if psql "$CONN" -f "$test_file" > /dev/null 2>&1; then
        echo "PASS"
        ((PASS++))
    else
        echo "FAIL"
        ((FAIL++))
    fi
done

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] || exit 1
```

### Layer 2: Go Unit Tests

Test the Go CLI code — config loading, output formatting, argument parsing.
Mock the database connection for unit tests.

**Framework:** Standard Go `testing` package.

**Pattern:**

```go
// cmd/config_test.go
package cmd

import (
    "os"
    "testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
    cfg, err := LoadConfig()
    if err != nil {
        t.Fatalf("LoadConfig failed: %v", err)
    }
    if cfg.Host != "dev02.brown.chat" {
        t.Errorf("expected default host, got %s", cfg.Host)
    }
    if cfg.Database != "contextpalace" {
        t.Errorf("expected default database, got %s", cfg.Database)
    }
}

func TestLoadConfig_EnvOverride(t *testing.T) {
    os.Setenv("CP_HOST", "localhost")
    defer os.Unsetenv("CP_HOST")

    cfg, err := LoadConfig()
    if err != nil {
        t.Fatalf("LoadConfig failed: %v", err)
    }
    if cfg.Host != "localhost" {
        t.Errorf("expected localhost, got %s", cfg.Host)
    }
}

func TestLoadConfig_ProjectFile(t *testing.T) {
    // Write temp .cp.yaml, verify it overrides global config
    // ...
}
```

**Run:** `go test ./cmd/... -v`

### Layer 3: Integration Tests

End-to-end tests that exercise the CLI against a real test database.
These test the full flow: CLI → Go code → PostgreSQL → result.

**Framework:** Go test with `os/exec` to run the `cp` binary, or test the command
functions directly with a real DB connection.

**Pattern:**

```go
// integration/requirement_test.go
//go:build integration

package integration

import (
    "os/exec"
    "strings"
    "testing"
)

func TestRequirementLifecycle(t *testing.T) {
    // Create
    out, err := exec.Command("cp", "requirement", "create",
        "Test Requirement", "--priority", "2", "--category", "test").CombinedOutput()
    if err != nil {
        t.Fatalf("create failed: %s\n%s", err, out)
    }
    id := extractID(string(out))

    // List
    out, _ = exec.Command("cp", "requirement", "list", "-o", "json").CombinedOutput()
    if !strings.Contains(string(out), id) {
        t.Errorf("requirement %s not in list output", id)
    }

    // Approve
    out, err = exec.Command("cp", "requirement", "approve", id).CombinedOutput()
    if err != nil {
        t.Errorf("approve failed: %s\n%s", err, out)
    }

    // Cleanup
    exec.Command("cp", "shard", "delete", id, "--force").Run()
}
```

**Run:** `go test -tags integration ./integration/... -v`

### Directory Structure

```
context-palace/
├── palace/                    # Existing (to be replaced by cp/)
├── cp/                        # New cp CLI
│   ├── main.go
│   ├── go.mod
│   ├── cmd/
│   │   ├── root.go
│   │   ├── root_test.go       # Config, connection tests
│   │   ├── recall.go
│   │   ├── recall_test.go
│   │   ├── requirement.go
│   │   ├── requirement_test.go
│   │   ├── knowledge.go
│   │   ├── knowledge_test.go
│   │   ├── shard.go
│   │   ├── shard_test.go
│   │   ├── memory.go
│   │   └── memory_test.go
│   ├── internal/
│   │   ├── client/            # Context Palace client library
│   │   │   ├── client.go
│   │   │   ├── client_test.go
│   │   │   ├── shards.go
│   │   │   ├── shards_test.go
│   │   │   ├── search.go
│   │   │   └── search_test.go
│   │   └── embedding/         # Embedding provider abstraction
│   │       ├── embed.go
│   │       ├── embed_test.go
│   │       ├── google.go
│   │       └── google_test.go
│   └── integration/
│       ├── requirement_test.go
│       ├── knowledge_test.go
│       ├── recall_test.go
│       └── shard_test.go
├── migrations/
│   ├── 001_schema.sql         # Existing base schema
│   ├── 002_metadata.sql       # SPEC-2: metadata column
│   └── 003_pgvector.sql       # SPEC-1: pgvector + embedding
├── tests/
│   └── sql/
│       ├── test_shard_crud.sql
│       ├── test_metadata.sql
│       ├── test_semantic_search.sql
│       ├── test_requirement_dashboard.sql
│       └── test_knowledge_versioning.sql
└── scripts/
    ├── test-sql.sh            # Run SQL tests
    ├── test-all.sh            # Run all tests
    └── migrate.sh             # Apply migrations
```

### Test Naming Convention

| Test file | What it tests |
|-----------|--------------|
| `*_test.go` (in cmd/) | Unit tests — no DB required |
| `*_test.go` (in internal/) | Unit tests — no DB required |
| `*_test.go` (in integration/) | Integration tests — requires test DB, build tag `integration` |
| `tests/sql/*.sql` | SQL function tests — run directly against test DB |

### Running Tests

```bash
# Unit tests only (no DB required)
cd cp && go test ./cmd/... ./internal/... -v

# SQL tests (requires test DB)
./scripts/test-sql.sh

# Integration tests (requires test DB + built binary)
cd cp && go build -o cp . && go test -tags integration ./integration/... -v

# All tests
./scripts/test-all.sh
```

### CI (future)

```yaml
# .github/workflows/test.yml
name: Tests
on: [push, pull_request]
jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: cd cp && go test ./cmd/... ./internal/... -v

  integration:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: pgvector/pgvector:pg16
        env:
          POSTGRES_DB: contextpalace_test
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
        ports: ['5432:5432']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: psql -f migrations/001_schema.sql
        env: { PGHOST: localhost, PGUSER: test, PGPASSWORD: test, PGDATABASE: contextpalace_test }
      - run: ./scripts/test-sql.sh
      - run: cd cp && go build -o cp . && go test -tags integration ./integration/... -v
```

### What to Test Per Spec

| Spec | SQL Tests | Go Unit Tests | Integration Tests |
|------|-----------|---------------|-------------------|
| SPEC-0 | — | Config loading, env overrides, output formatting | Connection, status command |
| SPEC-1 | semantic_search(), embed null handling | Embedding provider mock, truncation | Embed + recall round-trip |
| SPEC-2 | Metadata JSONB ops, GIN queries, merge | Metadata CLI parsing | Set + query round-trip |
| SPEC-3 | requirement_dashboard(), lifecycle transitions | — | Full lifecycle create→approve→verify |
| SPEC-4 | Version preservation, history query | — | Create → update → history → diff |
| SPEC-5 | — | Shard list filters, edge display | recall + shard ops end-to-end |
