package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// GetMetadata returns the full metadata JSONB for a shard
func (c *Client) GetMetadata(ctx context.Context, id string) (json.RawMessage, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var meta json.RawMessage
	err = conn.QueryRow(ctx, `
		SELECT COALESCE(metadata, '{}') FROM shards WHERE id = $1
	`, id).Scan(&meta)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("shard not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %v", err)
	}
	return meta, nil
}

// GetMetadataField extracts a nested value from shard metadata using a path array.
// Uses the #>> operator for safe parameterized path navigation.
func (c *Client) GetMetadataField(ctx context.Context, id string, path []string) (string, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	if len(path) == 0 {
		return "", fmt.Errorf("empty field path")
	}

	var value *string
	err = conn.QueryRow(ctx, `
		SELECT COALESCE(metadata, '{}') #>> $2 FROM shards WHERE id = $1
	`, id, path).Scan(&value)

	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("shard not found: %s", id)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get metadata field: %v", err)
	}
	if value == nil {
		return "", fmt.Errorf("field not found: %s", strings.Join(path, "."))
	}
	return *value, nil
}

// UpdateMetadata merges the given JSONB patch into the shard's metadata
func (c *Client) UpdateMetadata(ctx context.Context, id string, patch json.RawMessage) (json.RawMessage, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var result json.RawMessage
	err = conn.QueryRow(ctx, `SELECT update_metadata($1, $2)`, id, patch).Scan(&result)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("shard not found: %s", id)
		}
		return nil, fmt.Errorf("failed to update metadata: %v", err)
	}
	return result, nil
}

// SetMetadataPath sets a nested value in the shard's metadata using a path array
func (c *Client) SetMetadataPath(ctx context.Context, id string, path []string, value json.RawMessage) (json.RawMessage, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var result json.RawMessage
	err = conn.QueryRow(ctx, `SELECT set_metadata_path($1, $2, $3)`, id, path, value).Scan(&result)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("shard not found: %s", id)
		}
		return nil, fmt.Errorf("failed to set metadata path: %v", err)
	}
	return result, nil
}

// DeleteMetadataKey removes a top-level key from the shard's metadata
func (c *Client) DeleteMetadataKey(ctx context.Context, id, key string) (json.RawMessage, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var result json.RawMessage
	err = conn.QueryRow(ctx, `SELECT delete_metadata_key($1, $2)`, id, key).Scan(&result)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("shard not found: %s", id)
		}
		return nil, fmt.Errorf("failed to delete metadata key: %v", err)
	}
	return result, nil
}

// QueryByMetadata queries shards by type and/or metadata containment
func (c *Client) QueryByMetadata(ctx context.Context, shardType string, metaFilters map[string]interface{}, limit int) ([]Shard, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	query := `
		SELECT id, project, title, COALESCE(LEFT(content, 200), ''), type, status,
			priority, creator, owner, created_at, updated_at,
			COALESCE(metadata, '{}')
		FROM shards WHERE project = $1
	`
	args := []interface{}{c.Config.Project}
	paramIdx := 2

	if shardType != "" {
		query += fmt.Sprintf(` AND type = $%d`, paramIdx)
		args = append(args, shardType)
		paramIdx++
	}

	if len(metaFilters) > 0 {
		filterJSON, err := json.Marshal(metaFilters)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata filter: %v", err)
		}
		query += fmt.Sprintf(` AND metadata @> $%d`, paramIdx)
		args = append(args, string(filterJSON))
		paramIdx++
	}

	query += fmt.Sprintf(` ORDER BY priority NULLS LAST, created_at DESC LIMIT $%d`, paramIdx)
	args = append(args, limit)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query shards by metadata: %v", err)
	}
	defer rows.Close()

	var shards []Shard
	for rows.Next() {
		var s Shard
		if err := rows.Scan(&s.ID, &s.Project, &s.Title, &s.Content, &s.Type, &s.Status,
			&s.Priority, &s.Creator, &s.Owner, &s.CreatedAt, &s.UpdatedAt,
			&s.Metadata); err != nil {
			continue
		}
		shards = append(shards, s)
	}
	return shards, nil
}

// parseDotPath splits a dot-separated path into components
// e.g. "test_coverage.unit" -> ["test_coverage", "unit"]
func parseDotPath(s string) []string {
	return strings.Split(s, ".")
}

// parseMetaFlag parses a --meta flag value.
// Accepts either "key=value" or raw JSON object.
func parseMetaFlag(s string) (map[string]interface{}, error) {
	s = strings.TrimSpace(s)

	// Try JSON first
	if strings.HasPrefix(s, "{") {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(s), &result); err != nil {
			return nil, fmt.Errorf("invalid JSON: %v", err)
		}
		return result, nil
	}

	// Try key=value
	parts := strings.SplitN(s, "=", 2)
	if len(parts) == 2 && parts[0] != "" {
		return map[string]interface{}{
			parts[0]: inferJSONValue(parts[1]),
		}, nil
	}

	return nil, fmt.Errorf("invalid metadata format. Use key=value or JSON")
}

// inferJSONValue detects the type of a CLI string value.
// Returns numbers, bools, or strings as appropriate.
func inferJSONValue(s string) interface{} {
	// Try integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	// Try float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// Try bool
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	// Try JSON object/array
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		var v interface{}
		if json.Unmarshal([]byte(s), &v) == nil {
			return v
		}
	}
	// Default to string
	return s
}

