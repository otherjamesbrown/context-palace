package cmd

import (
	"io"
	"strings"
	"testing"
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
