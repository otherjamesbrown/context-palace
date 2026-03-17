package embedding

import "context"

// MockProvider implements Provider for testing.
type MockProvider struct {
	EmbedFunc      func(ctx context.Context, text string) ([]float32, error)
	DimensionsFunc func() int
	EmbedCalls     []string // records text arguments
}

func (m *MockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	m.EmbedCalls = append(m.EmbedCalls, text)
	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, text)
	}
	return make([]float32, m.Dimensions()), nil
}

func (m *MockProvider) Dimensions() int {
	if m.DimensionsFunc != nil {
		return m.DimensionsFunc()
	}
	return 768
}
