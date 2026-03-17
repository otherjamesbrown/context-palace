package generation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator_NilConfig(t *testing.T) {
	g, err := NewGenerator(nil)
	assert.NoError(t, err)
	assert.Nil(t, g)
}

func TestNewGenerator_UnsupportedProvider(t *testing.T) {
	cfg := &GenerationConfig{Provider: "openai"}
	_, err := NewGenerator(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestNewGenerator_MissingAPIKey(t *testing.T) {
	cfg := &GenerationConfig{
		Provider:  "google",
		APIKeyEnv: "NONEXISTENT_GEN_TEST_VAR_XYZ",
	}
	t.Setenv("NONEXISTENT_GEN_TEST_VAR_XYZ", "")
	_, err := NewGenerator(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found")
}

func TestNewGenerator_GoogleValid(t *testing.T) {
	cfg := &GenerationConfig{
		Provider:  "google",
		Model:     "gemini-2.0-flash",
		APIKeyEnv: "TEST_GEN_API_KEY",
	}
	t.Setenv("TEST_GEN_API_KEY", "dummy-key")
	g, err := NewGenerator(cfg)
	require.NoError(t, err)
	assert.NotNil(t, g)
}
