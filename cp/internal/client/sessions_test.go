package client

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrNoSession_IsDetectable(t *testing.T) {
	// ErrNoSession should be detectable with errors.Is
	assert.True(t, errors.Is(ErrNoSession, ErrNoSession))

	// A wrapped error should still match
	wrapped := fmt.Errorf("wrapped: %w", ErrNoSession)
	assert.True(t, errors.Is(wrapped, ErrNoSession))
}
