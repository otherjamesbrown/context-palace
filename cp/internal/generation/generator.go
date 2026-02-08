package generation

import (
	"context"
	"fmt"
	"os"
)

// Generator generates text from a prompt using an LLM.
type Generator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// GenerationConfig holds generation provider configuration.
type GenerationConfig struct {
	Provider  string `yaml:"provider"`    // "google"
	Model     string `yaml:"model"`       // "gemini-2.0-flash"
	APIKeyEnv string `yaml:"api_key_env"` // env var name containing API key
}

// NewGenerator creates a Generator from config.
func NewGenerator(cfg *GenerationConfig) (Generator, error) {
	if cfg == nil {
		return nil, nil
	}

	switch cfg.Provider {
	case "google":
		apiKey := os.Getenv(cfg.APIKeyEnv)
		if apiKey == "" {
			return nil, fmt.Errorf("generation API key not found: set %s environment variable", cfg.APIKeyEnv)
		}
		return NewGoogleGenerator(apiKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported generation provider: %q (supported: google)", cfg.Provider)
	}
}
