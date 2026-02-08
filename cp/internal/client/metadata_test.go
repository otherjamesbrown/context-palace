package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseDotPath tests ---

func TestParseDotPath_Simple(t *testing.T) {
	assert.Equal(t, []string{"foo"}, parseDotPath("foo"))
}

func TestParseDotPath_Nested(t *testing.T) {
	assert.Equal(t, []string{"test_coverage", "unit"}, parseDotPath("test_coverage.unit"))
}

func TestParseDotPath_DeepNested(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c", "d"}, parseDotPath("a.b.c.d"))
}

func TestParseDotPath_Empty(t *testing.T) {
	assert.Equal(t, []string{""}, parseDotPath(""))
}

// --- parseMetaFlag tests ---

func TestParseMetaFlag_KeyValue(t *testing.T) {
	result, err := parseMetaFlag("name=Alice")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"name": "Alice"}, result)
}

func TestParseMetaFlag_KeyValueInteger(t *testing.T) {
	result, err := parseMetaFlag("count=42")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"count": int64(42)}, result)
}

func TestParseMetaFlag_KeyValueBool(t *testing.T) {
	result, err := parseMetaFlag("active=true")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"active": true}, result)
}

func TestParseMetaFlag_KeyValueFloat(t *testing.T) {
	result, err := parseMetaFlag("rate=3.14")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"rate": 3.14}, result)
}

func TestParseMetaFlag_JSONObject(t *testing.T) {
	result, err := parseMetaFlag(`{"key": "value", "num": 1}`)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, float64(1), result["num"]) // JSON numbers decode as float64
}

func TestParseMetaFlag_InvalidJSON(t *testing.T) {
	_, err := parseMetaFlag(`{broken`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestParseMetaFlag_NoEquals(t *testing.T) {
	_, err := parseMetaFlag("justtext")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metadata format")
}

func TestParseMetaFlag_EmptyKey(t *testing.T) {
	_, err := parseMetaFlag("=value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metadata format")
}

func TestParseMetaFlag_Whitespace(t *testing.T) {
	result, err := parseMetaFlag("  name=Alice  ")
	require.NoError(t, err)
	assert.Equal(t, "Alice", result["name"]) // entire string is trimmed before parsing
}

func TestParseMetaFlag_ValueWithEquals(t *testing.T) {
	result, err := parseMetaFlag("formula=a=b")
	require.NoError(t, err)
	assert.Equal(t, "a=b", result["formula"])
}

// --- inferJSONValue tests ---

func TestInferJSONValue_Integer(t *testing.T) {
	assert.Equal(t, int64(42), inferJSONValue("42"))
}

func TestInferJSONValue_NegativeInteger(t *testing.T) {
	assert.Equal(t, int64(-5), inferJSONValue("-5"))
}

func TestInferJSONValue_Float(t *testing.T) {
	assert.Equal(t, 3.14, inferJSONValue("3.14"))
}

func TestInferJSONValue_BoolTrue(t *testing.T) {
	assert.Equal(t, true, inferJSONValue("true"))
}

func TestInferJSONValue_BoolFalse(t *testing.T) {
	assert.Equal(t, false, inferJSONValue("false"))
}

func TestInferJSONValue_String(t *testing.T) {
	assert.Equal(t, "hello", inferJSONValue("hello"))
}

func TestInferJSONValue_JSONObject(t *testing.T) {
	result := inferJSONValue(`{"key": "val"}`)
	m, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "val", m["key"])
}

func TestInferJSONValue_JSONArray(t *testing.T) {
	result := inferJSONValue(`[1, 2, 3]`)
	arr, ok := result.([]interface{})
	require.True(t, ok)
	assert.Len(t, arr, 3)
}

func TestInferJSONValue_EmptyString(t *testing.T) {
	assert.Equal(t, "", inferJSONValue(""))
}

func TestInferJSONValue_Zero(t *testing.T) {
	assert.Equal(t, int64(0), inferJSONValue("0"))
}

func TestInferJSONValue_InvalidBraces(t *testing.T) {
	// Starts with { but isn't valid JSON — falls through to string
	assert.Equal(t, "{broken", inferJSONValue("{broken"))
}
