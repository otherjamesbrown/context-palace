package workflows

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
)

type DriftScanConfig struct {
	RepoPath            string `json:"repo_path"`
	FactcheckModel      string `json:"factcheck_model"`
	JudgeModel          string `json:"judge_model"`
	JudgeArticlesPerRun int    `json:"judge_articles_per_run"`
	GapsShard           string `json:"gaps_shard,omitempty"`
	JudgeCLI            string `json:"judge_cli,omitempty"`
}

type driftClaimKind string

const (
	driftClaimFileExists driftClaimKind = "file_exists"
	driftClaimContains   driftClaimKind = "contains"
)

type driftClaim struct {
	Kind        driftClaimKind
	Path        string
	Pattern     string
	Description string
}

type driftSemanticArticle struct {
	ID        string
	Title     string
	Version   int
	UpdatedAt time.Time
	CheckedAt time.Time
}

type driftJudgeResponse struct {
	Issues []string `json:"issues"`
}

type driftCommandRunner func(ctx context.Context, dir, name string, args []string, stdin string) (string, error)

var (
	driftContainsBacktickRe = regexp.MustCompile("`([^`]+)`\\s+(?:contains|includes|mentions|defines|declares|references|uses)\\s+`([^`]+)`")
	driftContainsQuotedRe   = regexp.MustCompile("`([^`]+)`\\s+(?:contains|includes|mentions|defines|declares|references|uses)\\s+\"([^\"]+)\"")
	driftPathRe             = regexp.MustCompile("`([^`]+(?:/[^`]+|\\.[A-Za-z0-9_-]+))`")
)

func RunDriftScan(ctx context.Context, cp *client.Client, project string, config json.RawMessage) (WorkflowResult, error) {
	return runDriftScan(ctx, cp, project, config, defaultDriftCommandRunner, time.Now)
}

func runDriftScan(
	ctx context.Context,
	cp *client.Client,
	project string,
	config json.RawMessage,
	runCommand driftCommandRunner,
	now func() time.Time,
) (WorkflowResult, error) {
	cfg := DriftScanConfig{
		JudgeArticlesPerRun: 5,
		JudgeCLI:            "claude",
	}
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return WorkflowResult{}, fmt.Errorf("parse drift-scan config: %w", err)
		}
	}
	if cfg.RepoPath == "" {
		return WorkflowResult{}, fmt.Errorf("drift-scan requires config.repo_path")
	}

	workflowClient := cloneClientForProject(cp, project)
	docs, err := workflowClient.ListKnowledgeDocs(ctx, "", "", 10000)
	if err != nil {
		return WorkflowResult{}, fmt.Errorf("list knowledge docs: %w", err)
	}

	result := map[string]any{
		"articles_scanned":     0,
		"claims_checked":       0,
		"claims_broken":        0,
		"semantic_checks":      0,
		"semantic_issues":      0,
		"gaps_logged":          0,
		"last_semantic_check":  map[string]string{},
		"semantic_issue_count": 0,
	}
	if len(docs) == 0 {
		return WorkflowResult{
			Summary: "Drift scan completed: no open knowledge articles",
			Result:  result,
		}, nil
	}

	gapsShardID, err := ensureGapsShard(ctx, workflowClient, cfg.GapsShard)
	if err != nil {
		return WorkflowResult{}, err
	}

	runTag := fmt.Sprintf("drift-scan-%d", now().UTC().Unix())
	dateStamp := now().UTC().Format("2006-01-02")

	claimsChecked := 0
	claimsBroken := 0
	gapsLogged := 0
	var gapEntries []string

	for _, doc := range docs {
		fullDoc, err := workflowClient.ShowKnowledgeDoc(ctx, doc.ID)
		if err != nil {
			return WorkflowResult{}, fmt.Errorf("load knowledge doc %s: %w", doc.ID, err)
		}

		claims := extractDriftClaims(fullDoc.Content)
		for _, claim := range claims {
			claimsChecked++
			ok, err := verifyDriftClaim(ctx, cfg.RepoPath, claim, runCommand)
			if err != nil {
				return WorkflowResult{}, fmt.Errorf("verify claim for %s: %w", fullDoc.ID, err)
			}
			if ok {
				continue
			}
			claimsBroken++
			gapsLogged++
			gapEntries = append(gapEntries, fmt.Sprintf("%s | %s | drift-detected | %s: %s", dateStamp, runTag, fullDoc.ID, claim.Description))
		}
	}

	semanticChecks := 0
	semanticIssues := 0
	lastSemanticCheck := mergeSemanticCheckState(ctx, workflowClient, now)
	semanticCandidates := selectArticlesForSemanticCheck(docs, lastSemanticCheck, cfg.JudgeArticlesPerRun)
	for _, article := range semanticCandidates {
		if article.Version <= 1 {
			lastSemanticCheck[article.ID] = now()
			continue
		}

		diffText, err := workflowClient.DiffVersions(ctx, article.ID, article.Version-1, article.Version)
		if err != nil {
			return WorkflowResult{}, fmt.Errorf("diff article %s: %w", article.ID, err)
		}
		if strings.TrimSpace(diffText) == "" {
			lastSemanticCheck[article.ID] = now()
			continue
		}

		issues, err := runKBJudge(ctx, cfg, article.ID, article.Title, diffText, runCommand)
		if err != nil {
			return WorkflowResult{}, fmt.Errorf("judge article %s: %w", article.ID, err)
		}
		semanticChecks++
		lastSemanticCheck[article.ID] = now()

		for _, issue := range issues {
			semanticIssues++
			gapsLogged++
			gapEntries = append(gapEntries, fmt.Sprintf("%s | %s | semantic-drift | %s: %s", dateStamp, runTag, article.ID, issue))
		}
	}

	if len(gapEntries) > 0 {
		if err := workflowClient.AppendShardContent(ctx, gapsShardID, strings.Join(gapEntries, "\n")); err != nil {
			return WorkflowResult{}, fmt.Errorf("append drift scan gaps: %w", err)
		}
	}

	result["articles_scanned"] = len(docs)
	result["claims_checked"] = claimsChecked
	result["claims_broken"] = claimsBroken
	result["semantic_checks"] = semanticChecks
	result["semantic_issues"] = semanticIssues
	result["gaps_logged"] = gapsLogged
	result["last_semantic_check"] = serializeSemanticCheckState(lastSemanticCheck)
	delete(result, "semantic_issue_count")

	summary := fmt.Sprintf(
		"Drift scan completed: %d articles, %d claims checked, %d broken, %d semantic checks, %d semantic issues",
		len(docs), claimsChecked, claimsBroken, semanticChecks, semanticIssues,
	)

	return WorkflowResult{Summary: summary, Result: result}, nil
}

