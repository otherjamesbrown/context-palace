package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestResolveMessageBody(t *testing.T) {
	tests := []struct {
		name        string
		positional  string
		flag        string
		stdin       io.Reader
		wantBody    string
		wantErr     string
	}{
		{
			name:       "positional arg only",
			positional: "Hello from positional",
			wantBody:   "Hello from positional",
		},
		{
			name:     "flag only",
			flag:     "Hello from flag",
			wantBody: "Hello from flag",
		},
		{
			name:     "stdin only",
			stdin:    strings.NewReader("Hello from stdin"),
			wantBody: "Hello from stdin",
		},
		{
			name:       "positional and flag conflict",
			positional: "From positional",
			flag:       "From flag",
			wantErr:    "cannot specify both positional body argument and --body flag",
		},
		{
			name:    "no body from any source",
			wantErr: "message body is required",
		},
		{
			name:     "empty flag value",
			flag:     "",
			wantErr:  "message body is required",
		},
		{
			name:  "empty stdin",
			stdin: strings.NewReader(""),
			wantErr: "message body is required",
		},
		{
			name:       "positional takes priority over stdin",
			positional: "From positional",
			stdin:      strings.NewReader("From stdin"),
			wantBody:   "From positional",
		},
		{
			name:     "flag takes priority over stdin",
			flag:     "From flag",
			stdin:    strings.NewReader("From stdin"),
			wantBody: "From flag",
		},
		{
			name:     "stdin with whitespace trimmed",
			stdin:    strings.NewReader("  Hello from stdin  \n"),
			wantBody: "Hello from stdin",
		},
		{
			name:       "positional with whitespace only",
			positional: "   ",
			wantErr:    "message body is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMessageBody(tt.positional, tt.flag, tt.stdin)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantBody {
				t.Errorf("got %q, want %q", got, tt.wantBody)
			}
		})
	}
}

// TestMessageSendWithTwoArgsNoBodyNoStdin reproduces bug pf-424bad:
// When calling `message send <recipient> <subject>` with exactly 2 args,
// no --body flag, and no stdin available, the command should immediately
// error with a clear message instead of blocking on stdin read.
//
// The bug occurs because stdin detection via os.ModeCharDevice can fail
// in tmux/CI environments, causing the command to attempt reading from
// os.Stdin even when nothing is piped.
//
// This test verifies that when stdin is truly unavailable (nil), the
// resolveMessageBody function correctly returns an error immediately.
func TestMessageSendWithTwoArgsNoBodyNoStdin(t *testing.T) {
	// Test the scenario: 2 args provided, no --body flag, no stdin
	positionalBody := ""  // No 3rd positional arg
	flagBody := ""        // No --body flag
	var stdinReader io.Reader = nil  // No stdin available

	// This should error immediately, not block
	body, err := resolveMessageBody(positionalBody, flagBody, stdinReader)

	// We expect an error
	if err == nil {
		t.Fatalf("expected error when no body source available, got body: %q", body)
	}

	// Verify the error message is clear and actionable
	expectedMsg := "message body is required"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error containing %q, got: %v", expectedMsg, err)
	}

	// Verify the error mentions all available input methods
	if !strings.Contains(err.Error(), "3rd argument") {
		t.Errorf("error should mention 3rd positional argument option, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--body") {
		t.Errorf("error should mention --body flag option, got: %v", err)
	}
	if !strings.Contains(err.Error(), "stdin") {
		t.Errorf("error should mention stdin option, got: %v", err)
	}
}

// TestMessageSendBlocksOnRealStdin documents bug pf-424bad:
// This test demonstrates the problematic behavior when os.Stdin is passed
// to resolveMessageBody even though nothing is actually piped.
//
// CRITICAL: This test is SKIPPED by default because it would block indefinitely
// in the current (buggy) implementation. To manually verify the bug:
// 1. Remove the t.Skip() line
// 2. Run: go test ./cmd -run TestMessageSendBlocksOnRealStdin -v -timeout 5s
// 3. The test will TIMEOUT, demonstrating the bug
// 4. After fixing the bug in message.go, this test should PASS
//
// The bug is in message.go lines 40-44: stdin detection via os.ModeCharDevice
// fails in tmux/CI environments, causing stdinReader to be set to os.Stdin
// even when nothing is piped, which causes io.ReadAll to block indefinitely.
func TestMessageSendBlocksOnRealStdin(t *testing.T) {
	t.Skip("This test would block indefinitely with current buggy code - remove skip to manually verify bug reproduction")

	// Simulate the problematic code path in message.go lines 40-44
	// where stdinReader is set to os.Stdin even when nothing is piped
	positionalBody := ""
	flagBody := ""
	stdinReader := os.Stdin  // BUG: This will block if nothing is piped

	// Create a channel to catch the result
	done := make(chan struct{})
	var body string
	var err error

	go func() {
		body, err = resolveMessageBody(positionalBody, flagBody, stdinReader)
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// EXPECTED AFTER FIX: Should return quickly with an error
		if err == nil {
			t.Fatalf("expected error when no stdin piped, got body: %q", body)
		}
		if !strings.Contains(err.Error(), "message body is required") {
			t.Errorf("expected 'message body is required' error, got: %v", err)
		}
		t.Log("SUCCESS: Command returned immediately with error instead of blocking")
	case <-time.After(1 * time.Second):
		// BUG REPRODUCED: The function is blocking on os.Stdin.Read()
		t.Fatal("resolveMessageBody blocked on os.Stdin read - bug pf-424bad reproduced")
	}
}
