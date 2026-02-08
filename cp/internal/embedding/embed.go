package embedding

import (
	"context"
	"fmt"
	"strings"
)

// Provider generates vector embeddings from text
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimensions() int
}

// BuildEmbeddingText constructs the text to embed from shard fields.
// Format: "{type}: {title}\n\n{content}", truncated to 32000 chars.
func BuildEmbeddingText(shardType, title, content string) string {
	var sb strings.Builder

	if shardType != "" {
		sb.WriteString(shardType)
		sb.WriteString(": ")
	}
	sb.WriteString(title)

	content = strings.TrimSpace(content)
	if content != "" {
		sb.WriteString("\n\n")
		sb.WriteString(content)
	}

	text := sb.String()
	if text == "" {
		return ""
	}

	const maxChars = 32000
	if len(text) > maxChars {
		text = text[:maxChars]
	}

	return text
}

// FormatDimensionError returns a helpful error message for dimension mismatches
func FormatDimensionError(expected, got int) error {
	return fmt.Errorf("embedding dimension mismatch: expected %d, provider returns %d", expected, got)
}
