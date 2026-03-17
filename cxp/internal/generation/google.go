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

// GoogleGenerator generates text using Google Gemini.
type GoogleGenerator struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGoogleGenerator creates a GoogleGenerator with the given API key and model.
func NewGoogleGenerator(apiKey, model string) *GoogleGenerator {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GoogleGenerator{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type generateRequest struct {
	Contents []generateContent `json:"contents"`
}

type generateContent struct {
	Parts []generatePart `json:"parts"`
}

type generatePart struct {
	Text string `json:"text"`
}

type generateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (g *GoogleGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("cannot generate from empty prompt")
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.model, g.apiKey,
	)

	reqBody := generateRequest{
		Contents: []generateContent{
			{Parts: []generatePart{{Text: prompt}}},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
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

	var result generateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error %d: %s", result.Error.Code, result.Error.Message)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("API returned no content")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}
