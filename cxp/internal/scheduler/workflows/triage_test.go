package workflows

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTriageRunner_Name(t *testing.T) {
	r := TriageRunner{}
	if got := r.Name(); got != "triage" {
		t.Errorf("Name() = %q, want %q", got, "triage")
	}
}

func TestTriageRunner_Run_MissingConfig(t *testing.T) {
	r := TriageRunner{}

	// nil config
	_, _, err := r.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
	if !strings.Contains(err.Error(), "gaps_shard") {
		t.Errorf("error should mention gaps_shard, got: %v", err)
	}

	// empty JSON object missing required fields
	_, _, err = r.Run(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for empty config, got nil")
	}
	if !strings.Contains(err.Error(), "gaps_shard") {
		t.Errorf("error should mention gaps_shard, got: %v", err)
	}

	// only gaps_shard provided
	_, _, err = r.Run(context.Background(), json.RawMessage(`{"gaps_shard":"cp-abc"}`))
	if err == nil {
		t.Fatal("expected error when escalations_shard missing, got nil")
	}
	if !strings.Contains(err.Error(), "escalations_shard") {
		t.Errorf("error should mention escalations_shard, got: %v", err)
	}
}

func TestTriageRunner_Run_InvalidConfig(t *testing.T) {
	r := TriageRunner{}
	_, _, err := r.Run(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON config, got nil")
	}
}

func TestParseGapEntries_WellFormed(t *testing.T) {
	input := `2026-04-13 | drift-scan | drift-detected | article=cp-abc123 type=file_path value=services/foo.go reason=file not tracked in repo
2026-04-13 | canary | retrieval-failure | q="What model does X use?" expected=[...] missing=[...] source=pf-xyz`

	entries := parseGapEntries(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	e := entries[0]
	if e.Date != "2026-04-13" {
		t.Errorf("entry[0].Date = %q, want %q", e.Date, "2026-04-13")
	}
	if e.Source != "drift-scan" {
		t.Errorf("entry[0].Source = %q, want %q", e.Source, "drift-scan")
	}
	if e.Category != "drift-detected" {
		t.Errorf("entry[0].Category = %q, want %q", e.Category, "drift-detected")
	}
	if !strings.Contains(e.Description, "cp-abc123") {
		t.Errorf("entry[0].Description missing expected content, got: %q", e.Description)
	}

	e = entries[1]
	if e.Category != "retrieval-failure" {
		t.Errorf("entry[1].Category = %q, want %q", e.Category, "retrieval-failure")
	}
}

func TestParseGapEntries_SkipsInvalidLines(t *testing.T) {
	input := `
# this is a comment
2026-04-13 | drift-scan | drift-detected | valid entry
not a valid line
| missing date | category | description
2026-04-14 | canary | retrieval-failure | another valid entry
`
	entries := parseGapEntries(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(entries), entries)
	}
}

func TestParseGapEntries_Empty(t *testing.T) {
	entries := parseGapEntries("")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", len(entries))
	}
}

func TestParseGapEntries_TrimsWhitespace(t *testing.T) {
	input := "  2026-04-13  |  drift-scan  |  drift-detected  |  some description  "
	entries := parseGapEntries(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Source != "drift-scan" {
		t.Errorf("Source not trimmed, got %q", entries[0].Source)
	}
	if entries[0].Category != "drift-detected" {
		t.Errorf("Category not trimmed, got %q", entries[0].Category)
	}
}

func TestProposeActions_OneTaskPerCategory(t *testing.T) {
	items := []GapEntry{
		{Date: "2026-04-13", Source: "drift-scan", Category: "drift-detected", Description: "broken anchor A"},
		{Date: "2026-04-13", Source: "drift-scan", Category: "drift-detected", Description: "broken anchor B"},
	}

	proposals := proposeActions("drift-detected", items, 3)

	// Count task proposals
	taskCount := 0
	for _, p := range proposals {
		if p.Kind == "task" {
			taskCount++
		}
	}
	if taskCount != 1 {
		t.Errorf("expected 1 task proposal, got %d", taskCount)
	}
}

func TestProposeActions_EscalationOnRecurringDescription(t *testing.T) {
	desc := "article=cp-abc123 reason=file not found"
	items := []GapEntry{
		{Date: "2026-04-11", Source: "drift-scan", Category: "drift-detected", Description: desc},
		{Date: "2026-04-12", Source: "drift-scan", Category: "drift-detected", Description: desc},
		{Date: "2026-04-13", Source: "drift-scan", Category: "drift-detected", Description: desc},
	}

	proposals := proposeActions("drift-detected", items, 3)

	escalationCount := 0
	for _, p := range proposals {
		if p.Kind == "escalation" {
			escalationCount++
			if !strings.Contains(p.Title, "drift-detected") {
				t.Errorf("escalation title should contain category, got: %q", p.Title)
			}
		}
	}
	if escalationCount != 1 {
		t.Errorf("expected 1 escalation, got %d", escalationCount)
	}
}

func TestProposeActions_NoEscalationBelowThreshold(t *testing.T) {
	desc := "some recurring issue"
	items := []GapEntry{
		{Date: "2026-04-12", Source: "canary", Category: "retrieval-failure", Description: desc},
		{Date: "2026-04-13", Source: "canary", Category: "retrieval-failure", Description: desc},
	}

	proposals := proposeActions("retrieval-failure", items, 3)

	for _, p := range proposals {
		if p.Kind == "escalation" {
			t.Errorf("expected no escalation below threshold, got one: %+v", p)
		}
	}
}

func TestProposeActions_Empty(t *testing.T) {
	proposals := proposeActions("drift-detected", nil, 3)
	if len(proposals) != 0 {
		t.Errorf("expected no proposals for empty items, got %d", len(proposals))
	}
}

func TestActionHintForCategory_KnownCategories(t *testing.T) {
	known := []string{"drift-detected", "retrieval-failure", "hallucination", "omission", "coverage-hole"}
	for _, cat := range known {
		hint := actionHintForCategory(cat)
		if hint == "" {
			t.Errorf("actionHintForCategory(%q) returned empty string", cat)
		}
		if strings.Contains(hint, "review entries for patterns") {
			t.Errorf("actionHintForCategory(%q) returned generic hint, expected specific one", cat)
		}
	}
}

func TestActionHintForCategory_Unknown(t *testing.T) {
	hint := actionHintForCategory("some-unknown-category")
	if hint == "" {
		t.Error("actionHintForCategory with unknown category returned empty string")
	}
	if !strings.Contains(hint, "review entries for patterns") {
		t.Errorf("expected generic hint for unknown category, got: %q", hint)
	}
}
