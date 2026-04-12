package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeDriftScanDeps struct {
	articles    []driftArticle
	gapsShard   string
	verify      func(context.Context, DriftScanConfig, driftClaim) (bool, error)
	appendLines []string
	history     map[string]time.Time
	diffs       map[string]string
	judge       func(context.Context, DriftScanConfig, string) (driftJudgeVerdict, error)
	nowValue    time.Time
}

func (f *fakeDriftScanDeps) listOpenArticles(context.Context, string) ([]driftArticle, error) {
	return f.articles, nil
}

func (f *fakeDriftScanDeps) resolveGapsShard(context.Context, DriftScanConfig) (string, error) {
	return f.gapsShard, nil
}

func (f *fakeDriftScanDeps) verifyClaim(ctx context.Context, cfg DriftScanConfig, claim driftClaim) (bool, error) {
	if f.verify == nil {
		return true, nil
	}
	return f.verify(ctx, cfg, claim)
}

func (f *fakeDriftScanDeps) appendGap(_ context.Context, _ string, line string) error {
	f.appendLines = append(f.appendLines, line)
	return nil
}

func (f *fakeDriftScanDeps) lastSemanticChecks(context.Context, string) (map[string]time.Time, error) {
	if f.history == nil {
		return map[string]time.Time{}, nil
	}
	return f.history, nil
}

func (f *fakeDriftScanDeps) articleDiff(_ context.Context, articleID string) (string, error) {
	return f.diffs[articleID], nil
}

func (f *fakeDriftScanDeps) judgeDiff(ctx context.Context, cfg DriftScanConfig, diff string) (driftJudgeVerdict, error) {
	if f.judge == nil {
		return driftJudgeVerdict{}, nil
	}
	return f.judge(ctx, cfg, diff)
}

func (f *fakeDriftScanDeps) now() time.Time {
	if !f.nowValue.IsZero() {
		return f.nowValue
	}
	return time.Date(2026, 4, 12, 3, 0, 0, 0, time.UTC)
}

func TestRunDriftScanZeroArticles(t *testing.T) {
	deps := &fakeDriftScanDeps{
		gapsShard: "cp-kb-gaps",
		nowValue:  time.Date(2026, 4, 12, 3, 0, 0, 0, time.UTC),
	}

	result, err := runDriftScan(context.Background(), deps, DriftScanConfig{
		Project:  "cp",
		RepoPath: t.TempDir(),
	})
	require.NoError(t, err)
	require.Equal(t, DriftScanResult{}, result.DriftScanResult)
	require.Empty(t, deps.appendLines)
}

func TestRunDriftScanBrokenFileClaim(t *testing.T) {
	deps := &fakeDriftScanDeps{
		gapsShard: "cp-kb-gaps",
		articles: []driftArticle{
			{
				ID:      "cp-article-1",
				Content: "file_path: missing/file.go",
				Version: 1,
			},
		},
		verify: func(_ context.Context, _ DriftScanConfig, claim driftClaim) (bool, error) {
			return claim.Value != "missing/file.go", nil
		},
		nowValue: time.Date(2026, 4, 12, 3, 0, 0, 0, time.UTC),
	}

	result, err := runDriftScan(context.Background(), deps, DriftScanConfig{
		Project:  "cp",
		RepoPath: t.TempDir(),
		RunID:    42,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.ArticlesScanned)
	require.Equal(t, 1, result.ClaimsChecked)
	require.Equal(t, 1, result.ClaimsBroken)
	require.Equal(t, 1, result.GapsLogged)
	require.Len(t, deps.appendLines, 1)
	require.Contains(t, deps.appendLines[0], "2026-04-12 | drift-scan-42 | drift-detected | Missing file path claim missing/file.go (cp-article-1)")
}

func TestRunDriftScanPassingFileClaim(t *testing.T) {
	deps := &fakeDriftScanDeps{
		gapsShard: "cp-kb-gaps",
		articles: []driftArticle{
			{
				ID:      "cp-article-2",
				Content: "file_path: internal/workflows/drift_scan.go",
				Version: 1,
			},
		},
		verify: func(_ context.Context, _ DriftScanConfig, claim driftClaim) (bool, error) {
			return claim.Value == "internal/workflows/drift_scan.go", nil
		},
		nowValue: time.Date(2026, 4, 12, 3, 0, 0, 0, time.UTC),
	}

	result, err := runDriftScan(context.Background(), deps, DriftScanConfig{
		Project:  "cp",
		RepoPath: t.TempDir(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.ArticlesScanned)
	require.Equal(t, 1, result.ClaimsChecked)
	require.Zero(t, result.ClaimsBroken)
	require.Zero(t, result.GapsLogged)
	require.Empty(t, deps.appendLines)
}

func TestRunDriftScanSemanticSelectionUsesOldestHistory(t *testing.T) {
	deps := &fakeDriftScanDeps{
		gapsShard: "cp-kb-gaps",
		articles: []driftArticle{
			{ID: "article-c", Version: 2},
			{ID: "article-a", Version: 2},
			{ID: "article-b", Version: 2},
		},
		history: map[string]time.Time{
			"article-a": time.Date(2026, 4, 10, 3, 0, 0, 0, time.UTC),
			"article-b": time.Date(2026, 4, 11, 3, 0, 0, 0, time.UTC),
		},
		diffs: map[string]string{
			"article-c": "",
			"article-a": "",
			"article-b": "",
		},
		nowValue: time.Date(2026, 4, 12, 3, 0, 0, 0, time.UTC),
	}

	result, err := runDriftScan(context.Background(), deps, DriftScanConfig{
		Project:             "cp",
		RepoPath:            t.TempDir(),
		JudgeArticlesPerRun: 2,
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.SemanticChecks)
	require.Contains(t, result.LastSemanticCheck, "article-c")
	require.Contains(t, result.LastSemanticCheck, "article-a")
	require.NotContains(t, result.LastSemanticCheck, "article-b")
}
