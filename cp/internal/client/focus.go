package client

import (
	"context"
	"fmt"
	"time"
)

// Focus represents the current focus for an agent
type Focus struct {
	EpicID    string    `json:"epic_id"`
	EpicTitle string    `json:"epic_title"`
	SetAt     time.Time `json:"set_at"`
	Note      string    `json:"note,omitempty"`
}

// SetFocus sets the active epic for the current agent
func (c *Client) SetFocus(ctx context.Context, epicID string, note *string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	var noteArg interface{}
	if note != nil {
		noteArg = *note
	}

	_, err = conn.Exec(ctx, `SELECT focus_set($1, $2, $3, $4)`,
		c.Config.Project, c.Config.Agent, epicID, noteArg)
	if err != nil {
		return fmt.Errorf("%s", extractPgMessage(err.Error()))
	}
	return nil
}

// GetFocus returns the current focus for the agent, or nil if not set
func (c *Client) GetFocus(ctx context.Context) (*Focus, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var f Focus
	var note *string
	err = conn.QueryRow(ctx, `
		SELECT epic_id, epic_title, epic_status, set_at, note
		FROM focus_get($1, $2)
	`, c.Config.Project, c.Config.Agent).Scan(&f.EpicID, &f.EpicTitle, new(string), &f.SetAt, &note)
	if err != nil {
		// No rows = no focus set
		return nil, nil
	}
	if note != nil {
		f.Note = *note
	}
	return &f, nil
}

// ClearFocus removes the active focus for the current agent
func (c *Client) ClearFocus(ctx context.Context) (bool, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	var cleared bool
	err = conn.QueryRow(ctx, `SELECT focus_clear($1, $2)`,
		c.Config.Project, c.Config.Agent).Scan(&cleared)
	if err != nil {
		return false, fmt.Errorf("failed to clear focus: %v", err)
	}
	return cleared, nil
}
