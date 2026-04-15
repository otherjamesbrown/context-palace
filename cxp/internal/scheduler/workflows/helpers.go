package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// getShardContent retrieves the body of a shard by running cxp shard show.
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
		return "", fmt.Errorf("parse shard %s: %w", shardID, err)
	}
	return shard.Content, nil
}
