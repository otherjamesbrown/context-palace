package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	calls []string
	runFn func(name string, args ...string) ([]byte, error)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	return f.runFn(name, args...)
}

func TestParseGapEntries(t *testing.T) {
	content := strings.Join([]string{
		"2026-04-07T05:00:00Z | canary-1 | retrieval-failure | Search missed the release note",
		"not a gap line",
		"2026-04-08 | scan-1 | drift-detected | API version changed",
		"2026-04-08 | user | unknown | should be ignored",
	}, "\n")

	entries, err := parseGapEntries(content)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "retrieval-failure", entries[0].Category)
	require.Equal(t, "drift-detected", entries[1].Category)
}

func TestFilterEntriesSinceLastRun(t *testing.T) {
	lastRun := time.Date(2026, 4, 7, 5, 0, 0, 0, time.UTC)
	entries := []gapEntry{
		{Date: lastRun.Add(-time.Minute)},
		{Date: lastRun},
		{Date: lastRun.Add(time.Minute)},
	}

	filtered := filterEntriesSince(entries, &lastRun)
	require.Len(t, filtered, 1)
}

func TestDetectRecurringGaps(t *testing.T) {
	entries := []gapEntry{
		{Category: "coverage-hole", Description: "Missing onboarding guide"},
		{Category: "coverage-hole", Description: "Missing onboarding guide"},
		{Category: "coverage-hole", Description: "Missing onboarding guide"},
		{Category: "omission", Description: "Missing onboarding guide"},
	}

	recurring := detectRecurringGaps(entries)
	require.Len(t, recurring, 1)
	require.Equal(t, "coverage-hole", recurring[0].Category)
	require.Equal(t, 3, recurring[0].Count)
}

func TestTriageWorkflowRun(t *testing.T) {
	now := time.Date(2026, 4, 14, 5, 0, 0, 0, time.UTC)
	gapsContent := strings.Join([]string{
		"2026-04-01T05:00:00Z | canary-1 | retrieval-failure | Search missed release note facts",
		"2026-04-08T05:00:00Z | canary-2 | retrieval-failure | Search missed release note facts",
		"2026-04-09T05:00:00Z | user | coverage-hole | No article explaining onboarding flow",
		"2026-04-10T05:00:00Z | canary-3 | retrieval-failure | Search missed release note facts",
		"2026-04-11T05:00:00Z | judge-1 | omission | Troubleshooting section omitted rollback steps",
	}, "\n")

	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			cmdline := name + " " + strings.Join(args, " ")
			switch {
			case strings.HasPrefix(cmdline, "cxp shard show"):
				return []byte(fmt.Sprintf(`{"content":%q}`, gapsContent)), nil
			case strings.HasPrefix(cmdline, "claude --model triage-model --print"):
				prompt := args[len(args)-1]
				switch {
				case strings.Contains(prompt, "Category: coverage-hole"):
					return []byte(`{"summary":"Need a new article.","actions":[{"title":"Draft onboarding KB article","body":"Create an outline covering setup, prerequisites, and common failure modes."}]}`), nil
				case strings.Contains(prompt, "Category: omission"):
					return []byte("```json\n{\"summary\":\"Expand the troubleshooting doc.\",\"actions\":[{\"title\":\"Add rollback troubleshooting steps\",\"body\":\"Update the troubleshooting KB entry with rollback guidance and examples.\"}]}\n```"), nil
				case strings.Contains(prompt, "Category: retrieval-failure"):
					return []byte(`{"summary":"Search terms are weak.","actions":[{"title":"Tune KB titles for release notes","body":"Improve titles and headings so release-note facts rank higher in KB search."}]}`), nil
				default:
					return nil, fmt.Errorf("unexpected prompt: %s", prompt)
				}
			case strings.HasPrefix(cmdline, "cxp task create Draft onboarding KB article"):
				return []byte(`{"id":"cp-task-1"}`), nil
			case strings.HasPrefix(cmdline, "cxp task create Add rollback troubleshooting steps"):
				return []byte(`{"id":"cp-task-2"}`), nil
			case strings.HasPrefix(cmdline, "cxp task create Tune KB titles for release notes"):
				return []byte(`{"id":"cp-task-3"}`), nil
			case strings.HasPrefix(cmdline, "cxp shard append"):
				return []byte(`{"id":"cp-escalations","appended":true}`), nil
			case strings.HasPrefix(cmdline, "cxp knowledge create KB Triage Report 2026-04-14"):
				return []byte(`{"id":"cp-report-1"}`), nil
			default:
				return nil, fmt.Errorf("unexpected command: %s", cmdline)
			}
		},
	}

	workflow := NewTriageWorkflow(runner, func() time.Time { return now })
	resultJSON, err := workflow.Run(context.Background(), Request{
		Config:         json.RawMessage(`{"gaps_shard":"cp-gaps","escalations_shard":"cp-escalations","triage_model":"triage-model"}`),
		PreviousResult: json.RawMessage(`{"last_triage_at":"2026-04-07T05:00:00Z"}`),
	})
	require.NoError(t, err)

	var result TriageResult
	require.NoError(t, json.Unmarshal(resultJSON, &result))
	require.Equal(t, 5, result.GapsReviewed)
	require.Equal(t, 4, result.NewGapsSinceLastRun)
	require.Equal(t, 3, result.TasksCreated)
	require.Equal(t, 1, result.Escalations)
	require.Equal(t, "cp-report-1", result.ReportShard)
	require.Equal(t, now.Format(time.RFC3339), result.LastTriageAt)

	require.Contains(t, strings.Join(runner.calls, "\n"), "cxp shard append cp-escalations")
	require.Contains(t, strings.Join(runner.calls, "\n"), "cxp knowledge create KB Triage Report 2026-04-14")
}

func TestTriageWorkflowEmptyShardStillReports(t *testing.T) {
	runner := &fakeRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			cmdline := name + " " + strings.Join(args, " ")
			switch {
			case strings.HasPrefix(cmdline, "cxp shard show"):
				return []byte(`{"content":""}`), nil
			case strings.HasPrefix(cmdline, "cxp knowledge create KB Triage Report 2026-04-14"):
				return []byte(`{"id":"cp-report-empty"}`), nil
			default:
				return nil, fmt.Errorf("unexpected command: %s", cmdline)
			}
		},
	}

	workflow := NewTriageWorkflow(runner, func() time.Time {
		return time.Date(2026, 4, 14, 5, 0, 0, 0, time.UTC)
	})

	resultJSON, err := workflow.Run(context.Background(), Request{
		Config: json.RawMessage(`{"gaps_shard":"cp-gaps","escalations_shard":"cp-escalations","triage_model":"triage-model"}`),
	})
	require.NoError(t, err)

	var result TriageResult
	require.NoError(t, json.Unmarshal(resultJSON, &result))
	require.Equal(t, 0, result.GapsReviewed)
	require.Equal(t, 0, result.NewGapsSinceLastRun)
	require.Equal(t, 0, result.TasksCreated)
	require.Equal(t, 0, result.Escalations)
	require.Equal(t, "cp-report-empty", result.ReportShard)
}

func TestBuiltinRegistryIncludesTriage(t *testing.T) {
	registry := NewBuiltinRegistry(&fakeRunner{})
	workflow, ok := registry.Get(WorkflowTypeTriage)
	require.True(t, ok)
	require.Equal(t, WorkflowTypeTriage, workflow.Name())
}
