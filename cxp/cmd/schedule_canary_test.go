package cmd

import (
	"strings"
	"testing"

	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler/workflows"
)

func TestMergeCanaryQuestions_Dedup(t *testing.T) {
	existing := []workflows.CanaryQuestion{
		{Q: "What is this project?", ExpectedFacts: []string{"project"}},
	}
	newQs := []workflows.CanaryQuestion{
		{Q: "What is this project?", ExpectedFacts: []string{"project"}},
		{Q: "How do I get started?", ExpectedFacts: []string{"start"}},
	}

	merged, skipped := mergeCanaryQuestions(existing, newQs)

	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	if len(merged) != 2 {
		t.Errorf("len(merged) = %d, want 2", len(merged))
	}
	if merged[1].Q != "How do I get started?" {
		t.Errorf("merged[1].Q = %q, want 'How do I get started?'", merged[1].Q)
	}
}

func TestMergeCanaryQuestions_CaseInsensitive(t *testing.T) {
	existing := []workflows.CanaryQuestion{
		{Q: "WHAT IS THIS PROJECT?", ExpectedFacts: []string{"project"}},
	}
	newQs := []workflows.CanaryQuestion{
		{Q: "what is this project?", ExpectedFacts: []string{"project"}},
	}

	merged, skipped := mergeCanaryQuestions(existing, newQs)

	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	if len(merged) != 1 {
		t.Errorf("len(merged) = %d, want 1 (no new questions added)", len(merged))
	}
}

func TestMergeCanaryQuestions_EmptyExisting(t *testing.T) {
	newQs := []workflows.CanaryQuestion{
		{Q: "question one", ExpectedFacts: []string{"fact1"}},
		{Q: "question two", ExpectedFacts: []string{"fact2"}},
	}

	merged, skipped := mergeCanaryQuestions(nil, newQs)

	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	if len(merged) != 2 {
		t.Errorf("len(merged) = %d, want 2", len(merged))
	}
}

func TestMergeCanaryQuestions_Idempotent(t *testing.T) {
	qs := []workflows.CanaryQuestion{
		{Q: "question one", ExpectedFacts: []string{"fact1"}},
		{Q: "question two", ExpectedFacts: []string{"fact2"}},
	}

	merged1, _ := mergeCanaryQuestions(nil, qs)
	merged2, skipped := mergeCanaryQuestions(merged1, qs)

	if skipped != len(qs) {
		t.Errorf("skipped = %d, want %d on second run", skipped, len(qs))
	}
	if len(merged2) != len(merged1) {
		t.Errorf("len(merged2) = %d, want %d (idempotent)", len(merged2), len(merged1))
	}
}

func TestReplaceCanaryYAML_ReplacesExistingBlock(t *testing.T) {
	content := "# Canary Questions\n\n```yaml\n- q: old\n  expected_facts: [old]\n```\n\nsome trailing text"

	questions := []workflows.CanaryQuestion{
		{Q: "new question", ExpectedFacts: []string{"new"}, SourceKB: ""},
	}

	result, err := replaceCanaryYAML(content, questions)
	if err != nil {
		t.Fatalf("replaceCanaryYAML() error: %v", err)
	}

	if !strings.Contains(result, "```yaml") {
		t.Error("result missing ```yaml fence")
	}
	if !strings.Contains(result, "new question") {
		t.Error("result missing new question text")
	}
	if strings.Contains(result, "old question") || strings.Contains(result, "q: old") {
		t.Error("result still contains old question text")
	}
	if !strings.Contains(result, "# Canary Questions") {
		t.Error("result lost the header prose")
	}
	if !strings.Contains(result, "some trailing text") {
		t.Error("result lost trailing text")
	}
}

