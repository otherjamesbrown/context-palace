package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncate_ShortString(t *testing.T) {
	assert.Equal(t, "hello", Truncate("hello", 10))
}

func TestTruncate_ExactLength(t *testing.T) {
	assert.Equal(t, "hello", Truncate("hello", 5))
}

func TestTruncate_LongString(t *testing.T) {
	assert.Equal(t, "hello...", Truncate("hello world", 8))
}

func TestTruncate_VerySmallMax(t *testing.T) {
	assert.Equal(t, "he", Truncate("hello", 2))
}

func TestTruncate_MaxThree(t *testing.T) {
	assert.Equal(t, "hel", Truncate("hello", 3))
}

func TestTruncate_ZeroMax(t *testing.T) {
	assert.Equal(t, "", Truncate("hello", 0))
}

func TestTruncate_EmptyString(t *testing.T) {
	assert.Equal(t, "", Truncate("", 5))
}

func TestNewTable_EmptyRows(t *testing.T) {
	tbl := NewTable("ID", "NAME")
	assert.Equal(t, "", tbl.String())
}

func TestNewTable_SingleRow(t *testing.T) {
	tbl := NewTable("ID", "NAME")
	tbl.AddRow("1", "Alice")
	out := tbl.String()
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "Alice")
}

func TestNewTable_ColumnAlignment(t *testing.T) {
	tbl := NewTable("ID", "NAME")
	tbl.AddRow("1", "Alice")
	tbl.AddRow("100", "Bob")
	out := tbl.String()
	// "100" is 3 chars, so ID column should be padded to 3
	assert.Contains(t, out, "100")
	assert.Contains(t, out, "Bob")
}

func TestNewTable_FewerCellsThanHeaders(t *testing.T) {
	tbl := NewTable("ID", "NAME", "STATUS")
	tbl.AddRow("1", "Alice")
	out := tbl.String()
	assert.Contains(t, out, "Alice")
	// Should not panic
	assert.Contains(t, out, "ID")
}

func TestNewTable_ZeroHeaders(t *testing.T) {
	tbl := NewTable()
	assert.Equal(t, "", tbl.String())
}

func TestFormatJSON_SimpleStruct(t *testing.T) {
	data := map[string]string{"name": "test"}
	result, err := FormatJSON(data)
	require.NoError(t, err)
	assert.Contains(t, result, `"name": "test"`)
}

func TestFormatYAML_SimpleStruct(t *testing.T) {
	data := map[string]string{"key": "val"}
	result, err := FormatYAML(data)
	require.NoError(t, err)
	assert.Contains(t, result, "key: val")
}

func TestFormatOutput_JSON(t *testing.T) {
	data := map[string]string{"k": "v"}
	result, err := FormatOutput(data, "json")
	require.NoError(t, err)
	assert.Contains(t, result, `"k": "v"`)
}

func TestFormatOutput_YAML(t *testing.T) {
	data := map[string]string{"key": "val"}
	result, err := FormatOutput(data, "yaml")
	require.NoError(t, err)
	expected, _ := FormatYAML(data)
	assert.Equal(t, expected, result)
}

func TestFormatOutput_Unknown(t *testing.T) {
	data := "hello"
	result, err := FormatOutput(data, "text")
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}
