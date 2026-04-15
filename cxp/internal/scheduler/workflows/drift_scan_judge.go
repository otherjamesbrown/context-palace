package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const semanticJudgePrompt = `You are a technical reviewer for a knowledge base article. Compare the article's current content against its previous version and classify any meaningful drift.

Respond with ONLY a JSON object of the form:
{"verdict": "ok" | "drift_interpretation" | "drift_coverage" | "drift_scope", "detail": "one short sentence"}

Verdicts:
- "ok": no meaningful drift; changes are editorial or accurate updates
- "drift_interpretation": the new wording contradicts the previous meaning, or misstates what the code does
- "drift_coverage": something important present before was removed and should still be covered
- "drift_scope": the article's stated scope no longer matches its contents (either broader or narrower than the title/intro claims)

Only flag drift that would mislead another engineer. Minor rewordings, formatting fixes, and correct updates in response to code changes are "ok".

=== Previous version ===
%s

=== Current version ===
%s`

type judgeResponse struct {
	Verdict string `json:"verdict"`
	Detail  string `json:"detail"`
}

var validJudgeVerdicts = map[string]bool{
	"ok":                   true,
	"drift_interpretation": true,
	"drift_coverage":       true,
	"drift_scope":          true,
}

// runSemanticJudge picks the N articles with the oldest last_semantic_check_at
// metadata (or never-checked) from the given set, loads current+previous content
// via `cxp knowledge diff`, invokes the judge LLM, and records non-ok findings
// to the gaps shard. Updates last_semantic_check_at on each reviewed article.
func runSemanticJudge(ctx context.Context, cfg DriftScanConfig, articles []string, result *DriftScanResult) {
	gen, err := newGeneratorFromModel(cfg.JudgeModel)
	if err != nil {
		log.Printf("drift-scan: judge unavailable (%v); skipping semantic layer", err)
		return
	}

	picked := pickOldestForJudge(ctx, articles, cfg.JudgeArticlesPerRun)
	for _, articleID := range picked {
		prev, curr, err := loadArticleDiff(ctx, articleID)
		if err != nil {
			log.Printf("drift-scan: judge skip %s: %v", articleID, err)
			continue
		}
		if prev == "" {
			// No previous version — nothing meaningful to compare against.
			recordSemanticCheck(ctx, articleID)
			continue
		}

		verdict, detail, err := callJudge(ctx, gen, prev, curr)
		if err != nil {
			log.Printf("drift-scan: judge call failed for %s: %v", articleID, err)
			continue
		}

		result.SemanticChecksRun++
		if verdict != "ok" {
			finding := SemanticFinding{ArticleID: articleID, Category: verdict, Detail: detail}
			result.SemanticFindings = append(result.SemanticFindings, finding)
			if err := appendSemanticGap(ctx, cfg.GapsShard, finding); err == nil {
				result.GapsLogged++
			}
		}
		recordSemanticCheck(ctx, articleID)
	}
}

// pickOldestForJudge returns up to n article IDs ordered by oldest
// last_semantic_check_at metadata first (never-checked articles are oldest).
func pickOldestForJudge(ctx context.Context, articles []string, n int) []string {
	type scored struct {
		id   string
		when time.Time
	}
	items := make([]scored, 0, len(articles))
	for _, id := range articles {
		items = append(items, scored{id: id, when: readLastSemanticCheck(ctx, id)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].when.Before(items[j].when) })
	if n > len(items) {
		n = len(items)
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = items[i].id
	}
	return out
}

// readLastSemanticCheck returns the stored timestamp or zero Time if absent/invalid.
func readLastSemanticCheck(ctx context.Context, articleID string) time.Time {
	cmd := exec.CommandContext(ctx, "cxp", "shard", "metadata", "get", articleID, "last_semantic_check_at", "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}
	}
	var resp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, resp.Value)
	if err != nil {
		return time.Time{}
	}
	return t
}

// recordSemanticCheck writes the current UTC timestamp as last_semantic_check_at.
// Failure is non-fatal — the next run will re-check the article.
func recordSemanticCheck(ctx context.Context, articleID string) {
	ts := time.Now().UTC().Format(time.RFC3339)
	cmd := exec.CommandContext(ctx, "cxp", "shard", "metadata", "set", articleID, "last_semantic_check_at", ts)
	_ = cmd.Run()
}

// loadArticleDiff returns (previousContent, currentContent, error) for the article.
// If the article has no previous version, previousContent is empty.
func loadArticleDiff(ctx context.Context, articleID string) (string, string, error) {
	curr, err := getShardContent(ctx, articleID)
	if err != nil {
		return "", "", fmt.Errorf("load current: %w", err)
	}
	cmd := exec.CommandContext(ctx, "cxp", "knowledge", "diff", articleID, "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		// No previous version or command failed — return current with empty prev.
		return "", curr, nil
	}
	var resp struct {
		Previous string `json:"previous"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", curr, nil
	}
	return resp.Previous, curr, nil
}

// callJudge sends prev+curr to the judge LLM and parses the verdict.
func callJudge(ctx context.Context, gen interface {
	Generate(ctx context.Context, prompt string) (string, error)
}, prev, curr string) (verdict, detail string, err error) {
	prompt := fmt.Sprintf(semanticJudgePrompt, prev, curr)
	resp, err := gen.Generate(ctx, prompt)
	if err != nil {
		return "", "", fmt.Errorf("judge generation: %w", err)
	}
	jsonStr := extractJSONObject(resp)
	var j judgeResponse
	if err := json.Unmarshal([]byte(jsonStr), &j); err != nil {
		return "", "", fmt.Errorf("parse judge response: %w", err)
	}
	if !validJudgeVerdicts[j.Verdict] {
		return "", "", fmt.Errorf("invalid verdict %q", j.Verdict)
	}
	return j.Verdict, j.Detail, nil
}

// extractJSONObject strips markdown code fences and extracts the first JSON object from text.
func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	}
	s = strings.TrimSpace(s)
	if start := strings.Index(s, "{"); start >= 0 {
		if end := strings.LastIndex(s, "}"); end > start {
			return s[start : end+1]
		}
	}
	return s
}

// appendSemanticGap writes a semantic-drift finding to the gaps shard.
func appendSemanticGap(ctx context.Context, gapsShard string, f SemanticFinding) error {
	line := fmt.Sprintf("%s | drift-scan | semantic-drift | article=%s category=%s detail=%s",
		time.Now().UTC().Format("2006-01-02"),
		f.ArticleID, f.Category, f.Detail)
	cmd := exec.CommandContext(ctx, "cxp", "shard", "append", gapsShard, "--body", line)
	return cmd.Run()
}
