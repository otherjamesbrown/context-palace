package embedding

import (
	"fmt"
	"os"
)

// EmbeddingConfig holds embedding provider configuration.
type EmbeddingConfig struct {
	Provider  string `yaml:"provider"`   // "google"
	Model     string `yaml:"model"`      // "text-embedding-004"
	APIKeyEnv string `yaml:"api_key_env"` // env var name containing API key
}

// NewProvider creates an embedding Provider from config.
// Returns (nil, nil) when config is nil (graceful degradation).
func NewProvider(cfg *EmbeddingConfig) (Provider, error) {
	if cfg == nil {
		return nil, nil
	}

	switch cfg.Provider {
	case "google":
		apiKey := os.Getenv(cfg.APIKeyEnv)
		if apiKey == "" {
			return nil, fmt.Errorf("embedding API key not found: set %s environment variable", cfg.APIKeyEnv)
		}
		return NewGoogleProvider(apiKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %q (supported: google)", cfg.Provider)
	}
}
