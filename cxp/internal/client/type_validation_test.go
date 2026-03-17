package client

import (
	"strings"
	"testing"
)

func TestValidateShardType_ValidTypes(t *testing.T) {
	for _, typ := range ValidShardTypes {
		if err := ValidateShardType(typ); err != nil {
			t.Errorf("ValidateShardType(%q) returned error: %v", typ, err)
		}
	}
}

func TestValidateShardType_RetiredTypes(t *testing.T) {
	tests := []struct {
		retired     string
		replacement string
	}{
		{"epic", "design"},
		{"spec", "design"},
		{"feature", "design"},
		{"reference", "knowledge"},
		{"report", "knowledge"},
		{"journal", "knowledge"},
	}
	for _, tt := range tests {
		err := ValidateShardType(tt.retired)
		if err == nil {
			t.Errorf("ValidateShardType(%q) should return error", tt.retired)
			continue
		}
		if !strings.Contains(err.Error(), "retired") {
			t.Errorf("ValidateShardType(%q) error should mention 'retired', got: %v", tt.retired, err)
		}
		if !strings.Contains(err.Error(), tt.replacement) {
			t.Errorf("ValidateShardType(%q) error should suggest %q, got: %v", tt.retired, tt.replacement, err)
		}
	}
}

func TestValidateShardType_UnknownType(t *testing.T) {
	err := ValidateShardType("foobar")
	if err == nil {
		t.Error("ValidateShardType(\"foobar\") should return error")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("error should mention 'unknown type', got: %v", err)
	}
}
