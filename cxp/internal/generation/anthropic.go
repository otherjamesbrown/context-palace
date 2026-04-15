package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	anthropicAPIURL = "https://api.anthropic.com/v1/messages"
	anthropicVersion = "2023-06-01"
	anthropicDefaultModel = "claude-haiku-4-5-20251001"
	anthropicDefaultMaxTokens = 1024
)

// modelAliases maps short aliases to full Anthropic model IDs.
var modelAliases = map[string]string{
	"claude-haiku-4-5": "claude-haiku-4-5-20251001",
}

// AnthropicGenerator generates text using the Anthropic Messages API.
type AnthropicGenerator struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewAnthropicGenerator creates an AnthropicGenerator with the given API key and model.
// If model is empty or an alias (e.g. "claude-haiku-4-5"), it is resolved to the full model ID.
func NewAnthropicGenerator(apiKey, model string) *AnthropicGenerator {
	if model == "" {
		model = anthropicDefaultModel
	}
	if resolved, ok := modelAliases[model]; ok {
		model = resolved
	}
	return &AnthropicGenerator{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *AnthropicGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("cannot generate from empty prompt")
	}

	reqBody := anthropicRequest{
		Model:     a.model,
		MaxTokens: anthropicDefaultMaxTokens,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("generation request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error %s: %s", result.Error.Type, result.Error.Message)
	}

	for _, block := range result.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("API returned no text content")
}
