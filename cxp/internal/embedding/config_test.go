package embedding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider_NilConfig(t *testing.T) {
	p, err := NewProvider(nil)
	assert.NoError(t, err)
	assert.Nil(t, p)
}

func TestNewProvider_UnsupportedProvider(t *testing.T) {
	cfg := &EmbeddingConfig{Provider: "openai"}
	_, err := NewProvider(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestNewProvider_MissingAPIKey(t *testing.T) {
	cfg := &EmbeddingConfig{
		Provider:  "google",
		APIKeyEnv: "NONEXISTENT_EMBEDDING_TEST_VAR_XYZ",
	}
	t.Setenv("NONEXISTENT_EMBEDDING_TEST_VAR_XYZ", "")
	_, err := NewProvider(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found")
}

func TestNewProvider_GoogleValid(t *testing.T) {
	cfg := &EmbeddingConfig{
		Provider:  "google",
		Model:     "gemini-embedding-001",
		APIKeyEnv: "TEST_EMBED_API_KEY",
	}
	t.Setenv("TEST_EMBED_API_KEY", "dummy-key")
	p, err := NewProvider(cfg)
	require.NoError(t, err)
	assert.NotNil(t, p)
}
