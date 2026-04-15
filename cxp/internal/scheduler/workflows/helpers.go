package workflows

import (
	"context"
	"encoding/json"
	"os/exec"
)

// getShardContent retrieves the body of a shard by running cxp shard show.
func getShardContent(ctx context.Context, shardID string) (string, error) {
	cmd := exec.CommandContext(ctx, "cxp", "shard", "show", shardID, "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var shard struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(out, &shard); err != nil {
		return "", err
	}
	return shard.Content, nil
}
