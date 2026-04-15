package generation

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Generator generates text from a prompt using an LLM.
type Generator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// GenerationConfig holds generation provider configuration.
type GenerationConfig struct {
	Provider  string `yaml:"provider"`    // "google" or "anthropic"
	Model     string `yaml:"model"`       // "gemini-2.0-flash" or "claude-haiku-4-5"
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
	case "anthropic":
		apiKey := os.Getenv(cfg.APIKeyEnv)
		if apiKey == "" {
			return nil, fmt.Errorf("generation API key not found: set %s environment variable", cfg.APIKeyEnv)
		}
		return NewAnthropicGenerator(apiKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported generation provider: %q (supported: google, anthropic)", cfg.Provider)
	}
}

// ModelFamily extracts the family prefix from a "provider/model" string.
// For example: "gemini/gemini-2.0-flash" → "gemini", "claude/claude-haiku-4-5" → "claude".
// If the string has no slash, the whole string is returned.
func ModelFamily(modelString string) string {
	if idx := strings.Index(modelString, "/"); idx >= 0 {
		return modelString[:idx]
	}
	return modelString
}

// ValidateDifferentFamilies returns an error if both model strings belong to the same family.
// Used to enforce that the claim extractor and semantic judge use different model families.
func ValidateDifferentFamilies(model1, model2 string) error {
	f1 := ModelFamily(model1)
	f2 := ModelFamily(model2)
	if f1 == f2 {
		return fmt.Errorf("extractor and judge must use different model families, both are %q", f1)
	}
	return nil
}
