package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify_Basic(t *testing.T) {
	assert.Equal(t, "schema-migration", Slugify("Schema Migration", 50))
}

func TestSlugify_SpecialChars(t *testing.T) {
	assert.Equal(t, "fix-auth-bug-in-oauth", Slugify("Fix auth bug in OAuth!", 50))
}

func TestSlugify_Truncation(t *testing.T) {
	slug := Slugify("This is a very long title that should be truncated to fit within the limit", 20)
	assert.LessOrEqual(t, len(slug), 20)
	assert.Equal(t, "this-is-a-very-long", slug)
}

func TestSlugify_TruncateTrimsTrailingHyphen(t *testing.T) {
	// "add new feature" -> "add-new-feature" (15 chars)
	// truncated to 8: "add-new-" -> trim -> "add-new"
	slug := Slugify("add new feature", 8)
	assert.Equal(t, "add-new", slug)
}

func TestSlugify_Empty(t *testing.T) {
	assert.Equal(t, "", Slugify("", 50))
}

func TestSlugify_OnlySpecialChars(t *testing.T) {
	assert.Equal(t, "", Slugify("!@#$%", 50))
}

func TestSlugify_Numbers(t *testing.T) {
	assert.Equal(t, "v2-release", Slugify("v2 Release", 50))
}

func TestWorktreeBranch_Normal(t *testing.T) {
	branch := WorktreeBranch("pf-abc123", "Schema Migration")
	assert.Equal(t, "pf-abc123/schema-migration", branch)
}

func TestWorktreeBranch_EmptyTitle(t *testing.T) {
	branch := WorktreeBranch("pf-abc123", "")
	assert.Equal(t, "pf-abc123", branch)
}

func TestWorktreeBranch_LongTitle(t *testing.T) {
	branch := WorktreeBranch("pf-abc123", "This is a very long title that exceeds fifty characters in length easily")
	// Slug should be at most 50 chars
	parts := splitBranch(branch)
	assert.Equal(t, "pf-abc123", parts[0])
	assert.LessOrEqual(t, len(parts[1]), 50)
}

func TestWorktreePath_Format(t *testing.T) {
	path, err := WorktreePath("context-palace", "pf-abc123")
	assert.NoError(t, err)
	assert.Contains(t, path, "worktrees/context-palace/pf-abc123")
}

// splitBranch splits "taskid/slug" into parts
func splitBranch(s string) [2]string {
	for i, c := range s {
		if c == '/' {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}
