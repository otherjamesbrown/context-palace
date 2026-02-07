package client

import (
	"context"
	"fmt"
	"time"
)

// Message represents an inbox message
type Message struct {
	ID        string    `json:"id" yaml:"id"`
	Title     string    `json:"title" yaml:"title"`
	Creator   string    `json:"creator" yaml:"creator"`
	Kind      *string   `json:"kind,omitempty" yaml:"kind,omitempty"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	Content   string    `json:"content,omitempty" yaml:"content,omitempty"`
}

// SendMessage sends a message to recipients
func (c *Client) SendMessage(ctx context.Context, recipients []string, subject, body string, cc []string, kind string, replyTo string) (string, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	var ccArg interface{}
	if len(cc) > 0 {
		ccArg = cc
	}

	var kindArg interface{}
	if kind != "" {
		kindArg = kind
	}

	var replyToArg interface{}
	if replyTo != "" {
		replyToArg = replyTo
	}

	var newID string
	err = conn.QueryRow(ctx,
		`SELECT send_message($1, $2, $3, $4, $5, $6, $7, $8)`,
		c.Config.Project, c.Config.Agent, recipients, subject, body,
		ccArg, kindArg, replyToArg,
	).Scan(&newID)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %v", err)
	}
	return newID, nil
}

// GetInbox returns unread messages for the configured agent
func (c *Client) GetInbox(ctx context.Context) ([]Message, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		`SELECT id, title, creator, kind, created_at FROM unread_for($1, $2)`,
		c.Config.Project, c.Config.Agent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get inbox: %v", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Title, &m.Creator, &m.Kind, &m.CreatedAt); err != nil {
			continue
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// GetMessage fetches a message by ID with its content
func (c *Client) GetMessage(ctx context.Context, id string) (*Message, error) {
	shard, err := c.GetShard(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Message{
		ID:        shard.ID,
		Title:     shard.Title,
		Creator:   shard.Creator,
		Content:   shard.Content,
		CreatedAt: shard.CreatedAt,
	}, nil
}

// MarkRead marks messages as read for the configured agent
func (c *Client) MarkRead(ctx context.Context, shardIDs []string) (int, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx,
		`SELECT mark_read($1, $2)`,
		shardIDs, c.Config.Agent,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to mark as read: %v", err)
	}
	return count, nil
}
