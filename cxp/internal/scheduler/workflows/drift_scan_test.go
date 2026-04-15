package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/otherjamesbrown/context-palace/cxp/internal/generation"
)

// makeTempGitRepo creates a minimal git repo for verifier tests.
func makeTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	goContent := "package main\n\nfunc MyFunction() {}\n\ntype MyStruct struct{}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	return dir
}

// --- extractAnchors tests ---

func TestExtractAnchors_FilePaths(t *testing.T) {
	content := "See `internal/scheduler/runner.go` and `migrations/001_init.sql` for details."
	anchors := extractAnchors(content)

	found := map[string]bool{}
	for _, a := range anchors {
		if a.Type == "file_path" {
			found[a.Value] = true
		}
	}
	if !found["internal/scheduler/runner.go"] {
		t.Error("expected internal/scheduler/runner.go to be found")
	}
	if !found["migrations/001_init.sql"] {
		t.Error("expected migrations/001_init.sql to be found")
	}
}

func TestExtractAnchors_FunctionNames(t *testing.T) {
	content := "Call `ExtractAnchors()` or `VerifyAnchor()` to process content."
	anchors := extractAnchors(content)

	found := map[string]bool{}
	for _, a := range anchors {
		if a.Type == "function_name" {
			found[a.Value] = true
		}
	}
	if !found["ExtractAnchors"] {
		t.Error("expected ExtractAnchors to be found")
	}
	if !found["VerifyAnchor"] {
		t.Error("expected VerifyAnchor to be found")
	}
}

func TestExtractAnchors_Deduplication(t *testing.T) {
	content := "Use `config.yaml` for setup. See `config.yaml` again. Also `RunMigration()` and `RunMigration()`."
	anchors := extractAnchors(content)

	filePaths := 0
	funcNames := 0
	for _, a := range anchors {
		switch a.Type {
		case "file_path":
			filePaths++
		case "function_name":
			funcNames++
		}
	}
	if filePaths != 1 {
		t.Errorf("expected 1 file_path anchor, got %d", filePaths)
	}
	if funcNames != 1 {
		t.Errorf("expected 1 function_name anchor, got %d", funcNames)
	}
}

// --- verifyAnchor tests: file_path ---

func TestVerifyAnchor_KnownFile_OK(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{RepoPath: repo}, Anchor{Type: "file_path", Value: "README.md"})
	if !ok {
		t.Errorf("expected ok=true for tracked file, got reason: %s", reason)
	}
}

func TestVerifyAnchor_MissingFile_Broken(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{RepoPath: repo}, Anchor{Type: "file_path", Value: "nonexistent.go"})
	if ok {
		t.Error("expected ok=false for missing file")
	}
	if reason == "" {
		t.Error("expected non-empty reason for broken anchor")
	}
}

// --- verifyAnchor tests: function_name ---

func TestVerifyAnchor_KnownFunction_OK(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{RepoPath: repo}, Anchor{Type: "function_name", Value: "MyFunction"})
	if !ok {
		t.Errorf("expected ok=true for existing function, got reason: %s", reason)
	}
}

func TestVerifyAnchor_MissingFunction_Broken(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{RepoPath: repo}, Anchor{Type: "function_name", Value: "GhostFunction"})
	if ok {
		t.Error("expected ok=false for missing function")
	}
	if reason == "" {
		t.Error("expected non-empty reason for broken anchor")
	}
}

// --- verifyAnchor tests: type_name ---

func TestVerifyAnchor_KnownType_OK(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{RepoPath: repo}, Anchor{Type: "type_name", Value: "MyStruct"})
	if !ok {
		t.Errorf("expected ok=true for existing type, got reason: %s", reason)
	}
}

func TestVerifyAnchor_MissingType_Broken(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{RepoPath: repo}, Anchor{Type: "type_name", Value: "GhostStruct"})
	if ok {
		t.Error("expected ok=false for missing type")
	}
	if reason == "" {
		t.Error("expected non-empty reason for broken anchor")
	}
}

// --- verifyAnchor tests: db_table / db_column (skip when no DBConnStr) ---

func TestVerifyAnchor_DBTable_NoConn_Skipped(t *testing.T) {
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{}, Anchor{Type: "db_table", Value: "schedules"})
	if !ok {
		t.Errorf("expected ok=true (skipped) when no db_conn_str, got reason: %s", reason)
	}
}