func cloneClientForProject(cp *client.Client, project string) *client.Client {
	cfgCopy := *cp.Config
	cfgCopy.Project = project
	cloned := client.NewClient(&cfgCopy)
	cloned.EmbedProvider = cp.EmbedProvider
	cloned.Generator = cp.Generator
	return cloned
}

func ensureGapsShard(ctx context.Context, cp *client.Client, configuredID string) (string, error) {
	gapsID := configuredID
	if gapsID == "" {
		gapsID = fmt.Sprintf("%s-kb-gaps", cp.Config.Project)
	}

	if _, err := cp.GetShard(ctx, gapsID); err == nil {
		return gapsID, nil
	}

	title := fmt.Sprintf("%s KB Gaps", strings.ToUpper(cp.Config.Project))
	_, err := cp.CreateKnowledgeDocWithID(ctx, gapsID, title, "# KB Gaps\n", "reference", []string{"kb", "gaps"})
	if err != nil {
		return "", fmt.Errorf("ensure gaps shard %s: %w", gapsID, err)
	}
	return gapsID, nil
}

func extractDriftClaims(content string) []driftClaim {
	lines := strings.Split(content, "\n")
	seen := map[string]struct{}{}
	var claims []driftClaim

	addClaim := func(claim driftClaim) {
		key := string(claim.Kind) + "|" + claim.Path + "|" + claim.Pattern
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		claims = append(claims, claim)
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		for _, match := range driftContainsBacktickRe.FindAllStringSubmatch(line, -1) {
			addClaim(driftClaim{
				Kind:        driftClaimContains,
				Path:        match[1],
				Pattern:     match[2],
				Description: fmt.Sprintf("`%s` should contain `%s`", match[1], match[2]),
			})
		}
		for _, match := range driftContainsQuotedRe.FindAllStringSubmatch(line, -1) {
			addClaim(driftClaim{
				Kind:        driftClaimContains,
				Path:        match[1],
				Pattern:     match[2],
				Description: fmt.Sprintf("`%s` should contain \"%s\"", match[1], match[2]),
			})
		}

		if !looksLikeFileExistenceClaim(line) {
			continue
		}
		for _, match := range driftPathRe.FindAllStringSubmatch(line, -1) {
			addClaim(driftClaim{
				Kind:        driftClaimFileExists,
				Path:        match[1],
				Description: fmt.Sprintf("`%s` should exist", match[1]),
			})
		}
	}

	return claims
}

func looksLikeFileExistenceClaim(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, " contains ") || strings.Contains(lower, " includes ") {
		return false
	}

	keywords := []string{
		" file ", " path ", " exists", " present", " located", " lives in", " under ", " at `",
	}
	for _, keyword := range keywords {
		if strings.Contains(" "+lower+" ", keyword) {
			return true
		}
	}
	return strings.Contains(lower, "file `") || strings.Contains(lower, "path `")
}

