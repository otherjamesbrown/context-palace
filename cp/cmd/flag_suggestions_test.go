package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractUnknownFlag(t *testing.T) {
	tests := []struct {
		msg      string
		expected string
	}{
		{"unknown flag: --status", "status"},
		{"unknown flag: --append", "append"},
		{"unknown flag: --content", "content"},
		{"some other error", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractUnknownFlag(tt.msg)
		assert.Equal(t, tt.expected, got, "extractUnknownFlag(%q)", tt.msg)
	}
}

func TestBuildCmdPath(t *testing.T) {
	// buildCmdPath constructs from cobra command hierarchy
	// We test with the actual shard update command
	path := buildCmdPath(shardUpdateCmd)
	assert.Equal(t, "shard update", path)

	path = buildCmdPath(shardLinkCmd)
	assert.Equal(t, "shard link", path)

	path = buildCmdPath(shardCloseCmd)
	assert.Equal(t, "shard close", path)
}
