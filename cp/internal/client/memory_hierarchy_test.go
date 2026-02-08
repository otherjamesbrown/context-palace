package client

import (
	"testing"

	"github.com/otherjamesbrown/context-palace/cp/internal/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- jsonQuote tests ---

func TestJsonQuote_SimpleString(t *testing.T) {
	assert.Equal(t, `"hello"`, jsonQuote("hello"))
}

func TestJsonQuote_EmptyString(t *testing.T) {
	assert.Equal(t, `""`, jsonQuote(""))
}

func TestJsonQuote_SpecialCharacters(t *testing.T) {
	// JSON encoding should escape quotes and backslashes
	assert.Equal(t, `"say \"hello\""`, jsonQuote(`say "hello"`))
}

func TestJsonQuote_Newlines(t *testing.T) {
	result := jsonQuote("line1\nline2")
	assert.Contains(t, result, `\n`)
	assert.Equal(t, `"line1\nline2"`, result)
}

func TestJsonQuote_Unicode(t *testing.T) {
	// json.Marshal preserves UTF-8 by default
	result := jsonQuote("café")
	assert.Equal(t, `"café"`, result)
}

// --- sortStrings tests ---

func TestSortStrings_Empty(t *testing.T) {
	ss := []string{}
	sortStrings(ss)
	assert.Empty(t, ss)
}

func TestSortStrings_Single(t *testing.T) {
	ss := []string{"a"}
	sortStrings(ss)
	assert.Equal(t, []string{"a"}, ss)
}

func TestSortStrings_AlreadySorted(t *testing.T) {
	ss := []string{"a", "b", "c"}
	sortStrings(ss)
	assert.Equal(t, []string{"a", "b", "c"}, ss)
}

func TestSortStrings_Reversed(t *testing.T) {
	ss := []string{"c", "b", "a"}
	sortStrings(ss)
	assert.Equal(t, []string{"a", "b", "c"}, ss)
}

func TestSortStrings_Duplicates(t *testing.T) {
	ss := []string{"b", "a", "b", "a"}
	sortStrings(ss)
	assert.Equal(t, []string{"a", "a", "b", "b"}, ss)
}

func TestSortStrings_Nil(t *testing.T) {
	var ss []string
	sortStrings(ss) // should not panic
	assert.Nil(t, ss)
}

// --- findSyncDiscrepancies tests ---

func TestFindSyncDiscrepancies_AllMatch(t *testing.T) {
	blockEntries := []pointer.SubMemoryEntry{
		{ID: "a", Title: "A", Summary: "s"},
		{ID: "b", Title: "B", Summary: "s"},
	}
	actualChildren := []syncChildEntry{
		{ID: "a", Title: "A"},
		{ID: "b", Title: "B"},
	}

	discs := findSyncDiscrepancies("parent-1", blockEntries, actualChildren)
	assert.Empty(t, discs)
}

func TestFindSyncDiscrepancies_MissingPointer(t *testing.T) {
	blockEntries := []pointer.SubMemoryEntry{
		{ID: "a", Title: "A", Summary: "s"},
	}
	actualChildren := []syncChildEntry{
		{ID: "a", Title: "A"},
		{ID: "b", Title: "B"},
	}

	discs := findSyncDiscrepancies("parent-1", blockEntries, actualChildren)
	require.Len(t, discs, 1)
	assert.Equal(t, "missing_pointer", discs[0].Type)
	assert.Equal(t, "b", discs[0].ChildID)
	assert.Equal(t, "B", discs[0].ChildTitle)
	assert.Equal(t, "parent-1", discs[0].ParentID)
}

func TestFindSyncDiscrepancies_StalePointer(t *testing.T) {
	blockEntries := []pointer.SubMemoryEntry{
		{ID: "a", Title: "A", Summary: "s"},
		{ID: "b", Title: "B", Summary: "s"},
	}
	actualChildren := []syncChildEntry{
		{ID: "a", Title: "A"},
	}

	discs := findSyncDiscrepancies("parent-1", blockEntries, actualChildren)
	require.Len(t, discs, 1)
	assert.Equal(t, "stale_pointer", discs[0].Type)
	assert.Equal(t, "b", discs[0].ChildID)
	assert.Equal(t, "parent-1", discs[0].ParentID)
}

func TestFindSyncDiscrepancies_BothMissingAndStale(t *testing.T) {
	blockEntries := []pointer.SubMemoryEntry{
		{ID: "a", Title: "A", Summary: "s"},
		{ID: "stale", Title: "Gone", Summary: "s"},
	}
	actualChildren := []syncChildEntry{
		{ID: "a", Title: "A"},
		{ID: "new", Title: "New Child"},
	}

	discs := findSyncDiscrepancies("parent-1", blockEntries, actualChildren)
	require.Len(t, discs, 2)

	// Collect by type
	types := map[string]SyncDiscrepancy{}
	for _, d := range discs {
		types[d.Type+":"+d.ChildID] = d
	}

	assert.Contains(t, types, "missing_pointer:new")
	assert.Contains(t, types, "stale_pointer:stale")
}

func TestFindSyncDiscrepancies_EmptyBlock(t *testing.T) {
	blockEntries := []pointer.SubMemoryEntry{}
	actualChildren := []syncChildEntry{
		{ID: "a", Title: "A"},
	}

	discs := findSyncDiscrepancies("parent-1", blockEntries, actualChildren)
	require.Len(t, discs, 1)
	assert.Equal(t, "missing_pointer", discs[0].Type)
}

func TestFindSyncDiscrepancies_NoChildren(t *testing.T) {
	blockEntries := []pointer.SubMemoryEntry{
		{ID: "a", Title: "A", Summary: "s"},
	}
	actualChildren := []syncChildEntry{}

	discs := findSyncDiscrepancies("parent-1", blockEntries, actualChildren)
	require.Len(t, discs, 1)
	assert.Equal(t, "stale_pointer", discs[0].Type)
}

func TestFindSyncDiscrepancies_BothEmpty(t *testing.T) {
	discs := findSyncDiscrepancies("parent-1", nil, nil)
	assert.Empty(t, discs)
}
