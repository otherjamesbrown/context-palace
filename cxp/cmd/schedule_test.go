package cmd

import (
	"testing"
	"time"
)

func TestParseScheduleExpr(t *testing.T) {
	now := time.Date(2026, 4, 12, 17, 0, 0, 0, time.UTC)

	next, err := parseScheduleExpr("0 3 * * *", now)
	if err != nil {
		t.Fatalf("expected cron to parse, got error: %v", err)
	}

	want := time.Date(2026, 4, 13, 3, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("expected next run %s, got %s", want, next)
	}
}

func TestParseScheduleExpr_Invalid(t *testing.T) {
	if _, err := parseScheduleExpr("not a cron", time.Now()); err == nil {
		t.Fatal("expected invalid cron expression to fail")
	}
}

func TestValidateScheduleConfig(t *testing.T) {
	tests := []struct {
		name         string
		workflowType string
		raw          string
		wantErr      bool
	}{
		{
			name:         "drift-scan valid",
			workflowType: "drift-scan",
			raw:          `{"repo_path":"/tmp/repo","factcheck_model":"gemini/gemini-2.0-flash","judge_model":"claude/claude-haiku-4-5","judge_articles_per_run":5}`,
		},
		{
			name:         "canary valid",
			workflowType: "canary",
			raw:          `{"canary_shard":"pf-kb-canaries","agent_model":"gemini/gemini-2.5-flash","agent_tools":["cxp kb search","cxp kb show"]}`,
		},
		{
			name:         "triage valid",
			workflowType: "triage",
			raw:          `{"gaps_shard":"pf-kb-gaps","escalations_shard":"pf-kb-escalations","triage_model":"gemini/gemini-2.5-pro"}`,
		},
		{
			name:         "unknown field",
			workflowType: "triage",
			raw:          `{"unknown":true}`,
			wantErr:      true,
		},
		{
			name:         "wrong type",
			workflowType: "canary",
			raw:          `{"agent_tools":"cxp kb search"}`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateScheduleConfig(tt.workflowType, tt.raw)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}
		})
	}
}
