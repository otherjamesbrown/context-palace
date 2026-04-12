package workflows

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunCanaryAllPass(t *testing.T) {
	restore := stubCanaryDeps(t)
	defer restore()

	loadShardContentFunc = func(ctx context.Context, db *sql.DB, shardID string) (string, error) {
		return `
- question: What is Context Palace?
  expected_facts:
    - knowledge base
- question: How do I search it?
  expected_facts:
    - cxp kb search
`, nil
	}
	runCanaryAgentFunc = func(ctx context.Context, cfg CanaryConfig, question string) (string, error) {
		switch question {
		case "What is Context Palace?":
			return "It is a knowledge base and developer tool.", nil
		case "How do I search it?":
			return "Use cxp kb search for retrieval.", nil
		default:
			return "", nil
		}
	}

	result, err := RunCanary(context.Background(), new(sql.DB), CanaryConfig{
		Project:     "cp",
		CanaryShard: "cp-kb-canaries",
	})

	require.NoError(t, err)
	require.Equal(t, 2, result.QuestionsTested)
	require.Equal(t, 2, result.Passed)
	require.Equal(t, 0, result.Failed)
	require.Empty(t, result.Failures)
}

func TestRunCanaryOneFailureAppendsGap(t *testing.T) {
	restore := stubCanaryDeps(t)
	defer restore()

	var appendedShard string
	var appendedContent string

	loadShardContentFunc = func(ctx context.Context, db *sql.DB, shardID string) (string, error) {
		return `
- question: Where is the KB?
  expected_facts:
    - docs live in shards
    - searchable by cxp kb search
`, nil
	}
	runCanaryAgentFunc = func(ctx context.Context, cfg CanaryConfig, question string) (string, error) {
		return "The docs live in shards.", nil
	}
	appendShardContentFunc = func(ctx context.Context, db *sql.DB, shardID, content string) error {
		appendedShard = shardID
		appendedContent = content
		return nil
	}
	nowFunc = func() time.Time {
		return time.Date(2026, 4, 12, 8, 30, 0, 0, time.UTC)
	}

	result, err := RunCanary(context.Background(), new(sql.DB), CanaryConfig{
		Project:     "cp",
		CanaryShard: "cp-kb-canaries",
		GapsShard:   "cp-kb-gaps",
		RunID:       17,
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.QuestionsTested)
	require.Equal(t, 0, result.Passed)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Failures, 1)
	require.Equal(t, []string{"searchable by cxp kb search"}, result.Failures[0].MissingFacts)
	require.Equal(t, "cp-kb-gaps", appendedShard)
	require.Contains(t, appendedContent, "2026-04-12 | canary-17 | retrieval-failure")
	require.Contains(t, appendedContent, `Q: "Where is the KB?"`)
	require.Contains(t, appendedContent, `"searchable by cxp kb search"`)
	require.Contains(t, appendedContent, `got: "The docs live in shards."`)
}

func TestRunCanaryEmptyQuestionList(t *testing.T) {
	restore := stubCanaryDeps(t)
	defer restore()

	loadShardContentFunc = func(ctx context.Context, db *sql.DB, shardID string) (string, error) {
		return "", nil
	}
	runCalled := false
	runCanaryAgentFunc = func(ctx context.Context, cfg CanaryConfig, question string) (string, error) {
		runCalled = true
		return "", nil
	}

	result, err := RunCanary(context.Background(), new(sql.DB), CanaryConfig{
		Project:     "cp",
		CanaryShard: "cp-kb-canaries",
	})

	require.NoError(t, err)
	require.False(t, runCalled)
	require.Equal(t, CanaryResult{}, result)
}

func stubCanaryDeps(t *testing.T) func() {
	t.Helper()

	prevLoad := loadShardContentFunc
	prevAppend := appendShardContentFunc
	prevRun := runCanaryAgentFunc
	prevNow := nowFunc

	loadShardContentFunc = loadShardContent
	appendShardContentFunc = appendShardContent
	runCanaryAgentFunc = runCanaryAgent
	nowFunc = time.Now

	return func() {
		loadShardContentFunc = prevLoad
		appendShardContentFunc = prevAppend
		runCanaryAgentFunc = prevRun
		nowFunc = prevNow
	}
}