func TestVerifyAnchor_DBColumn_NoConn_Skipped(t *testing.T) {
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{}, Anchor{Type: "db_column", Value: "workflow_type"})
	if !ok {
		t.Errorf("expected ok=true (skipped) when no db_conn_str, got reason: %s", reason)
	}
}

// --- verifyAnchor tests: config_key (skip when no query or no conn) ---

func TestVerifyAnchor_ConfigKey_NoQuery_Skipped(t *testing.T) {
	ok, reason := verifyAnchor(context.Background(), DriftScanConfig{}, Anchor{Type: "config_key", Value: "claim_extraction_model"})
	if !ok {
		t.Errorf("expected ok=true (skipped) when no config_key_query, got reason: %s", reason)
	}
}

func TestVerifyAnchor_ConfigKey_QueryButNoConn_Skipped(t *testing.T) {
	cfg := DriftScanConfig{ConfigKeyQuery: "SELECT 1 FROM config WHERE key"}
	ok, reason := verifyAnchor(context.Background(), cfg, Anchor{Type: "config_key", Value: "some_key"})
	if !ok {
		t.Errorf("expected ok=true (skipped) when db_conn_str is empty, got reason: %s", reason)
	}
}

// --- verifyAnchor tests: shard_id ---

// TestVerifyAnchor_ShardID_Missing exercises the shard_id verifier against a
// non-existent shard. cxp will exit non-zero (either "not found" or "command not found"),
// so verifyAnchor must return ok=false.
func TestVerifyAnchor_ShardID_Missing(t *testing.T) {
	ok, _ := verifyAnchor(context.Background(), DriftScanConfig{}, Anchor{Type: "shard_id", Value: "cp-nonexistent000"})
	if ok {
		t.Error("expected ok=false for non-existent shard")
	}
}

// --- DriftScanRunner basic tests ---

func TestDriftScanRunner_Name(t *testing.T) {
	r := DriftScanRunner{}
	if r.Name() != "drift-scan" {
		t.Errorf("expected 'drift-scan', got %q", r.Name())
	}
}

func TestDriftScanRunner_Run_MissingRepoPath(t *testing.T) {
	r := DriftScanRunner{}
	cfg, _ := json.Marshal(DriftScanConfig{GapsShard: "cp-test-gaps"})
	_, _, err := r.Run(context.Background(), cfg)
	if err == nil {
		t.Error("expected error when repo_path is missing")
	}
}

func TestDriftScanRunner_Run_MissingGapsShard(t *testing.T) {
	r := DriftScanRunner{}
	cfg, _ := json.Marshal(DriftScanConfig{RepoPath: "/tmp/repo"})
	_, _, err := r.Run(context.Background(), cfg)
	if err == nil {
		t.Error("expected error when gaps_shard is missing")
	}
}

// --- LLM extractor tests ---

func TestExtractClaimsLLM_ValidResponse(t *testing.T) {
	gen := &generation.MockGenerator{
		GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
			return `[{"type":"file_path","value":"cmd/main.go"},{"type":"type_name","value":"DriftScanConfig"}]`, nil
		},
	}
	anchors, err := extractClaimsLLM(context.Background(), "some article", gen, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anchors) != 2 {
		t.Fatalf("expected 2 anchors, got %d", len(anchors))
	}
	if anchors[0].Type != "file_path" || anchors[0].Value != "cmd/main.go" {
		t.Errorf("unexpected anchor[0]: %+v", anchors[0])
	}
	if anchors[1].Type != "type_name" || anchors[1].Value != "DriftScanConfig" {
		t.Errorf("unexpected anchor[1]: %+v", anchors[1])
	}
}

func TestExtractClaimsLLM_MarkdownWrapped(t *testing.T) {
	gen := &generation.MockGenerator{
		GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
			return "```json\n[{\"type\":\"type_name\",\"value\":\"MyStruct\"}]\n```", nil
		},
	}
	anchors, err := extractClaimsLLM(context.Background(), "article", gen, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anchors) != 1 || anchors[0].Type != "type_name" || anchors[0].Value != "MyStruct" {
		t.Errorf("unexpected anchors: %+v", anchors)
	}
}

