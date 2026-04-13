package scheduler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type mockRunner struct {
	name string
}

func (m mockRunner) Name() string { return m.name }
func (m mockRunner) Run(_ context.Context, _ json.RawMessage) (string, json.RawMessage, error) {
	return "", nil, nil
}

func TestRegistry_RegisterGet(t *testing.T) {
	r := NewRegistry()
	runner := mockRunner{name: "test-workflow"}
	r.Register(runner)

	got, err := r.Get("test-workflow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name() != runner.Name() {
		t.Errorf("got runner %q, want %q", got.Name(), runner.Name())
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown workflow type, got nil")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error %q does not contain workflow type name", err.Error())
	}
}

func TestRegistry_ListSorted(t *testing.T) {
	r := NewRegistry()
	r.Register(mockRunner{name: "zebra"})
	r.Register(mockRunner{name: "alpha"})
	r.Register(mockRunner{name: "middle"})

	names := r.List()
	want := []string{"alpha", "middle", "zebra"}
	if len(names) != len(want) {
		t.Fatalf("got %d names, want %d", len(names), len(want))
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestRegistry_RegisterOverwrites(t *testing.T) {
	r := NewRegistry()

	first := mockRunner{name: "my-workflow"}
	r.Register(first)

	second := mockRunner{name: "my-workflow"}
	r.Register(second)

	names := r.List()
	if len(names) != 1 {
		t.Errorf("expected 1 registered runner after overwrite, got %d", len(names))
	}

	got, err := r.Get("my-workflow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name() != "my-workflow" {
		t.Errorf("got %q, want %q", got.Name(), "my-workflow")
	}
}
