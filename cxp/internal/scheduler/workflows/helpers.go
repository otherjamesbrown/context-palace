package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// getShardContent retrieves the body of a shard by running cxp shard show.
// The shard JSON output uses the "content" field (matching client.Shard.Content).
func getShardContent(ctx context.Context, shardID string) (string, error) {
	cmd := exec.CommandContext(ctx, "cxp", "shard", "show", shardID, "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cxp shard show %s: %w", shardID, err)
	}
	var shard struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(out, &shard); err != nil {
		// Fallback: return raw output so callers still have something to work with.
		return string(out), nil
	}
	return shard.Content, nil
}
