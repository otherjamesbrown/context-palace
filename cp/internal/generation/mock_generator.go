package generation

import "context"

// MockGenerator implements Generator for testing.
type MockGenerator struct {
	GenerateFunc  func(ctx context.Context, prompt string) (string, error)
	GenerateCalls []string // records prompt arguments
}

func (m *MockGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	m.GenerateCalls = append(m.GenerateCalls, prompt)
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, prompt)
	}
	return "{}", nil
}
