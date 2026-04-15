package workflows

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExtractYAMLBlock_FencedBlock(t *testing.T) {
	content := "# Header\n\nSome prose.\n\n```yaml\n- q: test\n  expected_facts: [foo]\n```\n\nMore prose."
	got := extractYAMLBlock(content)
	want := "\n- q: test\n  expected_facts: [foo]\n"
	if got != want {
		t.Errorf("extractYAMLBlock() = %q, want %q", got, want)
	}
}

func TestExtractYAMLBlock_NoFence_ReturnsFullContent(t *testing.T) {
	content := "- q: test\n  expected_facts: [foo]\n"
	got := extractYAMLBlock(content)
	if got != content {
		t.Errorf("extractYAMLBlock() = %q, want %q (full content)", got, content)
	}
}

func TestLoadCanaryQuestions_ParsesFencedYAML(t *testing.T) {
	// Simulate getShardContent by testing extractYAMLBlock + yaml.Unmarshal path
	// directly via a content string with a fenced block.
	content := `# Canary Questions

` + "```yaml" + `
- q: "What model does classify_project use?"
  expected_facts: ["gemini-2.5-flash", "classify_project"]
  source_kb: pf-861f0c

- q: "Where are skill files stored?"
  expected_facts: ["skills/"]
  source_kb: pf-2091d5
` + "```" + `
`
	yamlBlock := extractYAMLBlock(content)
	questions := mustParseCanaryYAML(t, yamlBlock)
	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}
	if questions[0].Q != "What model does classify_project use?" {
		t.Errorf("q[0].Q = %q", questions[0].Q)
	}
	if len(questions[0].ExpectedFacts) != 2 {
		t.Errorf("q[0].ExpectedFacts len = %d, want 2", len(questions[0].ExpectedFacts))
	}
	if questions[0].SourceKB != "pf-861f0c" {
		t.Errorf("q[0].SourceKB = %q, want pf-861f0c", questions[0].SourceKB)
	}
}

func TestLoadCanaryQuestions_ParsesBareYAML(t *testing.T) {
	content := `- q: "bare question"
  expected_facts: ["fact1", "fact2"]
`
	yamlBlock := extractYAMLBlock(content)
	questions := mustParseCanaryYAML(t, yamlBlock)
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if questions[0].Q != "bare question" {
		t.Errorf("q[0].Q = %q, want %q", questions[0].Q, "bare question")
	}
}

func TestLoadCanaryQuestions_IgnoresMissingQOrFacts(t *testing.T) {
	content := `- q: "valid question"
  expected_facts: ["fact1"]
- q: ""
  expected_facts: ["fact2"]
- q: "no facts"
  expected_facts: []
- expected_facts: ["fact3"]
`
	yamlBlock := extractYAMLBlock(content)
	questions := mustParseCanaryYAML(t, yamlBlock)
	// Filter the same way loadCanaryQuestions does
	var valid []CanaryQuestion
	for _, q := range questions {
		if q.Q != "" && len(q.ExpectedFacts) > 0 {
			valid = append(valid, q)
		}
	}
	if len(valid) != 1 {
		t.Fatalf("expected 1 valid question after filtering, got %d", len(valid))
	}
	if valid[0].Q != "valid question" {
		t.Errorf("valid[0].Q = %q, want %q", valid[0].Q, "valid question")
	}
}

func TestFindMissingFacts_AllPresent(t *testing.T) {
	response := "The classify_project stage uses gemini-2.5-flash for routing."
	missing := findMissingFacts(response, []string{"gemini-2.5-flash", "classify_project"})
	if len(missing) != 0 {
		t.Errorf("expected no missing facts, got %v", missing)
	}
}

func TestFindMissingFacts_CaseInsensitive(t *testing.T) {
	response := "GEMINI-2.5-FLASH is the model used."
	missing := findMissingFacts(response, []string{"gemini-2.5-flash"})
	if len(missing) != 0 {
		t.Errorf("expected no missing facts (case insensitive), got %v", missing)
	}
}

func TestFindMissingFacts_SomeMissing(t *testing.T) {
	response := "The model is gemini-2.5-flash."
	missing := findMissingFacts(response, []string{"gemini-2.5-flash", "classify_project", "ai_routing_rules"})
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing facts, got %v", missing)
	}
	got := map[string]bool{}
	for _, m := range missing {
		got[m] = true
	}
	if !got["classify_project"] || !got["ai_routing_rules"] {
		t.Errorf("unexpected missing set: %v", missing)
	}
}

func TestFindMissingFacts_EmptyResponse(t *testing.T) {
	missing := findMissingFacts("", []string{"fact1", "fact2"})
	if len(missing) != 2 {
		t.Errorf("expected 2 missing facts for empty response, got %v", missing)
	}
}

func TestCanaryRunner_Name(t *testing.T) {
	r := CanaryRunner{}
	if got := r.Name(); got != "canary" {
		t.Errorf("Name() = %q, want %q", got, "canary")
	}
}

func TestCanaryRunner_Run_MissingCanaryShard(t *testing.T) {
	r := CanaryRunner{}
	cfg, _ := json.Marshal(CanaryConfig{GapsShard: "cp-gaps-123"})
	_, _, err := r.Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error when canary_shard is missing")
	}
}

func TestCanaryRunner_Run_MissingGapsShard(t *testing.T) {
	r := CanaryRunner{}
	cfg, _ := json.Marshal(CanaryConfig{CanaryShard: "cp-canary-123"})
	_, _, err := r.Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error when gaps_shard is missing")
	}
}

func TestCanaryRunner_Run_MissingBothShards(t *testing.T) {
	r := CanaryRunner{}
	_, _, err := r.Run(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error when both shards are missing")
	}
}

func TestCanaryRunner_Run_NilConfig(t *testing.T) {
	r := CanaryRunner{}
	_, _, err := r.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when config is nil (missing required fields)")
	}
}

func TestCanaryRunner_Run_EmptyShard(t *testing.T) {
	// Override getShardContentFn to return empty content (simulates an unseeded shard).
	orig := getShardContentFn
	getShardContentFn = func(_ context.Context, _ string) (string, error) {
		return "", nil
	}
	defer func() { getShardContentFn = orig }()

	r := CanaryRunner{}
	cfg, _ := json.Marshal(CanaryConfig{
		CanaryShard: "cp-canary-test",
		GapsShard:   "cp-gaps-test",
	})
	summary, _, err := r.Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for empty canaries shard, got nil")
	}
	if !strings.Contains(summary, "seed-canaries") {
		t.Errorf("summary %q does not contain 'seed-canaries'", summary)
	}
	if !strings.Contains(err.Error(), "seed-canaries") {
		t.Errorf("error %q does not contain 'seed-canaries'", err.Error())
	}
}

// mustParseCanaryYAML is a test helper that parses YAML into []CanaryQuestion.
func mustParseCanaryYAML(t *testing.T, yamlBlock string) []CanaryQuestion {
	t.Helper()
	var questions []CanaryQuestion
	if err := yaml.Unmarshal([]byte(yamlBlock), &questions); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}
	return questions
}
