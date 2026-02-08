package client

import (
	"context"
	"fmt"
	"time"

	pgvec "github.com/pgvector/pgvector-go"
)

// RecallResult represents a semantic search result
type RecallResult struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Similarity float64   `json:"similarity"`
	Snippet    string    `json:"snippet"`
	Labels     []string  `json:"labels,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SemanticSearch performs vector similarity search using the semantic_search() SQL function.
func (c *Client) SemanticSearch(ctx context.Context, queryEmbedding []float32, types []string, labels []string, status []string, limit int, minSimilarity float64) ([]RecallResult, error) {
	return c.SemanticSearchWithSince(ctx, queryEmbedding, types, labels, status, limit, minSimilarity, nil)
}

// SemanticSearchWithSince performs semantic search with an optional time cutoff.
func (c *Client) SemanticSearchWithSince(ctx context.Context, queryEmbedding []float32, types []string, labels []string, status []string, limit int, minSimilarity float64, since *time.Time) ([]RecallResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	vec := pgvec.NewVector(queryEmbedding)

	// Convert nil slices to typed nil for proper SQL NULL handling
	var typesArg, labelsArg, statusArg, sinceArg any
	if types != nil {
		typesArg = types
	}
	if labels != nil {
		labelsArg = labels
	}
	if status != nil {
		statusArg = status
	}
	if since != nil {
		sinceArg = *since
	}

	rows, err := conn.Query(ctx, `
		SELECT id, title, type, status, similarity, snippet, labels, created_at
		FROM semantic_search($1, $2, $3, $4, $5, $6, $7, $8)
	`, c.Config.Project, vec, typesArg, labelsArg, statusArg, limit, minSimilarity, sinceArg)
	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %v", err)
	}
	defer rows.Close()

	var results []RecallResult
	for rows.Next() {
		var r RecallResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Type, &r.Status, &r.Similarity, &r.Snippet, &r.Labels, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan result: %v", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("result iteration error: %v", err)
	}

	return results, nil
}

// MemoryRecallResult represents a memory semantic search result
type MemoryRecallResult struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Similarity float64   `json:"similarity"`
	Labels     []string  `json:"labels,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// MemoryRecall performs semantic search limited to memory shards.
func (c *Client) MemoryRecall(ctx context.Context, queryEmbedding []float32, labels []string, limit int, minSimilarity float64) ([]MemoryRecallResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	vec := pgvec.NewVector(queryEmbedding)

	var labelsArg any
	if labels != nil {
		labelsArg = labels
	}

	rows, err := conn.Query(ctx, `
		SELECT id, title, content, similarity, labels, created_at
		FROM memory_recall($1, $2, $3, $4, $5)
	`, c.Config.Project, vec, labelsArg, limit, minSimilarity)
	if err != nil {
		return nil, fmt.Errorf("memory recall failed: %v", err)
	}
	defer rows.Close()

	var results []MemoryRecallResult
	for rows.Next() {
		var r MemoryRecallResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Content, &r.Similarity, &r.Labels, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan result: %v", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("result iteration error: %v", err)
	}

	return results, nil
}

// UpdateEmbedding stores an embedding vector for a shard.
func (c *Client) UpdateEmbedding(ctx context.Context, shardID string, emb []float32) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	vec := pgvec.NewVector(emb)

	result, err := conn.Exec(ctx, `
		UPDATE shards SET embedding = $1 WHERE id = $2
	`, vec, shardID)
	if err != nil {
		return fmt.Errorf("failed to update embedding: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("shard not found: %s", shardID)
	}
	return nil
}

// ShardForEmbedding represents a shard that needs embedding
type ShardForEmbedding struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// GetShardsNeedingEmbedding returns shards without embeddings.
func (c *Client) GetShardsNeedingEmbedding(ctx context.Context, limit int) ([]ShardForEmbedding, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, type FROM shards_needing_embedding($1, $2)
	`, c.Config.Project, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query shards needing embedding: %v", err)
	}
	defer rows.Close()

	var shards []ShardForEmbedding
	for rows.Next() {
		var s ShardForEmbedding
		if err := rows.Scan(&s.ID, &s.Title, &s.Type); err != nil {
			return nil, fmt.Errorf("failed to scan shard: %v", err)
		}
		shards = append(shards, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("result iteration error: %v", err)
	}

	return shards, nil
}

// GetShardContentForEmbedding fetches the type, title, and content of a shard for embedding.
func (c *Client) GetShardContentForEmbedding(ctx context.Context, id string) (shardType, title, content string, err error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return "", "", "", err
	}
	defer conn.Close(ctx)

	err = conn.QueryRow(ctx, `
		SELECT type, title, COALESCE(content, '') FROM shards WHERE id = $1
	`, id).Scan(&shardType, &title, &content)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch shard for embedding: %v", err)
	}
	return shardType, title, content, nil
}
