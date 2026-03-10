package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidStatus(t *testing.T) {
	valid := []string{"open", "ready", "in_progress", "needs-review", "closed"}
	for _, s := range valid {
		assert.True(t, IsValidStatus(s), "expected %q to be valid", s)
	}

	invalid := []string{"", "pending", "done", "review", "blocked"}
	for _, s := range invalid {
		assert.False(t, IsValidStatus(s), "expected %q to be invalid", s)
	}
}

func TestValidTransitionsFrom(t *testing.T) {
	tests := []struct {
		from     string
		expected []string
	}{
		{"open", []string{"ready", "in_progress", "closed"}},
		{"ready", []string{"open", "in_progress", "closed"}},
		{"in_progress", []string{"open", "needs-review", "closed"}},
		{"needs-review", []string{"open", "in_progress", "closed"}},
		{"closed", []string{"open"}},
		{"invalid", nil},
	}

	for _, tt := range tests {
		got := ValidTransitionsFrom(tt.from)
		assert.Equal(t, tt.expected, got, "ValidTransitionsFrom(%q)", tt.from)
	}
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from, to string
		valid    bool
	}{
		// Forward flow
		{"open", "ready", true},
		{"ready", "in_progress", true},
		{"in_progress", "needs-review", true},
		{"needs-review", "closed", true},

		// Reopen from any status
		{"closed", "open", true},
		{"in_progress", "open", true},
		{"needs-review", "open", true},
		{"ready", "open", true},

		// Skip steps (allowed)
		{"open", "in_progress", true},
		{"open", "closed", true},
		{"ready", "closed", true},
		{"in_progress", "closed", true},

		// Backward (not allowed, except reopen)
		{"closed", "ready", false},
		{"closed", "in_progress", false},
		{"closed", "needs-review", false},
		{"needs-review", "ready", false},
		{"in_progress", "ready", false},

		// Same status
		{"open", "open", false},
		{"closed", "closed", false},
	}

	for _, tt := range tests {
		got := IsValidTransition(tt.from, tt.to)
		assert.Equal(t, tt.valid, got, "%s → %s", tt.from, tt.to)
	}
}
