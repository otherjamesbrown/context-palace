package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

// ValidDocTypes lists the allowed knowledge document types
var ValidDocTypes = []string{"architecture", "vision", "roadmap", "decision", "reference"}

// KnowledgeDoc represents a knowledge document shard
type KnowledgeDoc struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Content   string         `json:"content,omitempty"`
	DocType   string         `json:"doc_type"`
	Version   int            `json:"version"`
	Labels    []string       `json:"labels,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// VersionEntry represents a single version in document history
type VersionEntry struct {
	Version       int       `json:"version"`
	ChangedAt     time.Time `json:"changed_at"`
	ChangedBy     string    `json:"changed_by"`
	ChangeSummary string    `json:"change_summary"`
	ShardID       string    `json:"shard_id"`
}

// UpdateResult represents the result of an update or append operation
type UpdateResult struct {
	ID                string `json:"id"`
	Version           int    `json:"version"`
	PreviousVersionID string `json:"previous_version_id"`
	Summary           string `json:"summary"`
}

// ValidateDocType checks if a doc_type is valid
func ValidateDocType(docType string) error {
	for _, valid := range ValidDocTypes {
		if docType == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid doc_type '%s'. Valid types: %s", docType, strings.Join(ValidDocTypes, ", "))
}

// CreateKnowledgeDoc creates a new knowledge document
func (c *Client) CreateKnowledgeDoc(ctx context.Context, title, content, docType string, labels []string) (string, error) {
	meta := map[string]any{
		"doc_type":            docType,
		"version":             1,
		"last_changed_by":     c.Config.Agent,
		"last_change_summary": "Initial document",
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %v", err)
	}

	return c.CreateShardWithMetadata(ctx, title, content, "knowledge", nil, labels, json.RawMessage(metaJSON))
}

// ListKnowledgeDocs lists knowledge documents with optional doc_type filter
func (c *Client) ListKnowledgeDocs(ctx context.Context, docTypeFilter string, limit int) ([]KnowledgeDoc, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	query := `
		SELECT s.id, s.title, COALESCE(s.metadata->>'doc_type', ''),
			COALESCE((s.metadata->>'version')::int, 1),
			s.created_at, s.updated_at
		FROM shards s
		WHERE s.project = $1 AND s.type = 'knowledge' AND s.status = 'open'
	`
	args := []any{c.Config.Project}
	paramIdx := 2

	if docTypeFilter != "" {
		query += fmt.Sprintf(` AND s.metadata @> $%d::jsonb`, paramIdx)
		filterJSON, _ := json.Marshal(map[string]string{"doc_type": docTypeFilter})
		args = append(args, string(filterJSON))
		paramIdx++
	}

	query += fmt.Sprintf(` ORDER BY s.updated_at DESC LIMIT $%d`, paramIdx)
	args = append(args, limit)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list knowledge docs: %v", err)
	}
	defer rows.Close()

	var docs []KnowledgeDoc
	for rows.Next() {
		var d KnowledgeDoc
		if err := rows.Scan(&d.ID, &d.Title, &d.DocType, &d.Version,
			&d.CreatedAt, &d.UpdatedAt); err != nil {
			continue
		}
		docs = append(docs, d)
	}
	return docs, nil
}

// ShowKnowledgeDoc fetches a knowledge document by ID with full detail
func (c *Client) ShowKnowledgeDoc(ctx context.Context, id string) (*KnowledgeDoc, error) {
	shard, err := c.GetShard(ctx, id)
	if err != nil {
		return nil, err
	}
	if shard.Type != "knowledge" {
		return nil, fmt.Errorf("shard %s is type '%s', expected 'knowledge'", id, shard.Type)
	}

	doc := &KnowledgeDoc{
		ID:        shard.ID,
		Title:     shard.Title,
		Content:   shard.Content,
		Labels:    shard.Labels,
		CreatedAt: shard.CreatedAt,
		UpdatedAt: shard.UpdatedAt,
	}

	// Parse metadata
	if shard.Metadata != nil {
		var meta map[string]any
		if err := json.Unmarshal(shard.Metadata, &meta); err == nil {
			doc.Metadata = meta
			if dt, ok := meta["doc_type"].(string); ok {
				doc.DocType = dt
			}
			if v, ok := meta["version"].(float64); ok {
				doc.Version = int(v)
			}
		}
	}

	return doc, nil
}

// GetKnowledgeVersion fetches the content at a specific version number
func (c *Client) GetKnowledgeVersion(ctx context.Context, id string, version int) (*KnowledgeDoc, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var doc KnowledgeDoc
	var metadata json.RawMessage
	err = conn.QueryRow(ctx,
		`SELECT shard_id, version, title, content, metadata, created_at FROM knowledge_version($1, $2, $3)`,
		id, version, c.Config.Project,
	).Scan(&doc.ID, &doc.Version, &doc.Title, &doc.Content, &metadata, &doc.CreatedAt)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "Version") {
			return nil, fmt.Errorf("%s", extractPgMessage(errMsg))
		}
		return nil, fmt.Errorf("get knowledge version: %w", err)
	}

	if metadata != nil {
		var meta map[string]any
		if json.Unmarshal(metadata, &meta) == nil {
			doc.Metadata = meta
			if dt, ok := meta["doc_type"].(string); ok {
				doc.DocType = dt
			}
		}
	}

	// Fetch labels for the shard
	labelRows, err := conn.Query(ctx, `SELECT label FROM labels WHERE shard_id = $1 ORDER BY label`, doc.ID)
	if err == nil {
		defer labelRows.Close()
		for labelRows.Next() {
			var label string
			if labelRows.Scan(&label) == nil {
				doc.Labels = append(doc.Labels, label)
			}
		}
	}

	return &doc, nil
}

// UpdateKnowledgeDoc calls the update_knowledge_doc SQL function
func (c *Client) UpdateKnowledgeDoc(ctx context.Context, id, content, summary string) (*UpdateResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var result UpdateResult
	err = conn.QueryRow(ctx,
		`SELECT shard_id, version FROM update_knowledge_doc($1, $2, $3, $4, $5)`,
		id, content, summary, c.Config.Agent, c.Config.Project,
	).Scan(&result.ID, &result.Version)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return nil, fmt.Errorf("knowledge document %s not found", id)
		}
		if strings.Contains(errMsg, "identical") {
			return nil, fmt.Errorf("content is identical to current version")
		}
		if strings.Contains(errMsg, "closed") {
			return nil, fmt.Errorf("%s", extractPgMessage(errMsg))
		}
		return nil, fmt.Errorf("update knowledge doc: %w", err)
	}

	result.PreviousVersionID = fmt.Sprintf("%s-v%d", id, result.Version-1)
	result.Summary = summary

	// Regenerate embedding (non-fatal)
	c.tryEmbed(ctx, id, "knowledge", "", content)

	return &result, nil
}

// AppendKnowledgeDoc calls the append_knowledge_doc SQL function
func (c *Client) AppendKnowledgeDoc(ctx context.Context, id, content, summary string) (*UpdateResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var result UpdateResult
	err = conn.QueryRow(ctx,
		`SELECT shard_id, version FROM append_knowledge_doc($1, $2, $3, $4, $5)`,
		id, content, summary, c.Config.Agent, c.Config.Project,
	).Scan(&result.ID, &result.Version)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return nil, fmt.Errorf("knowledge document %s not found", id)
		}
		if strings.Contains(errMsg, "closed") {
			return nil, fmt.Errorf("%s", extractPgMessage(errMsg))
		}
		return nil, fmt.Errorf("append knowledge doc: %w", err)
	}

	result.PreviousVersionID = fmt.Sprintf("%s-v%d", id, result.Version-1)
	result.Summary = summary

	// Fetch updated content for embedding
	shardType, title, fullContent, fetchErr := c.GetShardContentForEmbedding(ctx, id)
	if fetchErr != nil {
		log.Printf("warning: failed to fetch content for embedding: %v", fetchErr)
	} else {
		c.tryEmbed(ctx, id, shardType, title, fullContent)
	}

	return &result, nil
}

// KnowledgeHistory returns the version history for a knowledge document
func (c *Client) KnowledgeHistory(ctx context.Context, id string) ([]VersionEntry, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		`SELECT version, changed_at, changed_by, change_summary, shard_id FROM knowledge_history($1, $2)`,
		id, c.Config.Project,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge history: %v", err)
	}
	defer rows.Close()

	var entries []VersionEntry
	for rows.Next() {
		var e VersionEntry
		if err := rows.Scan(&e.Version, &e.ChangedAt, &e.ChangedBy, &e.ChangeSummary, &e.ShardID); err != nil {
			return nil, fmt.Errorf("failed to scan history entry: %v", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history iteration error: %v", err)
	}

	return entries, nil
}

// DiffVersions produces a unified diff between two versions of a knowledge document
func (c *Client) DiffVersions(ctx context.Context, id string, from, to int) (string, error) {
	fromDoc, err := c.GetKnowledgeVersion(ctx, id, from)
	if err != nil {
		return "", fmt.Errorf("fetch version %d: %w", from, err)
	}
	toDoc, err := c.GetKnowledgeVersion(ctx, id, to)
	if err != nil {
		return "", fmt.Errorf("fetch version %d: %w", to, err)
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(fromDoc.Content),
		B:        difflib.SplitLines(toDoc.Content),
		FromFile: fmt.Sprintf("%s v%d", id, from),
		ToFile:   fmt.Sprintf("%s v%d", id, to),
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}

// extractPgMessage extracts the message from a PostgreSQL error string.
// pgx errors look like: "ERROR: Knowledge document pf-xxx is closed... (SQLSTATE P0001)"
func extractPgMessage(errMsg string) string {
	// Strip "ERROR: " prefix if present
	msg := errMsg
	if idx := strings.Index(msg, "ERROR: "); idx >= 0 {
		msg = msg[idx+7:]
	}
	// Strip trailing "(SQLSTATE ...)" if present
	if idx := strings.LastIndex(msg, " (SQLSTATE"); idx >= 0 {
		msg = msg[:idx]
	}
	return msg
}
