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

// KBSearchResult represents a knowledge tree search result
type KBSearchResult struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Type          string    `json:"type"`
	Status        string    `json:"status"`
	Description   string    `json:"description"`
	TextRank      float64   `json:"text_rank"`
	Similarity    float64   `json:"similarity"`
	CombinedScore float64   `json:"combined_score"`
	Depth         int       `json:"depth"`
	ParentPath    []string  `json:"parent_path"`
	Snippet       string    `json:"snippet"`
	Labels        []string  `json:"labels,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// KBSearch searches the knowledge tree rooted at rootID using the knowledge_tree_search() function.
func (c *Client) KBSearch(ctx context.Context, rootID string, query string, queryEmbedding []float32, includeClosed bool, limit int, minSimilarity float64) ([]KBSearchResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var embArg any
	if queryEmbedding != nil {
		embArg = pgvec.NewVector(queryEmbedding)
	}
	var queryArg any
	if query != "" {
		queryArg = query
	}

	rows, err := conn.Query(ctx, `
		SELECT id, title, type, status, COALESCE(description,''), text_rank, similarity,
			combined_score, depth, parent_path, COALESCE(snippet,''), labels, created_at
		FROM knowledge_tree_search($1, $2, $3, $4, $5, $6, $7)
	`, c.Config.Project, rootID, queryArg, embArg, includeClosed, limit, minSimilarity)
	if err != nil {
		return nil, fmt.Errorf("kb search failed: %v", err)
	}
	defer rows.Close()

	var results []KBSearchResult
	for rows.Next() {
		var r KBSearchResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Type, &r.Status, &r.Description,
			&r.TextRank, &r.Similarity, &r.CombinedScore, &r.Depth,
			&r.ParentPath, &r.Snippet, &r.Labels, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan kb result: %v", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// KBTreeNode represents a node in the knowledge tree listing
type KBTreeNode struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	Description     string    `json:"description"`
	ChildCount      int       `json:"child_count"`
	Depth           int       `json:"depth"`
	ParentPath      []string  `json:"parent_path"`
	Labels          []string  `json:"labels,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	EdgeTrigger     *string   `json:"edge_trigger,omitempty"`
	EdgeDescription *string   `json:"edge_description,omitempty"`
	EdgeOrdering    *int      `json:"edge_ordering,omitempty"`
}

// KBTree lists the knowledge tree structure from a root shard.
func (c *Client) KBTree(ctx context.Context, rootID string, maxDepth int, includeClosed bool) ([]KBTreeNode, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, type, status, COALESCE(description,''), child_count,
			depth, parent_path, labels, created_at,
			edge_trigger, edge_description, edge_ordering
		FROM knowledge_tree_list($1, $2, $3, $4)
	`, c.Config.Project, rootID, maxDepth, includeClosed)
	if err != nil {
		return nil, fmt.Errorf("kb tree failed: %v", err)
	}
	defer rows.Close()

	var nodes []KBTreeNode
	for rows.Next() {
		var n KBTreeNode
		if err := rows.Scan(&n.ID, &n.Title, &n.Type, &n.Status, &n.Description,
			&n.ChildCount, &n.Depth, &n.ParentPath, &n.Labels, &n.CreatedAt,
			&n.EdgeTrigger, &n.EdgeDescription, &n.EdgeOrdering); err != nil {
			return nil, fmt.Errorf("failed to scan kb tree node: %v", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// KBRoots discovers knowledge base root shards — knowledge shards that have
// children (incoming child-of edges) but no parent (no outgoing child-of edge).
// Returns them ordered by title.
func (c *Client) KBRoots(ctx context.Context, includeClosed bool) ([]KBTreeNode, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	statusFilter := ""
	if !includeClosed {
		statusFilter = "AND s.status NOT IN ('closed', 'deferred')"
	}

	query := fmt.Sprintf(`
		SELECT s.id, s.title, s.type, s.status, COALESCE(s.content, ''),
			(SELECT count(*) FROM edges e2 WHERE e2.to_id = s.id AND e2.edge_type = 'child-of') AS child_count,
			0 AS depth, ARRAY[]::text[] AS parent_path, s.labels, s.created_at,
			NULL::text, NULL::text, NULL::int
		FROM shards s
		LEFT JOIN edges e ON s.id = e.from_id AND e.edge_type = 'child-of'
		WHERE s.project = $1 AND s.type = 'knowledge'
		AND e.to_id IS NULL
		AND EXISTS (SELECT 1 FROM edges e2 WHERE e2.to_id = s.id AND e2.edge_type = 'child-of')
		%s
		ORDER BY s.title
	`, statusFilter)

	rows, err := conn.Query(ctx, query, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("kb roots query failed: %v", err)
	}
	defer rows.Close()

	var nodes []KBTreeNode
	for rows.Next() {
		var n KBTreeNode
		if err := rows.Scan(&n.ID, &n.Title, &n.Type, &n.Status, &n.Description,
			&n.ChildCount, &n.Depth, &n.ParentPath, &n.Labels, &n.CreatedAt,
			&n.EdgeTrigger, &n.EdgeDescription, &n.EdgeOrdering); err != nil {
			return nil, fmt.Errorf("failed to scan kb root: %v", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
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
