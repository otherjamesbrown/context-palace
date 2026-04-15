package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler"
)

type DriftScanRunner struct{}

func (DriftScanRunner) Name() string { return "drift-scan" }

type DriftScanConfig struct {
	RepoPath    string `json:"repo_path"`
	GapsShard   string `json:"gaps_shard"`
	MaxArticles int    `json:"max_articles,omitempty"`
}

type DriftScanResult struct {
	ArticlesScanned int            `json:"articles_scanned"`
	AnchorsChecked  int            `json:"anchors_checked"`
	AnchorsBroken   int            `json:"anchors_broken"`
	GapsLogged      int            `json:"gaps_logged"`
	BrokenAnchors   []BrokenAnchor `json:"broken_anchors,omitempty"`
}

type BrokenAnchor struct {
	ArticleID string `json:"article_id"`
	Type      string `json:"type"` // "file_path" | "function_name"
	Value     string `json:"value"`
	Reason    string `json:"reason"`
}

func (r DriftScanRunner) Run(ctx context.Context, configRaw json.RawMessage) (string, json.RawMessage, error) {
	var cfg DriftScanConfig
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &cfg); err != nil {
			return "", nil, fmt.Errorf("invalid drift_scan config: %w", err)
		}
	}
	if cfg.RepoPath == "" {
		return "", nil, fmt.Errorf("drift_scan config requires repo_path")
	}
	if cfg.GapsShard == "" {
		return "", nil, fmt.Errorf("drift_scan config requires gaps_shard")
	}

	articles, err := listKBArticles(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("list articles: %w", err)
	}
	if cfg.MaxArticles > 0 && len(articles) > cfg.MaxArticles {
		articles = articles[:cfg.MaxArticles]
	}

	result := DriftScanResult{ArticlesScanned: len(articles)}

	for _, articleID := range articles {
		content, err := driftShardContent(ctx, articleID)
		if err != nil {
			continue
		}
		anchors := extractAnchors(content)
		result.AnchorsChecked += len(anchors)

		for _, anchor := range anchors {
			ok, reason := verifyAnchor(cfg.RepoPath, anchor)
			if !ok {
				result.AnchorsBroken++
				broken := BrokenAnchor{
					ArticleID: articleID,
					Type:      anchor.Type,
					Value:     anchor.Value,
					Reason:    reason,
				}
				result.BrokenAnchors = append(result.BrokenAnchors, broken)
				if err := appendGap(ctx, cfg.GapsShard, broken); err == nil {
					result.GapsLogged++
				}
			}
		}
	}

	summary := fmt.Sprintf("scanned %d articles, %d anchors broken, %d gaps logged",
		result.ArticlesScanned, result.AnchorsBroken, result.GapsLogged)

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return summary, nil, err
	}
	return summary, resultJSON, nil
}

// Anchor types for extraction.
type Anchor struct {
	Type  string // "file_path" | "function_name"
	Value string
}

var (
	filePathRE = regexp.MustCompile("`([a-zA-Z0-9_\\-/]+\\.(go|sql|yaml|yml|md|json|sh))`")
	funcNameRE = regexp.MustCompile("`([A-Z][a-zA-Z0-9_]+)\\(\\)`")
)

func extractAnchors(content string) []Anchor {
	seen := map[string]bool{}
	var anchors []Anchor

	for _, m := range filePathRE.FindAllStringSubmatch(content, -1) {
		key := "file:" + m[1]
		if !seen[key] {
			seen[key] = true
			anchors = append(anchors, Anchor{Type: "file_path", Value: m[1]})
		}
	}
	for _, m := range funcNameRE.FindAllStringSubmatch(content, -1) {
		key := "func:" + m[1]
		if !seen[key] {
			seen[key] = true
			anchors = append(anchors, Anchor{Type: "function_name", Value: m[1]})
		}
	}
	return anchors
}

func verifyAnchor(repoPath string, anchor Anchor) (bool, string) {
	switch anchor.Type {
	case "file_path":
		cmd := exec.Command("git", "-C", repoPath, "ls-files", "--error-unmatch", anchor.Value)
		if err := cmd.Run(); err != nil {
			return false, "file not tracked in repo"
		}
		return true, ""
	case "function_name":
		cmd := exec.Command("git", "-C", repoPath, "grep", "-q", "-E",
			fmt.Sprintf("(func|fn|def)[[:space:]]+%s", regexp.QuoteMeta(anchor.Value)))
		if err := cmd.Run(); err != nil {
			return false, "function name not found in repo"
		}
		return true, ""
	}
	return true, ""
}

func listKBArticles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "cxp", "kb", "list", "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, err
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids, nil
}

func driftShardContent(ctx context.Context, id string) (string, error) {
	cmd := exec.CommandContext(ctx, "cxp", "shard", "show", id, "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var shard struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(out, &shard); err != nil {
		return "", err
	}
	return shard.Content, nil
}

func appendGap(ctx context.Context, gapsShard string, broken BrokenAnchor) error {
	line := fmt.Sprintf("%s | drift-scan | drift-detected | article=%s type=%s value=%s reason=%s",
		time.Now().UTC().Format("2006-01-02"),
		broken.ArticleID, broken.Type, broken.Value, broken.Reason)
	cmd := exec.CommandContext(ctx, "cxp", "shard", "append", gapsShard, "--body", line)
	return cmd.Run()
}

func init() {
	scheduler.DefaultRegistry.Register(DriftScanRunner{})
}