func TestExtractClaimsLLM_MaxClaimsCap(t *testing.T) {
	claims := make([]llmClaim, 10)
	for i := range claims {
		claims[i] = llmClaim{Type: "file_path", Value: fmt.Sprintf("file%d.go", i)}
	}
	jsonBytes, _ := json.Marshal(claims)
	gen := &generation.MockGenerator{
		GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
			return string(jsonBytes), nil
		},
	}
	anchors, err := extractClaimsLLM(context.Background(), "article", gen, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anchors) != 5 {
		t.Errorf("expected 5 anchors (capped at MaxClaimsPerArticle), got %d", len(anchors))
	}
}

func TestExtractClaimsLLM_LLMError_ReturnsError(t *testing.T) {
	gen := &generation.MockGenerator{
		GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
			return "", fmt.Errorf("LLM unavailable")
		},
	}
	_, err := extractClaimsLLM(context.Background(), "article", gen, 50)
	if err == nil {
		t.Error("expected error when LLM generation fails")
	}
}

func TestExtractClaimsLLM_InvalidJSON_ReturnsError(t *testing.T) {
	gen := &generation.MockGenerator{
		GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
			return "not json at all", nil
		},
	}
	_, err := extractClaimsLLM(context.Background(), "article", gen, 50)
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestExtractClaimsLLM_FiltersInvalidTypes(t *testing.T) {
	gen := &generation.MockGenerator{
		GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
			return `[{"type":"unknown_type","value":"foo"},{"type":"file_path","value":"valid.go"}]`, nil
		},
	}
	anchors, err := extractClaimsLLM(context.Background(), "article", gen, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anchors) != 1 || anchors[0].Value != "valid.go" {
		t.Errorf("expected only valid claim types, got: %+v", anchors)
	}
}

// --- mergeAndDedup tests ---

func TestMergeAndDedup_DedupsCrossSources(t *testing.T) {
	regex := []Anchor{
		{Type: "file_path", Value: "main.go"},
		{Type: "function_name", Value: "RunIt"},
	}
	llm := []Anchor{
		{Type: "file_path", Value: "main.go"}, // duplicate
		{Type: "type_name", Value: "MyStruct"},
	}
	merged := mergeAndDedup(regex, llm)
	if len(merged) != 3 {
		t.Errorf("expected 3 unique anchors, got %d: %+v", len(merged), merged)
	}
}

func TestMergeAndDedup_RegexFirst(t *testing.T) {
	regex := []Anchor{{Type: "file_path", Value: "a.go"}}
	llm := []Anchor{{Type: "file_path", Value: "b.go"}}
	merged := mergeAndDedup(regex, llm)
	if len(merged) != 2 {
		t.Fatalf("expected 2, got %d", len(merged))
	}
	if merged[0].Value != "a.go" {
		t.Error("expected regex anchor to appear first")
	}
}

func TestMergeAndDedup_Empty(t *testing.T) {
	merged := mergeAndDedup(nil, nil)
	if len(merged) != 0 {
		t.Errorf("expected empty, got %d", len(merged))
	}
}

// --- parseModelString tests ---

func TestParseModelString(t *testing.T) {
	tests := []struct {
		input    string
		provider string
		model    string
		wantErr  bool
	}{
		{"gemini/gemini-2.0-flash", "gemini", "gemini-2.0-flash", false},
		{"google/gemini-2.0-flash", "google", "gemini-2.0-flash", false},
		{"noSlash", "", "", true},
		{"/noProvider", "", "", true},
		{"provider/", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, m, err := parseModelString(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p != tt.provider {
				t.Errorf("expected provider %q, got %q", tt.provider, p)
			}
			if m != tt.model {
				t.Errorf("expected model %q, got %q", tt.model, m)
			}
		})
	}
}

// --- extractJSONArray tests ---

func TestExtractJSONArray_Plain(t *testing.T) {
	input := `[{"type":"file_path","value":"x.go"}]`
	got := extractJSONArray(input)
	if got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestExtractJSONArray_MarkdownFence(t *testing.T) {
	input := "```json\n[{\"type\":\"file_path\",\"value\":\"x.go\"}]\n```"
	got := extractJSONArray(input)
	if got != `[{"type":"file_path","value":"x.go"}]` {
		t.Errorf("unexpected result: %q", got)
	}
}

func TestExtractJSONArray_PreambleText(t *testing.T) {
	input := "Here are the claims:\n[{\"type\":\"type_name\",\"value\":\"Foo\"}]\nEnd."
	got := extractJSONArray(input)
	if got != `[{"type":"type_name","value":"Foo"}]` {
		t.Errorf("unexpected result: %q", got)
	}
}
