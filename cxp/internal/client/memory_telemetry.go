package client

import (
	"context"
	"fmt"
)

// MemoryTouch increments access telemetry for a memory shard.
func (c *Client) MemoryTouch(ctx context.Context, memoryID string, agent string, depth int) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `SELECT memory_touch($1, $2, $3)`, memoryID, agent, depth)
	if err != nil {
		return fmt.Errorf("failed to touch memory %s: %v", memoryID, err)
	}
	return nil
}
