package workflows

import (
	"testing"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/stretchr/testify/assert"
)

func TestExtractDriftClaims(t *testing.T) {
	content := `
The file ` + "`cmd/root.go`" + ` exists and is the CLI entrypoint.
` + "`internal/app/config.go`" + ` contains ` + "`type Config struct`" + `.
` + "`README.md`" + ` contains "Context Palace".
`

	claims := extractDriftClaims(content)
	if assert.Len(t, claims, 3) {
		assert.Equal(t, driftClaimFileExists, claims[0].Kind)
		assert.Equal(t, "cmd/root.go", claims[0].Path)
		assert.Equal(t, driftClaimContains, claims[1].Kind)
		assert.Equal(t, "type Config struct", claims[1].Pattern)
		assert.Equal(t, "Context Palace", claims[2].Pattern)
	}
}

func TestSelectArticlesForSemanticCheck(t *testing.T) {
	now := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	docs := []client.KnowledgeDoc{
		{ID: "a", Title: "A", Version: 2},
		{ID: "b", Title: "B", Version: 3},
		{ID: "c", Title: "C", Version: 1},
	}

	lastChecked := map[string]time.Time{
		"b": now.Add(-2 * time.Hour),
		"c": now.Add(-24 * time.Hour),
	}

	selected := selectArticlesForSemanticCheck(docs, lastChecked, 2)
	if assert.Len(t, selected, 2) {
		assert.Equal(t, "a", selected[0].ID)
		assert.Equal(t, "c", selected[1].ID)
	}
}

func TestParseJudgeOutput(t *testing.T) {
	assert.Nil(t, parseJudgeOutput(`{"issues":[]}`))
	assert.Equal(t, []string{"Missing mention of the new retry limit."}, parseJudgeOutput(`{"issues":["Missing mention of the new retry limit."]}`))
	assert.Equal(t, []string{"freeform failure"}, parseJudgeOutput("freeform failure"))
}
