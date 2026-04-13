package workflows

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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

	// Create a tracked file and a Go file with a function
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	goContent := "package main\n\nfunc MyFunction() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	return dir
}

func TestVerifyAnchor_KnownFile_OK(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(repo, Anchor{Type: "file_path", Value: "README.md"})
	if !ok {
		t.Errorf("expected ok=true for tracked file, got reason: %s", reason)
	}
}

func TestVerifyAnchor_MissingFile_Broken(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(repo, Anchor{Type: "file_path", Value: "nonexistent.go"})
	if ok {
		t.Error("expected ok=false for missing file")
	}
	if reason == "" {
		t.Error("expected non-empty reason for broken anchor")
	}
}

func TestVerifyAnchor_KnownFunction_OK(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(repo, Anchor{Type: "function_name", Value: "MyFunction"})
	if !ok {
		t.Errorf("expected ok=true for existing function, got reason: %s", reason)
	}
}

func TestVerifyAnchor_MissingFunction_Broken(t *testing.T) {
	repo := makeTempGitRepo(t)
	ok, reason := verifyAnchor(repo, Anchor{Type: "function_name", Value: "GhostFunction"})
	if ok {
		t.Error("expected ok=false for missing function")
	}
	if reason == "" {
		t.Error("expected non-empty reason for broken anchor")
	}
}

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
