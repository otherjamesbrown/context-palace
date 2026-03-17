package client

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateKnowledgeDocWithCustomID verifies that when a custom ID is provided,
// it is used instead of auto-generating a pf-* ID.
func TestCreateKnowledgeDocWithCustomID(t *testing.T) {
	requireDBTests(t)

	ctx := context.Background()
	c := testClient(t)

	customID := "test-custom-doc-id"
	title := "Test Document with Custom ID"
	content := "This is a test document created with a custom ID."
	docType := "reference"
	labels := []string{"test", "custom-id"}

	// Create knowledge doc with custom ID
	id, err := c.CreateKnowledgeDocWithID(ctx, customID, title, content, docType, labels)
	require.NoError(t, err, "CreateKnowledgeDocWithID should succeed")
	assert.Equal(t, customID, id, "returned ID should match the custom ID")

	// Verify the doc was created with the custom ID
	doc, err := c.ShowKnowledgeDoc(ctx, customID)
	require.NoError(t, err, "ShowKnowledgeDoc should find the custom ID doc")
	assert.Equal(t, customID, doc.ID)
	assert.Equal(t, title, doc.Title)
	assert.Equal(t, content, doc.Content)
	assert.Equal(t, docType, doc.DocType)
	assert.Equal(t, labels, doc.Labels)
}

// TestCreateKnowledgeDocWithoutCustomID verifies backward compatibility:
// when no custom ID is provided, the system auto-generates a pf-* ID.
func TestCreateKnowledgeDocWithoutCustomID(t *testing.T) {
	requireDBTests(t)

	ctx := context.Background()
	c := testClient(t)

	title := "Test Document with Auto ID"
	content := "This document should get an auto-generated ID."
	docType := "reference"
	labels := []string{"test"}

	// Create knowledge doc WITHOUT custom ID (using existing method)
	id, err := c.CreateKnowledgeDoc(ctx, title, content, docType, labels)
	require.NoError(t, err, "CreateKnowledgeDoc should succeed")
	assert.NotEmpty(t, id, "returned ID should not be empty")
	assert.Regexp(t, `^pf-[a-f0-9]{6}$`, id, "auto-generated ID should match pf-* pattern")

	// Verify the doc exists
	doc, err := c.ShowKnowledgeDoc(ctx, id)
	require.NoError(t, err, "ShowKnowledgeDoc should find the auto-generated ID doc")
	assert.Equal(t, id, doc.ID)
	assert.Equal(t, title, doc.Title)
}

// TestCreateKnowledgeDocWithDuplicateCustomID verifies that attempting to create
// a knowledge doc with a duplicate custom ID returns a clear error.
func TestCreateKnowledgeDocWithDuplicateCustomID(t *testing.T) {
	requireDBTests(t)

	ctx := context.Background()
	c := testClient(t)

	customID := "test-duplicate-custom-id"
	title := "First Document"
	content := "First document content."
	docType := "reference"
	labels := []string{"test"}

	// Create first doc with custom ID
	id1, err := c.CreateKnowledgeDocWithID(ctx, customID, title, content, docType, labels)
	require.NoError(t, err, "First CreateKnowledgeDocWithID should succeed")
	assert.Equal(t, customID, id1)

	// Attempt to create second doc with the same custom ID
	title2 := "Second Document"
	content2 := "Second document content."
	id2, err := c.CreateKnowledgeDocWithID(ctx, customID, title2, content2, docType, labels)
	require.Error(t, err, "Second CreateKnowledgeDocWithID should fail with duplicate ID")
	assert.Empty(t, id2, "returned ID should be empty on error")
	assert.Contains(t, err.Error(), "already exists", "error message should mention duplicate ID")
}

// TestCreateKnowledgeDocWithEmptyCustomID verifies that an empty custom ID
// is rejected with a clear validation error.
func TestCreateKnowledgeDocWithEmptyCustomID(t *testing.T) {
	requireDBTests(t)

	ctx := context.Background()
	c := testClient(t)

	customID := "" // empty custom ID
	title := "Test Document"
	content := "Test content."
	docType := "reference"
	labels := []string{"test"}

	// Attempt to create doc with empty custom ID
	id, err := c.CreateKnowledgeDocWithID(ctx, customID, title, content, docType, labels)
	require.Error(t, err, "CreateKnowledgeDocWithID should fail with empty ID")
	assert.Empty(t, id, "returned ID should be empty on error")
	assert.Contains(t, err.Error(), "empty", "error message should mention empty ID")
}

// TestCreateShardWithMetadataAndCustomID verifies that CreateShardWithMetadata
// accepts an optional custom ID parameter and uses it.
func TestCreateShardWithMetadataAndCustomID(t *testing.T) {
	requireDBTests(t)

	ctx := context.Background()
	c := testClient(t)

	customID := "test-shard-custom-id"
	title := "Test Shard with Custom ID"
	content := "Test shard content."
	shardType := "knowledge"
	labels := []string{"test"}
	metadata := []byte(`{"doc_type":"reference","version":1}`)

	// Create shard with custom ID
	id, err := c.CreateShardWithMetadataAndID(ctx, customID, title, content, shardType, nil, labels, metadata)
	require.NoError(t, err, "CreateShardWithMetadataAndID should succeed")
	assert.Equal(t, customID, id, "returned ID should match the custom ID")

	// Verify the shard was created with the custom ID
	shard, err := c.GetShard(ctx, customID)
	require.NoError(t, err, "GetShard should find the custom ID shard")
	assert.Equal(t, customID, shard.ID)
	assert.Equal(t, title, shard.Title)
	assert.Equal(t, content, shard.Content)
	assert.Equal(t, shardType, shard.Type)
}

// TestCreateShardWithMetadataWithoutCustomID verifies backward compatibility:
// CreateShardWithMetadata without custom ID still works and auto-generates.
func TestCreateShardWithMetadataWithoutCustomID(t *testing.T) {
	requireDBTests(t)

	ctx := context.Background()
	c := testClient(t)

	title := "Test Shard Auto ID"
	content := "Test shard content."
	shardType := "task"
	labels := []string{"test"}
	metadata := []byte(`{"priority":1}`)

	// Create shard WITHOUT custom ID (using existing method)
	id, err := c.CreateShardWithMetadata(ctx, title, content, shardType, nil, labels, metadata)
	require.NoError(t, err, "CreateShardWithMetadata should succeed")
	assert.NotEmpty(t, id, "returned ID should not be empty")
	assert.Regexp(t, `^pf-[a-f0-9]{6}$`, id, "auto-generated ID should match pf-* pattern")

	// Verify the shard exists
	shard, err := c.GetShard(ctx, id)
	require.NoError(t, err, "GetShard should find the auto-generated ID shard")
	assert.Equal(t, id, shard.ID)
}

// testClient returns a configured client for integration tests.
// This is a helper function that should match existing test infrastructure.
func testClient(t *testing.T) *Client {
	t.Helper()

	// Create a basic client configuration matching the actual Config structure
	cfg := &Config{
		Connection: ConnectionConfig{
			Host:     "localhost",
			Database: "context_palace_test",
			User:     "test_user",
		},
		Project: "test",
		Agent:   "test-agent",
	}

	client := NewClient(cfg)
	require.NotNil(t, client, "client should not be nil")

	return client
}

func requireDBTests(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv("CP_RUN_DB_TESTS") != "1" {
		t.Skip("skipping database integration test; set CP_RUN_DB_TESTS=1 to enable")
	}
}
