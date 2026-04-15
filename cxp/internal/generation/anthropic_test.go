package generation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ModelFamily tests ---

func TestModelFamily_WithSlash(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"gemini/gemini-2.0-flash", "gemini"},
		{"claude/claude-haiku-4-5", "claude"},
		{"google/gemini-pro", "google"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, ModelFamily(tc.input), "input: %s", tc.input)
	}
}

func TestModelFamily_NoSlash(t *testing.T) {
	assert.Equal(t, "gemini-2.0-flash", ModelFamily("gemini-2.0-flash"))
	assert.Equal(t, "claude-haiku-4-5", ModelFamily("claude-haiku-4-5"))
}

// --- ValidateDifferentFamilies tests ---

func TestValidateDifferentFamilies_Different(t *testing.T) {
	err := ValidateDifferentFamilies("gemini/gemini-2.0-flash", "claude/claude-haiku-4-5")
	assert.NoError(t, err)
}

func TestValidateDifferentFamilies_Same(t *testing.T) {
	err := ValidateDifferentFamilies("gemini/gemini-2.0-flash", "gemini/gemini-pro")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gemini")
}

func TestValidateDifferentFamilies_SameNoSlash(t *testing.T) {
	err := ValidateDifferentFamilies("claude", "claude")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude")
}

// --- NewGenerator anthropic tests ---

func TestNewGenerator_AnthropicMissingAPIKey(t *testing.T) {
	cfg := &GenerationConfig{
		Provider:  "anthropic",
		APIKeyEnv: "NONEXISTENT_ANTHROPIC_TEST_VAR_XYZ",
	}
	t.Setenv("NONEXISTENT_ANTHROPIC_TEST_VAR_XYZ", "")
	_, err := NewGenerator(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found")
}

func TestNewGenerator_AnthropicValid(t *testing.T) {
	cfg := &GenerationConfig{
		Provider:  "anthropic",
		Model:     "claude-haiku-4-5",
		APIKeyEnv: "TEST_ANTHROPIC_API_KEY",
	}
	t.Setenv("TEST_ANTHROPIC_API_KEY", "dummy-key")
	g, err := NewGenerator(cfg)
	require.NoError(t, err)
	assert.NotNil(t, g)
}

// --- AnthropicGenerator.Generate tests (mock HTTP) ---

func newTestAnthropicServer(t *testing.T, statusCode int, body interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, anthropicVersion, r.Header.Get("anthropic-version"))
		assert.NotEmpty(t, r.Header.Get("x-api-key"))
		w.WriteHeader(statusCode)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func newAnthropicGeneratorWithURL(apiKey, model, baseURL string) *AnthropicGenerator {
	g := NewAnthropicGenerator(apiKey, model)
	// Override transport to redirect to test server — we patch via a custom RoundTripper.
	g.httpClient.Transport = &prefixRewriter{prefix: baseURL}
	return g
}

// prefixRewriter rewrites the request URL to hit the test server instead of api.anthropic.com.
type prefixRewriter struct {
	prefix string
}

func (r *prefixRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = r.prefix[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

func TestAnthropicGenerator_Success(t *testing.T) {
	srv := newTestAnthropicServer(t, 200, anthropicResponse{
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{{Type: "text", Text: "hello from claude"}},
	})
	defer srv.Close()

	g := newAnthropicGeneratorWithURL("test-key", "claude-haiku-4-5", srv.URL)
	out, err := g.Generate(context.Background(), "say hello")
	require.NoError(t, err)
	assert.Equal(t, "hello from claude", out)
}

func TestAnthropicGenerator_EmptyPrompt(t *testing.T) {
	g := NewAnthropicGenerator("key", "")
	_, err := g.Generate(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty prompt")
}

func TestAnthropicGenerator_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid api key"}}`))
	}))
	defer srv.Close()

	g := newAnthropicGeneratorWithURL("bad-key", "", srv.URL)
	_, err := g.Generate(context.Background(), "prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestAnthropicGenerator_NoTextContent(t *testing.T) {
	srv := newTestAnthropicServer(t, 200, anthropicResponse{
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{{Type: "tool_use", Text: ""}},
	})
	defer srv.Close()

	g := newAnthropicGeneratorWithURL("test-key", "", srv.URL)
	_, err := g.Generate(context.Background(), "prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no text content")
}

func TestAnthropicGenerator_ModelAlias(t *testing.T) {
	g := NewAnthropicGenerator("key", "claude-haiku-4-5")
	assert.Equal(t, "claude-haiku-4-5-20251001", g.model)
}

func TestAnthropicGenerator_DefaultModel(t *testing.T) {
	g := NewAnthropicGenerator("key", "")
	assert.Equal(t, anthropicDefaultModel, g.model)
}
