package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ValidateDocType tests ---

func TestValidateDocType_ValidTypes(t *testing.T) {
	for _, dt := range ValidDocTypes {
		assert.NoError(t, ValidateDocType(dt), "expected %q to be valid", dt)
	}
}

func TestValidateDocType_Invalid(t *testing.T) {
	err := ValidateDocType("blog")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid doc_type 'blog'")
}

func TestValidateDocType_Empty(t *testing.T) {
	err := ValidateDocType("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid doc_type")
}

func TestValidateDocType_CaseSensitive(t *testing.T) {
	err := ValidateDocType("Architecture") // capital A
	assert.Error(t, err)
}

// --- extractPgMessage tests ---

func TestExtractPgMessage_FullError(t *testing.T) {
	msg := extractPgMessage("ERROR: shard not found: pf-123 (SQLSTATE P0001)")
	assert.Equal(t, "shard not found: pf-123", msg)
}

func TestExtractPgMessage_NoPrefix(t *testing.T) {
	msg := extractPgMessage("shard not found: pf-123 (SQLSTATE P0001)")
	assert.Equal(t, "shard not found: pf-123", msg)
}

func TestExtractPgMessage_NoSuffix(t *testing.T) {
	msg := extractPgMessage("ERROR: shard not found: pf-123")
	assert.Equal(t, "shard not found: pf-123", msg)
}

func TestExtractPgMessage_PlainMessage(t *testing.T) {
	msg := extractPgMessage("just an error")
	assert.Equal(t, "just an error", msg)
}

func TestExtractPgMessage_Empty(t *testing.T) {
	msg := extractPgMessage("")
	assert.Equal(t, "", msg)
}

func TestExtractPgMessage_OnlyPrefix(t *testing.T) {
	msg := extractPgMessage("ERROR: ")
	assert.Equal(t, "", msg)
}

func TestExtractPgMessage_MultipleSQLSTATE(t *testing.T) {
	// Should strip the last (SQLSTATE ...) occurrence
	msg := extractPgMessage("ERROR: something (SQLSTATE 23505) extra (SQLSTATE P0001)")
	assert.Equal(t, "something (SQLSTATE 23505) extra", msg)
}
