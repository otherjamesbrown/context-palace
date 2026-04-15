package workflows

import (
	"context"
	"errors"
	"testing"
)

type stubGen struct {
	resp string
	err  error
}

func (s stubGen) Generate(_ context.Context, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.resp, nil
}

func TestExtractJSONObject(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare", `{"verdict":"ok","detail":"looks good"}`, `{"verdict":"ok","detail":"looks good"}`},
		{"json fence", "```json\n{\"verdict\":\"ok\"}\n```", `{"verdict":"ok"}`},
		{"plain fence", "```\n{\"verdict\":\"ok\"}\n```", `{"verdict":"ok"}`},
		{"preamble+object", "Here is the verdict:\n{\"verdict\":\"drift_scope\",\"detail\":\"x\"}\nThanks!", `{"verdict":"drift_scope","detail":"x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSONObject(tc.in)
			if got != tc.want {
				t.Fatalf("extractJSONObject(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCallJudgeValidVerdicts(t *testing.T) {
	cases := map[string]string{
		"ok":                   `{"verdict":"ok","detail":"no drift"}`,
		"drift_interpretation": `{"verdict":"drift_interpretation","detail":"contradicts prior meaning"}`,
		"drift_coverage":       `{"verdict":"drift_coverage","detail":"removed retry section"}`,
		"drift_scope":          `{"verdict":"drift_scope","detail":"now includes deployment"}`,
	}
	for want, resp := range cases {
		t.Run(want, func(t *testing.T) {
			verdict, detail, err := callJudge(context.Background(), stubGen{resp: resp}, "old", "new")
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if verdict != want {
				t.Fatalf("verdict = %q, want %q", verdict, want)
			}
			if detail == "" {
				t.Fatal("detail should be non-empty")
			}
		})
	}
}

func TestCallJudgeRejectsUnknownVerdict(t *testing.T) {
	_, _, err := callJudge(context.Background(), stubGen{resp: `{"verdict":"needs-fix","detail":"x"}`}, "old", "new")
	if err == nil {
		t.Fatal("expected error for unknown verdict")
	}
}

func TestCallJudgeWrapsGeneratorError(t *testing.T) {
	_, _, err := callJudge(context.Background(), stubGen{err: errors.New("upstream down")}, "old", "new")
	if err == nil {
		t.Fatal("expected error from generator")
	}
}

func TestRunSemanticJudgeUnavailableGenerator(t *testing.T) {
	// Invalid model string → newGeneratorFromModel returns error → runSemanticJudge logs and returns.
	result := DriftScanResult{}
	cfg := DriftScanConfig{JudgeModel: "notavalidmodel", JudgeArticlesPerRun: 1}
	runSemanticJudge(context.Background(), cfg, []string{"cp-x"}, &result)
	if result.SemanticChecksRun != 0 {
		t.Fatalf("expected no semantic checks when judge unavailable, got %d", result.SemanticChecksRun)
	}
}
