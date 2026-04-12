package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
)

func TestCanaryWorkflowRecordsFailures(t *testing.T) {
	runner := fakeRunner{
		responses: map[string]fakeResponse{
			commandKey("cxp", "--output", "json", "shard", "show", "pf-kb-canaries"): {
				stdout: `{"content":"questions:\n  - question: \"How do I connect to prod?\"\n    expected_facts:\n      - \"psql\"\n      - \"~/.pgpass\"\n  - question: \"What port does the API listen on?\"\n    expected_facts:\n      - \"8080\"\n"}`,
			},
			commandKey("claude", "--model", "gemini/gemini-2.5-flash", "--allowedTools", "cxp kb search,cxp kb show", "--print", "How do I connect to prod?"): {
				stdout: "Use PSQL and store credentials in ~/.pgpass.",
			},
			commandKey("claude", "--model", "gemini/gemini-2.5-flash", "--allowedTools", "cxp kb search,cxp kb show", "--print", "What port does the API listen on?"): {
				stdout: "The API server is fronted by nginx.",
			},
		},
	}
	gaps := &fakeGapAppender{}
	workflow := NewCanaryWorkflow(Dependencies{
		Runner: runner,
		Gaps:   gaps,
		Now: func() time.Time {
			return time.Date(2026, 4, 12, 8, 30, 0, 0, time.UTC)
		},
	})

	resultAny, err := workflow.Run(context.Background(), client.Schedule{
		Project:      "pf",
		WorkflowType: "canary",
		Config:       json.RawMessage(`{"canary_shard":"pf-kb-canaries"}`),
	}, client.ScheduleRun{ID: 42})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	result := resultAny.(CanaryResult)
	if result.QuestionsTested != 2 || result.Passed != 1 || result.Failed != 1 {
		t.Fatalf("unexpected result counts: %+v", result)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(result.Failures))
	}
	if got := result.Failures[0].MissingFacts; len(got) != 1 || got[0] != "8080" {
		t.Fatalf("unexpected missing facts: %#v", got)
	}
	if len(gaps.entries) != 1 {
		t.Fatalf("expected 1 gap append, got %d", len(gaps.entries))
	}
	if gaps.entries[0].shardID != "pf-kb-gaps" {
		t.Fatalf("gap appended to wrong shard: %s", gaps.entries[0].shardID)
	}
	if !strings.Contains(gaps.entries[0].content, `2026-04-12 | canary-42 | retrieval-failure | Q: "What port does the API listen on?" expected: [8080] missing: [8080]`) {
		t.Fatalf("unexpected gap entry: %s", gaps.entries[0].content)
	}
}

func TestCanaryWorkflowHandlesEmptyShard(t *testing.T) {
	runner := fakeRunner{
		responses: map[string]fakeResponse{
			commandKey("cxp", "--output", "json", "shard", "show", "pf-kb-canaries"): {
				stdout: `{"content":""}`,
			},
		},
	}
	gaps := &fakeGapAppender{}
	workflow := NewCanaryWorkflow(Dependencies{Runner: runner, Gaps: gaps, Now: time.Now})

	resultAny, err := workflow.Run(context.Background(), client.Schedule{
		Project:      "pf",
		WorkflowType: "canary",
		Config:       json.RawMessage(`{"canary_shard":"pf-kb-canaries"}`),
	}, client.ScheduleRun{ID: 7})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	result := resultAny.(CanaryResult)
	if result.QuestionsTested != 0 || result.Passed != 0 || result.Failed != 0 {
		t.Fatalf("unexpected result counts: %+v", result)
	}
	if len(gaps.entries) != 0 {
		t.Fatalf("expected no gap entries, got %d", len(gaps.entries))
	}
}

func TestCanaryWorkflowContinuesAfterDispatchFailure(t *testing.T) {
	runner := fakeRunner{
		responses: map[string]fakeResponse{
			commandKey("cxp", "--output", "json", "shard", "show", "pf-kb-canaries"): {
				stdout: "{\"content\":\"```yaml\\nquestions:\\n  - question: \\\"First\\\"\\n    expected_facts:\\n      - \\\"alpha\\\"\\n  - question: \\\"Second\\\"\\n    expected_facts:\\n      - \\\"beta\\\"\\n```\"}",
			},
			commandKey("claude", "--model", "gemini/gemini-2.5-flash", "--allowedTools", "cxp kb search,cxp kb show", "--print", "First"): {
				err: errors.New("exit status 1"),
			},
			commandKey("claude", "--model", "gemini/gemini-2.5-flash", "--allowedTools", "cxp kb search,cxp kb show", "--print", "Second"): {
				stdout: "beta is the value",
			},
		},
	}
	gaps := &fakeGapAppender{}
	workflow := NewCanaryWorkflow(Dependencies{
		Runner: runner,
		Gaps:   gaps,
		Now: func() time.Time {
			return time.Date(2026, 4, 12, 8, 30, 0, 0, time.UTC)
		},
	})

	resultAny, err := workflow.Run(context.Background(), client.Schedule{
		Project:      "pf",
		WorkflowType: "canary",
		Config:       json.RawMessage(`{"canary_shard":"pf-kb-canaries","gaps_shard":"custom-gaps"}`),
	}, client.ScheduleRun{ID: 9})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	result := resultAny.(CanaryResult)
	if result.QuestionsTested != 2 || result.Passed != 1 || result.Failed != 1 {
		t.Fatalf("unexpected result counts: %+v", result)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(result.Failures))
	}
	if got := result.Failures[0].MissingFacts; len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("unexpected missing facts: %#v", got)
	}
	if len(gaps.entries) != 1 || gaps.entries[0].shardID != "custom-gaps" {
		t.Fatalf("unexpected gap entries: %#v", gaps.entries)
	}
}

func TestRegistryIncludesCanary(t *testing.T) {
	registry := NewRegistry(Dependencies{})
	if _, ok := registry.Lookup("canary"); !ok {
		t.Fatal("expected canary workflow to be registered")
	}
	if _, ok := registry.Lookup("drift_scan"); !ok {
		t.Fatal("expected drift-scan workflow to be registered")
	}
}

type fakeRunner struct {
	responses map[string]fakeResponse
}

type fakeResponse struct {
	stdout string
	err    error
}

func (f fakeRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	key := commandKey(name, args...)
	response, ok := f.responses[key]
	if !ok {
		return "", errors.New("unexpected command: " + key)
	}
	return response.stdout, response.err
}

type fakeGapAppender struct {
	entries []gapEntry
}

type gapEntry struct {
	shardID string
	content string
}

func (f *fakeGapAppender) AppendShardContent(ctx context.Context, id, newContent string) error {
	f.entries = append(f.entries, gapEntry{shardID: id, content: newContent})
	return nil
}

func commandKey(name string, args ...string) string {
	return name + "\x1f" + strings.Join(args, "\x1f")
}
