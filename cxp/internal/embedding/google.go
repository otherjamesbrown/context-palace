package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GoogleProvider embeds text using Google's text-embedding-004 model via the Gemini API.
type GoogleProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGoogleProvider creates a GoogleProvider with the given API key and model.
func NewGoogleProvider(apiKey, model string) *GoogleProvider {
	if model == "" {
		model = "gemini-embedding-001"
	}
	return &GoogleProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (g *GoogleProvider) Dimensions() int {
	return 768
}

// googleEmbedRequest is the request body for the Gemini embedContent API.
type googleEmbedRequest struct {
	Content              googleContent `json:"content"`
	OutputDimensionality int           `json:"outputDimensionality,omitempty"`
}

type googleContent struct {
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text string `json:"text"`
}

// googleEmbedResponse is the response from the Gemini embedContent API.
type googleEmbedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
	Error *googleError `json:"error,omitempty"`
}

type googleError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (g *GoogleProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("cannot embed empty text")
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s",
		g.model, g.apiKey,
	)

	reqBody := googleEmbedRequest{
		Content: googleContent{
			Parts: []googlePart{{Text: text}},
		},
		OutputDimensionality: g.Dimensions(),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Retry with exponential backoff on 429/5xx
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	var lastErr error

	for attempt := 0; attempt <= len(delays); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delays[attempt-1]):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := g.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %v", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %v", err)
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
		}

		var result googleEmbedResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %v", err)
		}

		if result.Error != nil {
			return nil, fmt.Errorf("API error %d: %s", result.Error.Code, result.Error.Message)
		}

		if len(result.Embedding.Values) == 0 {
			return nil, fmt.Errorf("API returned empty embedding")
		}

		return result.Embedding.Values, nil
	}

	return nil, fmt.Errorf("embedding failed after retries: %v", lastErr)
}
