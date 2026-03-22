package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPipelineConfig(t *testing.T) {
	cfg := DefaultPipelineConfig()

	if cfg.Dispatch.MaxConcurrent != 3 {
		t.Errorf("MaxConcurrent = %d, want 3", cfg.Dispatch.MaxConcurrent)
	}
	if cfg.Dispatch.TmuxSession != "main" {
		t.Errorf("TmuxSession = %q, want %q", cfg.Dispatch.TmuxSession, "main")
	}
	if cfg.Dispatch.ClaudeFlags != "--print" {
		t.Errorf("ClaudeFlags = %q, want %q", cfg.Dispatch.ClaudeFlags, "--print")
	}
	if cfg.SkillsDir != "skills" {
		t.Errorf("SkillsDir = %q, want %q", cfg.SkillsDir, "skills")
	}
}

func TestMergePipelineConfig_SliceOverride(t *testing.T) {
	base := &PipelineConfig{
		Build: []string{"make build"},
		Test:  []string{"make test"},
	}
	override := &PipelineConfig{
		Build: []string{"go build ./..."},
	}

	merged := MergePipelineConfig(base, override)

	if len(merged.Build) != 1 || merged.Build[0] != "go build ./..." {
		t.Errorf("Build = %v, want [go build ./...]", merged.Build)
	}
	// Test should be preserved from base
	if len(merged.Test) != 1 || merged.Test[0] != "make test" {
		t.Errorf("Test = %v, want [make test]", merged.Test)
	}
}

func TestMergePipelineConfig_MapMerge(t *testing.T) {
	base := &PipelineConfig{
		Agents: map[string]AgentCfg{
			"steve": {Domains: []string{"backend"}},
			"alice": {Domains: []string{"frontend"}},
		},
	}
	override := &PipelineConfig{
		Agents: map[string]AgentCfg{
			"steve": {Domains: []string{"backend", "infra"}},
		},
	}

	merged := MergePipelineConfig(base, override)

	if len(merged.Agents) != 2 {
		t.Fatalf("Agents count = %d, want 2", len(merged.Agents))
	}
	// steve should be overridden
	if len(merged.Agents["steve"].Domains) != 2 {
		t.Errorf("steve domains = %v, want [backend infra]", merged.Agents["steve"].Domains)
	}
	// alice should be preserved
	if len(merged.Agents["alice"].Domains) != 1 || merged.Agents["alice"].Domains[0] != "frontend" {
		t.Errorf("alice domains = %v, want [frontend]", merged.Agents["alice"].Domains)
	}
}

func TestMergePipelineConfig_ScalarOverride(t *testing.T) {
	base := &PipelineConfig{
		Dispatch: DispatchCfg{
			MaxConcurrent: 3,
			TmuxSession:   "main",
			ClaudeFlags:   "--print",
		},
		GitHub: GitHubCfg{OwnerRepo: "org/repo"},
	}
	override := &PipelineConfig{
		Dispatch: DispatchCfg{
			MaxConcurrent: 5,
		},
		GitHub: GitHubCfg{OwnerRepo: "other/repo"},
	}

	merged := MergePipelineConfig(base, override)

	if merged.Dispatch.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want 5", merged.Dispatch.MaxConcurrent)
	}
	// Zero-value strings in override should NOT replace base
	if merged.Dispatch.TmuxSession != "main" {
		t.Errorf("TmuxSession = %q, want %q", merged.Dispatch.TmuxSession, "main")
	}
	if merged.GitHub.OwnerRepo != "other/repo" {
		t.Errorf("OwnerRepo = %q, want %q", merged.GitHub.OwnerRepo, "other/repo")
	}
}

func TestMergePipelineConfig_NoMutation(t *testing.T) {
	base := &PipelineConfig{
		Build: []string{"make build"},
		Agents: map[string]AgentCfg{
			"steve": {Domains: []string{"backend"}},
		},
	}
	override := &PipelineConfig{
		Build: []string{"go build"},
		Agents: map[string]AgentCfg{
			"bob": {Domains: []string{"ops"}},
		},
	}

	_ = MergePipelineConfig(base, override)

	// Verify base wasn't mutated
	if len(base.Build) != 1 || base.Build[0] != "make build" {
		t.Errorf("base.Build was mutated: %v", base.Build)
	}
	if _, ok := base.Agents["bob"]; ok {
		t.Error("base.Agents was mutated: bob was added")
	}
}

func TestLoadPipelineConfig_MissingFiles(t *testing.T) {
	// Use a temp dir that won't have any .cxp/pipeline.yaml
	tmp := t.TempDir()
	cfg, err := LoadPipelineConfig(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return defaults
	if cfg.Dispatch.MaxConcurrent != 3 {
		t.Errorf("MaxConcurrent = %d, want 3 (default)", cfg.Dispatch.MaxConcurrent)
	}
	if cfg.SkillsDir != "skills" {
		t.Errorf("SkillsDir = %q, want %q (default)", cfg.SkillsDir, "skills")
	}
}

func TestLoadPipelineConfig_RepoOverride(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, ".cxp")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlContent := []byte("dispatch:\n  max_concurrent: 7\nskills_dir: custom_skills\n")
	if err := os.WriteFile(filepath.Join(repoDir, "pipeline.yaml"), yamlContent, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadPipelineConfig(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Dispatch.MaxConcurrent != 7 {
		t.Errorf("MaxConcurrent = %d, want 7", cfg.Dispatch.MaxConcurrent)
	}
	if cfg.SkillsDir != "custom_skills" {
		t.Errorf("SkillsDir = %q, want %q", cfg.SkillsDir, "custom_skills")
	}
	// Defaults should still be present for non-overridden fields
	if cfg.Dispatch.TmuxSession != "main" {
		t.Errorf("TmuxSession = %q, want %q (default)", cfg.Dispatch.TmuxSession, "main")
	}
}

func TestResolveSkill_RepoBeforeGlobal(t *testing.T) {
	tmp := t.TempDir()

	// Create repo-level skill
	repoSkillDir := filepath.Join(tmp, "skills", "my-skill")
	if err := os.MkdirAll(repoSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a marker file so the directory exists
	if err := os.WriteFile(filepath.Join(repoSkillDir, "skill.yaml"), []byte("name: my-skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, err := ResolveSkill(tmp, "my-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != repoSkillDir {
		t.Errorf("path = %q, want %q", path, repoSkillDir)
	}
}

func TestResolveSkill_NotFound(t *testing.T) {
	tmp := t.TempDir()
	_, err := ResolveSkill(tmp, "nonexistent-skill")
	if err == nil {
		t.Fatal("expected error for missing skill, got nil")
	}
}