func verifyDriftClaim(ctx context.Context, repoPath string, claim driftClaim, runCommand driftCommandRunner) (bool, error) {
	switch claim.Kind {
	case driftClaimFileExists:
		return fileExists(ctx, repoPath, claim.Path, runCommand)
	case driftClaimContains:
		return fileContains(repoPath, claim.Path, claim.Pattern)
	default:
		return false, fmt.Errorf("unsupported claim kind: %s", claim.Kind)
	}
}

func fileExists(ctx context.Context, repoPath, path string, runCommand driftCommandRunner) (bool, error) {
	if output, err := runCommand(ctx, repoPath, "git", []string{"-C", repoPath, "ls-files", "--error-unmatch", path}, ""); err == nil && strings.TrimSpace(output) != "" {
		return true, nil
	}

	fullPath := filepath.Join(repoPath, filepath.Clean(path))
	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		return true, nil
	}
	return false, nil
}

func fileContains(repoPath, path, pattern string) (bool, error) {
	fullPath := filepath.Join(repoPath, filepath.Clean(path))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	return strings.Contains(string(data), pattern), nil
}

func mergeSemanticCheckState(ctx context.Context, cp *client.Client, now func() time.Time) map[string]time.Time {
	runs, err := cp.ListRecentScheduleRunsByWorkflow(ctx, "drift-scan", 200)
	if err != nil {
		return map[string]time.Time{}
	}

	state := map[string]time.Time{}
	for i := len(runs) - 1; i >= 0; i-- {
		run := runs[i]
		var payload map[string]any
		if err := json.Unmarshal(run.Result, &payload); err != nil {
			continue
		}
		raw, ok := payload["last_semantic_check"].(map[string]any)
		if !ok {
			continue
		}
		for articleID, value := range raw {
			ts, ok := value.(string)
			if !ok {
				continue
			}
			parsed, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				continue
			}
			state[articleID] = parsed
		}
	}
	return state
}

func selectArticlesForSemanticCheck(docs []client.KnowledgeDoc, lastChecked map[string]time.Time, limit int) []driftSemanticArticle {
	if limit <= 0 {
		return nil
	}

	candidates := make([]driftSemanticArticle, 0, len(docs))
	for _, doc := range docs {
		candidates = append(candidates, driftSemanticArticle{
			ID:        doc.ID,
			Title:     doc.Title,
			Version:   doc.Version,
			UpdatedAt: doc.UpdatedAt,
			CheckedAt: lastChecked[doc.ID],
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.CheckedAt.IsZero() {
			if right.CheckedAt.IsZero() {
				return left.UpdatedAt.Before(right.UpdatedAt)
			}
			return true
		}
		if right.CheckedAt.IsZero() {
			return false
		}
		if left.CheckedAt.Equal(right.CheckedAt) {
			return left.UpdatedAt.Before(right.UpdatedAt)
		}
		return left.CheckedAt.Before(right.CheckedAt)
	})

	if limit > len(candidates) {
		limit = len(candidates)
	}
	return candidates[:limit]
}

func runKBJudge(ctx context.Context, cfg DriftScanConfig, articleID, title, diffText string, runCommand driftCommandRunner) ([]string, error) {
	prompt := buildJudgePrompt(articleID, title, diffText)
	args := []string{"-p"}
	if cfg.JudgeModel != "" {
		args = append(args, "--model", cfg.JudgeModel)
	}
	args = append(args, prompt)

	output, err := runCommand(ctx, "", cfg.JudgeCLI, args, "")
	if err != nil {
		return nil, err
	}
	return parseJudgeOutput(output), nil
}

func buildJudgePrompt(articleID, title, diffText string) string {
	return strings.TrimSpace(fmt.Sprintf(`
You are running the kb-judge skill for a daily drift scan.
Review this knowledge article diff and decide whether it introduced stale or misleading documentation.

Article: %s
Title: %s

Return JSON only in this shape:
{"issues":["short issue description", "..."]}

If there is no problem, return {"issues":[]}.

Diff:
%s
`, articleID, title, diffText))
}

func parseJudgeOutput(output string) []string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		var response driftJudgeResponse
		if err := json.Unmarshal([]byte(trimmed[start:end+1]), &response); err == nil {
			return compactIssues(response.Issues)
		}
	}

	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, `"issues":[]`) || strings.Contains(lower, "no issues") || strings.Contains(lower, "no drift") {
		return nil
	}
	return []string{trimmed}
}

func compactIssues(issues []string) []string {
	var cleaned []string
	for _, issue := range issues {
		issue = strings.TrimSpace(issue)
		if issue == "" {
			continue
		}
		cleaned = append(cleaned, issue)
	}
	return cleaned
}

func serializeSemanticCheckState(state map[string]time.Time) map[string]string {
	out := make(map[string]string, len(state))
	for articleID, checkedAt := range state {
		if checkedAt.IsZero() {
			continue
		}
		out[articleID] = checkedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func defaultDriftCommandRunner(ctx context.Context, dir, name string, args []string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
