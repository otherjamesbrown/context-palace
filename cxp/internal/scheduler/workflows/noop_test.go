package workflows

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNoopRunner_Name(t *testing.T) {
	r := NoopRunner{}
	if got := r.Name(); got != "noop" {
		t.Errorf("Name() = %q, want %q", got, "noop")
	}
}

func TestNoopRunner_Run(t *testing.T) {
	r := NoopRunner{}
	summary, result, err := r.Run(context.Background(), nil)

	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	if summary != "noop completed" {
		t.Errorf("summary = %q, want %q", summary, "noop completed")
	}

	var got map[string]string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	msg, ok := got["message"]
	if !ok {
		t.Error("result JSON missing 'message' field")
	} else if msg != "noop workflow completed successfully" {
		t.Errorf("message = %q, want %q", msg, "noop workflow completed successfully")
	}

	ranAt, ok := got["ran_at"]
	if !ok {
		t.Fatal("result JSON missing 'ran_at' field")
	}
	if _, err := time.Parse(time.RFC3339, ranAt); err != nil {
		t.Errorf("ran_at %q is not valid RFC3339: %v", ranAt, err)
	}
}
