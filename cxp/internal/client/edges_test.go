package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidEdgeType_ValidTypes(t *testing.T) {
	validTypes := []string{
		"blocked-by", "blocks", "child-of", "discovered-from", "extends",
		"has-artifact", "implements", "parent", "previous-version",
		"references", "relates-to", "replies-to", "triggered-by",
	}
	for _, et := range validTypes {
		assert.True(t, IsValidEdgeType(et), "expected %q to be valid", et)
	}
}

func TestIsValidEdgeType_InvalidTypes(t *testing.T) {
	invalidTypes := []string{
		"", "unknown", "BLOCKED-BY", "blocked_by", "child-of ",
		" blocks", "references\n",
	}
	for _, et := range invalidTypes {
		assert.False(t, IsValidEdgeType(et), "expected %q to be invalid", et)
	}
}