func TestReplaceCanaryYAML_AppendsWhenNoBlock(t *testing.T) {
	content := "# Canary Questions\n\nNo questions yet."

	questions := []workflows.CanaryQuestion{
		{Q: "first question", ExpectedFacts: []string{"fact1"}, SourceKB: ""},
	}

	result, err := replaceCanaryYAML(content, questions)
	if err != nil {
		t.Fatalf("replaceCanaryYAML() error: %v", err)
	}

	if !strings.Contains(result, "```yaml") {
		t.Error("result missing ```yaml fence")
	}
	if !strings.Contains(result, "first question") {
		t.Error("result missing first question text")
	}
	if !strings.Contains(result, "# Canary Questions") {
		t.Error("result lost the header prose")
	}
}

func TestExtractYAMLBlockFrom_FencedBlock(t *testing.T) {
	content := "Header\n\n```yaml\n- q: test\n  expected_facts: [foo]\n```\n\nTrailer"
	got := extractYAMLBlockFrom(content)
	want := "\n- q: test\n  expected_facts: [foo]\n"
	if got != want {
		t.Errorf("extractYAMLBlockFrom() = %q, want %q", got, want)
	}
}

func TestExtractYAMLBlockFrom_NoFence(t *testing.T) {
	content := "no yaml fence here"
	got := extractYAMLBlockFrom(content)
	if got != "" {
		t.Errorf("extractYAMLBlockFrom() = %q, want empty string", got)
	}
}

func TestParseCommaList_Basic(t *testing.T) {
	got := parseCommaList("fact1, fact2,  fact3")
	want := []string{"fact1", "fact2", "fact3"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestParseCommaList_Empty(t *testing.T) {
	got := parseCommaList("")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestParseCommaList_SingleValue(t *testing.T) {
	got := parseCommaList("fact1")
	if len(got) != 1 || got[0] != "fact1" {
		t.Errorf("got %v, want [fact1]", got)
	}
}

func TestCanaryValidationFlags_AllValid(t *testing.T) {
	q := workflows.CanaryQuestion{Q: "question", ExpectedFacts: []string{"fact"}, SourceKB: "cp-123"}
	seen := map[string]int{"question": 1}
	flags := canaryValidationFlags(q, seen)
	if len(flags) != 0 {
		t.Errorf("expected no flags for valid question, got %v", flags)
	}
}

func TestCanaryValidationFlags_MissingFacts(t *testing.T) {
	q := workflows.CanaryQuestion{Q: "question", ExpectedFacts: nil, SourceKB: "cp-123"}
	seen := map[string]int{"question": 1}
	flags := canaryValidationFlags(q, seen)
	if len(flags) != 1 || !strings.Contains(flags[0], "missing expected_facts") {
		t.Errorf("expected missing_facts flag, got %v", flags)
	}
}

func TestCanaryValidationFlags_EmptySourceKB(t *testing.T) {
	q := workflows.CanaryQuestion{Q: "question", ExpectedFacts: []string{"fact"}, SourceKB: ""}
	seen := map[string]int{"question": 1}
	flags := canaryValidationFlags(q, seen)
	if len(flags) != 1 || !strings.Contains(flags[0], "empty source_kb") {
		t.Errorf("expected empty_source_kb flag, got %v", flags)
	}
}

func TestCanaryValidationFlags_Duplicate(t *testing.T) {
	q := workflows.CanaryQuestion{Q: "question", ExpectedFacts: []string{"fact"}, SourceKB: "cp-123"}
	seen := map[string]int{"question": 2}
	flags := canaryValidationFlags(q, seen)
	if len(flags) != 1 || !strings.Contains(flags[0], "duplicate") {
		t.Errorf("expected duplicate flag, got %v", flags)
	}
}

func TestCanaryValidationFlags_MultipleWarnings(t *testing.T) {
	q := workflows.CanaryQuestion{Q: "question", ExpectedFacts: nil, SourceKB: ""}
	seen := map[string]int{"question": 2}
	flags := canaryValidationFlags(q, seen)
	if len(flags) != 3 {
		t.Errorf("expected 3 flags (missing_facts, empty_source_kb, duplicate), got %v", flags)
	}
}
