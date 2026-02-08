package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	pgvec "github.com/pgvector/pgvector-go"

	"github.com/otherjamesbrown/context-palace/cp/internal/embedding"
	"github.com/otherjamesbrown/context-palace/cp/internal/pointer"
)

// MemoryTreeNode represents a node in the memory tree returned by memory_tree().
type MemoryTreeNode struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	ParentID     *string    `json:"parent_id,omitempty"`
	Depth        int        `json:"depth"`
	Status       string     `json:"status"`
	Labels       []string   `json:"labels,omitempty"`
	AccessCount  int        `json:"access_count"`
	LastAccessed *time.Time `json:"last_accessed,omitempty"`
	ChildCount   int        `json:"child_count"`
	Summary      *string    `json:"summary,omitempty"`
}

// MemoryChild represents a direct child returned by memory_children().
type MemoryChild struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Labels       []string   `json:"labels,omitempty"`
	AccessCount  int        `json:"access_count"`
	LastAccessed *time.Time `json:"last_accessed,omitempty"`
	ChildCount   int        `json:"child_count"`
	Content      string     `json:"content"`
}

// MemoryPathNode represents a node in the path from root to a memory.
type MemoryPathNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Depth int    `json:"depth"`
}

// MemoryHotResult represents a promotion candidate from memory_hot().
type MemoryHotResult struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Depth             int    `json:"depth"`
	AccessCount       int    `json:"access_count"`
	ParentID          string `json:"parent_id"`
	ParentTitle       string `json:"parent_title"`
	ParentAccessCount int    `json:"parent_access_count"`
}

// GetMemoryTree returns the memory hierarchy from memory_tree().
func (c *Client) GetMemoryTree(ctx context.Context, rootID *string) ([]MemoryTreeNode, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var rootArg any
	if rootID != nil {
		rootArg = *rootID
	}

	rows, err := conn.Query(ctx, `
		SELECT id, title, parent_id, depth, status, labels,
			access_count, last_accessed, child_count, summary
		FROM memory_tree($1, $2)
	`, c.Config.Project, rootArg)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory tree: %v", err)
	}
	defer rows.Close()

	var nodes []MemoryTreeNode
	for rows.Next() {
		var n MemoryTreeNode
		if err := rows.Scan(&n.ID, &n.Title, &n.ParentID, &n.Depth, &n.Status,
			&n.Labels, &n.AccessCount, &n.LastAccessed, &n.ChildCount, &n.Summary); err != nil {
			return nil, fmt.Errorf("failed to scan tree node: %v", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tree iteration error: %v", err)
	}
	return nodes, nil
}

// GetMemoryChildren returns direct children of a memory.
func (c *Client) GetMemoryChildren(ctx context.Context, parentID string) ([]MemoryChild, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, status, labels, access_count, last_accessed, child_count, content
		FROM memory_children($1, $2)
	`, c.Config.Project, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory children: %v", err)
	}
	defer rows.Close()

	var children []MemoryChild
	for rows.Next() {
		var ch MemoryChild
		if err := rows.Scan(&ch.ID, &ch.Title, &ch.Status, &ch.Labels,
			&ch.AccessCount, &ch.LastAccessed, &ch.ChildCount, &ch.Content); err != nil {
			return nil, fmt.Errorf("failed to scan memory child: %v", err)
		}
		children = append(children, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("children iteration error: %v", err)
	}
	return children, nil
}

// GetMemoryPath returns the path from root to a memory.
func (c *Client) GetMemoryPath(ctx context.Context, memoryID string) ([]MemoryPathNode, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `SELECT id, title, depth FROM memory_path($1)`, memoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory path: %v", err)
	}
	defer rows.Close()

	var path []MemoryPathNode
	for rows.Next() {
		var n MemoryPathNode
		if err := rows.Scan(&n.ID, &n.Title, &n.Depth); err != nil {
			return nil, fmt.Errorf("failed to scan path node: %v", err)
		}
		path = append(path, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("path iteration error: %v", err)
	}
	return path, nil
}

// GetMemoryHot returns promotion candidates (children accessed more than parent).
func (c *Client) GetMemoryHot(ctx context.Context, minDepth, limit int) ([]MemoryHotResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, depth, access_count, parent_id, parent_title, parent_access_count
		FROM memory_hot($1, $2, $3)
	`, c.Config.Project, minDepth, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory hot: %v", err)
	}
	defer rows.Close()

	var results []MemoryHotResult
	for rows.Next() {
		var r MemoryHotResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Depth, &r.AccessCount,
			&r.ParentID, &r.ParentTitle, &r.ParentAccessCount); err != nil {
			return nil, fmt.Errorf("failed to scan hot result: %v", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hot iteration error: %v", err)
	}
	return results, nil
}

// AddSubOpts holds options for AddSubMemory.
type AddSubOpts struct {
	Title   string
	Body    string
	Labels  []string
	Summary string
	Vector  []float32 // pre-computed embedding
}

// AddSubResult holds the result of AddSubMemory.
type AddSubResult struct {
	ChildID     string `json:"id"`
	Title       string `json:"title"`
	ParentID    string `json:"parent_id"`
	Summary     string `json:"summary"`
}

// AddSubMemory creates a child memory under parentID in an atomic transaction.
// Embedding should be pre-computed and passed in opts.Vector.
func (c *Client) AddSubMemory(ctx context.Context, parentID string, opts AddSubOpts) (*AddSubResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Lock parent shard
	var parentContent string
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(content, '') FROM shards WHERE id = $1 FOR UPDATE
	`, parentID).Scan(&parentContent)
	if err != nil {
		return nil, fmt.Errorf("failed to lock parent %s: %v", parentID, err)
	}

	// Create child shard
	labels := opts.Labels
	if labels == nil {
		labels = []string{}
	}

	var childID string
	err = tx.QueryRow(ctx, `
		SELECT create_shard($1, $2, $3, $4, 'memory', $5, $6, NULL, '{}')
	`, c.Config.Project, c.Config.Agent, opts.Title, opts.Body, labels, parentID).Scan(&childID)
	if err != nil {
		return nil, fmt.Errorf("failed to create child shard: %v", err)
	}

	// Store pre-computed embedding
	if opts.Vector != nil {
		vec := pgvec.NewVector(opts.Vector)
		_, err = tx.Exec(ctx, `UPDATE shards SET embedding = $1 WHERE id = $2`, vec, childID)
		if err != nil {
			return nil, fmt.Errorf("failed to store embedding: %v", err)
		}
	}

	// Create child-of edge with summary in metadata
	edgeMeta := fmt.Sprintf(`{"summary": %s}`, jsonQuote(opts.Summary))
	_, err = tx.Exec(ctx, `
		INSERT INTO edges (from_id, to_id, edge_type, metadata)
		VALUES ($1, $2, 'child-of', $3::jsonb)
	`, childID, parentID, edgeMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to create child-of edge: %v", err)
	}

	// Update parent content with pointer block entry
	newParentContent, err := pointer.AppendSubMemory(parentContent, pointer.SubMemoryEntry{
		ID:      childID,
		Title:   opts.Title,
		Summary: opts.Summary,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update pointer block: %v", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE shards SET content = $1, updated_at = now() WHERE id = $2
	`, newParentContent, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to update parent content: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit: %v", err)
	}

	return &AddSubResult{
		ChildID:  childID,
		Title:    opts.Title,
		ParentID: parentID,
		Summary:  opts.Summary,
	}, nil
}

// DeleteResult holds the result of DeleteMemory.
type DeleteResult struct {
	Deleted       []string `json:"deleted"`
	ParentUpdated *string  `json:"parent_updated,omitempty"`
}

// DeleteMemory deletes a memory shard, removes its pointer from parent, optionally recursive.
func (c *Client) DeleteMemory(ctx context.Context, memoryID string, recursive bool) (*DeleteResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Fetch the shard to check parent and children
	var parentID *string
	var title string
	err = conn.QueryRow(ctx, `
		SELECT title, parent_id FROM shards WHERE id = $1
	`, memoryID).Scan(&title, &parentID)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("shard %s not found", memoryID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shard: %v", err)
	}

	// Check for children
	var childCount int
	err = conn.QueryRow(ctx, `
		SELECT count(*) FROM shards WHERE parent_id = $1 AND type = 'memory'
	`, memoryID).Scan(&childCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count children: %v", err)
	}

	if childCount > 0 && !recursive {
		return nil, fmt.Errorf("memory has %d children. Use --recursive to delete subtree, or move children first", childCount)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	var deleted []string

	// If recursive, collect and delete descendants depth-first
	if recursive && childCount > 0 {
		descendants, err := c.collectDescendants(ctx, conn, memoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to collect descendants: %v", err)
		}
		// Delete in reverse order (deepest first)
		for i := len(descendants) - 1; i >= 0; i-- {
			_, err = tx.Exec(ctx, `DELETE FROM shards WHERE id = $1`, descendants[i])
			if err != nil {
				return nil, fmt.Errorf("failed to delete %s: %v", descendants[i], err)
			}
			deleted = append(deleted, descendants[i])
		}
	}

	// If has parent, lock parent and remove pointer
	if parentID != nil && *parentID != "" {
		var parentContent string
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(content, '') FROM shards WHERE id = $1 FOR UPDATE
		`, *parentID).Scan(&parentContent)
		if err != nil {
			return nil, fmt.Errorf("failed to lock parent: %v", err)
		}

		newContent, err := pointer.RemoveSubMemory(parentContent, memoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to update parent pointer block: %v", err)
		}

		_, err = tx.Exec(ctx, `
			UPDATE shards SET content = $1, updated_at = now() WHERE id = $2
		`, newContent, *parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to update parent content: %v", err)
		}
	}

	// Delete the shard itself (CASCADE removes edges)
	_, err = tx.Exec(ctx, `DELETE FROM shards WHERE id = $1`, memoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete %s: %v", memoryID, err)
	}
	deleted = append(deleted, memoryID)

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit: %v", err)
	}

	result := &DeleteResult{Deleted: deleted}
	if parentID != nil && *parentID != "" {
		result.ParentUpdated = parentID
	}
	return result, nil
}

// collectDescendants returns all descendant IDs of a memory (breadth-first order).
func (c *Client) collectDescendants(ctx context.Context, conn *pgx.Conn, parentID string) ([]string, error) {
	var all []string
	queue := []string{parentID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		rows, err := conn.Query(ctx, `
			SELECT id FROM shards WHERE parent_id = $1 AND type = 'memory'
		`, current)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var childID string
			if rows.Scan(&childID) == nil {
				all = append(all, childID)
				queue = append(queue, childID)
			}
		}
		rows.Close()
	}
	return all, nil
}

// MoveResult holds the result of MoveMemory.
type MoveResult struct {
	ID        string  `json:"id"`
	OldParent *string `json:"old_parent,omitempty"`
	NewParent *string `json:"new_parent,omitempty"`
}

// MoveMemory re-parents a memory shard. If toRoot is true, the memory becomes a root.
func (c *Client) MoveMemory(ctx context.Context, memoryID string, newParentID string, toRoot bool) (*MoveResult, error) {
	if !toRoot && memoryID == newParentID {
		return nil, fmt.Errorf("cannot move memory to itself")
	}

	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Fetch current state
	var oldParentID *string
	var memTitle string
	err = conn.QueryRow(ctx, `
		SELECT title, parent_id FROM shards WHERE id = $1 AND type = 'memory'
	`, memoryID).Scan(&memTitle, &oldParentID)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("memory %s not found", memoryID)
	}
	if err != nil {
		return nil, err
	}

	// Check for cycles: new parent must not be a descendant of this memory
	if !toRoot {
		// Verify new parent exists and is a memory
		var newParentType string
		err = conn.QueryRow(ctx, `SELECT type FROM shards WHERE id = $1`, newParentID).Scan(&newParentType)
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("target %s not found", newParentID)
		}
		if newParentType != "memory" {
			return nil, fmt.Errorf("target %s is type '%s', expected 'memory'", newParentID, newParentType)
		}

		path, err := c.GetMemoryPath(ctx, newParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to check for cycles: %v", err)
		}
		for _, node := range path {
			if node.ID == memoryID {
				return nil, fmt.Errorf("cannot move to own descendant (would create cycle)")
			}
		}
	}

	// Get existing summary from old parent's pointer block (to preserve it)
	var existingSummary string
	if oldParentID != nil && *oldParentID != "" {
		var oldParentContent string
		_ = conn.QueryRow(ctx, `SELECT COALESCE(content, '') FROM shards WHERE id = $1`, *oldParentID).Scan(&oldParentContent)
		_, entries, _ := pointer.ParseSubMemories(oldParentContent)
		for _, e := range entries {
			if e.ID == memoryID {
				existingSummary = e.Summary
				break
			}
		}
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Lock parents in ID order to prevent deadlocks
	lockIDs := []string{}
	if oldParentID != nil && *oldParentID != "" {
		lockIDs = append(lockIDs, *oldParentID)
	}
	if !toRoot {
		lockIDs = append(lockIDs, newParentID)
	}
	// Sort and lock
	sortStrings(lockIDs)
	for _, id := range lockIDs {
		var dummy string
		_ = tx.QueryRow(ctx, `SELECT id FROM shards WHERE id = $1 FOR UPDATE`, id).Scan(&dummy)
	}

	// Remove from old parent's pointer block
	if oldParentID != nil && *oldParentID != "" {
		var oldContent string
		_ = tx.QueryRow(ctx, `SELECT COALESCE(content, '') FROM shards WHERE id = $1`, *oldParentID).Scan(&oldContent)
		newContent, err := pointer.RemoveSubMemory(oldContent, memoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to remove old pointer: %v", err)
		}
		_, err = tx.Exec(ctx, `UPDATE shards SET content = $1, updated_at = now() WHERE id = $2`, newContent, *oldParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to update old parent: %v", err)
		}

		// Delete old child-of edge
		_, _ = tx.Exec(ctx, `DELETE FROM edges WHERE from_id = $1 AND to_id = $2 AND edge_type = 'child-of'`, memoryID, *oldParentID)
	}

	if toRoot {
		// Move to root
		_, err = tx.Exec(ctx, `UPDATE shards SET parent_id = NULL, updated_at = now() WHERE id = $1`, memoryID)
	} else {
		// Set new parent_id
		_, err = tx.Exec(ctx, `UPDATE shards SET parent_id = $1, updated_at = now() WHERE id = $2`, newParentID, memoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to update parent_id: %v", err)
		}

		// Add to new parent's pointer block
		var newParentContent string
		_ = tx.QueryRow(ctx, `SELECT COALESCE(content, '') FROM shards WHERE id = $1`, newParentID).Scan(&newParentContent)
		summary := existingSummary
		if summary == "" {
			summary = "No summary — update manually"
		}
		newContent, err := pointer.AppendSubMemory(newParentContent, pointer.SubMemoryEntry{
			ID:      memoryID,
			Title:   memTitle,
			Summary: summary,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to add pointer to new parent: %v", err)
		}
		_, err = tx.Exec(ctx, `UPDATE shards SET content = $1, updated_at = now() WHERE id = $2`, newContent, newParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to update new parent: %v", err)
		}

		// Create new child-of edge with same summary
		edgeMeta := fmt.Sprintf(`{"summary": %s}`, jsonQuote(summary))
		_, err = tx.Exec(ctx, `
			INSERT INTO edges (from_id, to_id, edge_type, metadata)
			VALUES ($1, $2, 'child-of', $3::jsonb)
		`, memoryID, newParentID, edgeMeta)
		if err != nil {
			return nil, fmt.Errorf("failed to create new child-of edge: %v", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update parent_id: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit: %v", err)
	}

	result := &MoveResult{ID: memoryID, OldParent: oldParentID}
	if !toRoot {
		result.NewParent = &newParentID
	}
	return result, nil
}

// PromoteResult holds the result of PromoteMemory.
type PromoteResult struct {
	ID        string  `json:"id"`
	OldParent *string `json:"old_parent,omitempty"`
	NewParent *string `json:"new_parent,omitempty"`
	NewDepth  int     `json:"new_depth"`
}

// PromoteMemory moves a memory up one level in the hierarchy.
func (c *Client) PromoteMemory(ctx context.Context, memoryID string) (*PromoteResult, error) {
	// Get current path to determine parent and grandparent
	path, err := c.GetMemoryPath(ctx, memoryID)
	if err != nil {
		return nil, err
	}

	if len(path) == 0 {
		return nil, fmt.Errorf("memory %s not found", memoryID)
	}

	// Find the memory's position in the path
	var myDepth int
	for _, node := range path {
		if node.ID == memoryID {
			myDepth = node.Depth
			break
		}
	}

	if myDepth == 0 {
		return nil, fmt.Errorf("memory is already at root level")
	}

	// Find grandparent (parent of parent)
	if myDepth == 1 {
		// Parent is root, promote to root
		moveResult, err := c.MoveMemory(ctx, memoryID, "", true)
		if err != nil {
			return nil, err
		}
		return &PromoteResult{
			ID:        memoryID,
			OldParent: moveResult.OldParent,
			NewParent: nil,
			NewDepth:  0,
		}, nil
	}

	// Find grandparent ID (depth = myDepth - 2)
	var grandparentID string
	for _, node := range path {
		if node.Depth == myDepth-2 {
			grandparentID = node.ID
			break
		}
	}

	moveResult, err := c.MoveMemory(ctx, memoryID, grandparentID, false)
	if err != nil {
		return nil, err
	}
	return &PromoteResult{
		ID:        memoryID,
		OldParent: moveResult.OldParent,
		NewParent: &grandparentID,
		NewDepth:  myDepth - 1,
	}, nil
}

// SyncDiscrepancy describes a pointer/graph mismatch found by sync.
type SyncDiscrepancy struct {
	ParentID   string `json:"parent"`
	Type       string `json:"type"` // "missing_pointer", "stale_pointer", "edge_mismatch"
	ChildID    string `json:"child,omitempty"`
	ChildTitle string `json:"child_title,omitempty"`
}

// SyncResult holds the result of SyncMemoryPointers.
type SyncResult struct {
	ParentsChecked int               `json:"parents_checked"`
	Discrepancies  []SyncDiscrepancy `json:"discrepancies"`
	Fixed          bool              `json:"fixed"`
}

// SyncMemoryPointers reconciles pointer blocks with actual graph.
func (c *Client) SyncMemoryPointers(ctx context.Context, parentID *string, dryRun bool) (*SyncResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Find all parents to check
	var query string
	var args []any
	if parentID != nil {
		query = `SELECT id, COALESCE(content, '') FROM shards WHERE id = $1 AND type = 'memory'`
		args = []any{*parentID}
	} else {
		query = `
			SELECT DISTINCT s.id, COALESCE(s.content, '')
			FROM shards s
			WHERE s.project = $1 AND s.type = 'memory' AND s.status != 'closed'
			AND (
				EXISTS (SELECT 1 FROM shards c WHERE c.parent_id = s.id AND c.type = 'memory')
				OR s.content LIKE '%<!-- sub-memories -->%'
			)
		`
		args = []any{c.Config.Project}
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query parents: %v", err)
	}
	defer rows.Close()

	type parentInfo struct {
		id      string
		content string
	}
	var parents []parentInfo
	for rows.Next() {
		var p parentInfo
		if rows.Scan(&p.id, &p.content) == nil {
			parents = append(parents, p)
		}
	}
	rows.Close()

	result := &SyncResult{ParentsChecked: len(parents)}
	var discs []SyncDiscrepancy

	for _, parent := range parents {
		// Parse pointer block
		_, blockEntries, _ := pointer.ParseSubMemories(parent.content)
		blockMap := map[string]pointer.SubMemoryEntry{}
		for _, e := range blockEntries {
			blockMap[e.ID] = e
		}

		// Get actual children from DB
		childRows, err := conn.Query(ctx, `
			SELECT s.id, s.title, COALESCE(e.metadata->>'summary', '') as summary
			FROM shards s
			LEFT JOIN edges e ON e.from_id = s.id AND e.to_id = $1 AND e.edge_type = 'child-of'
			WHERE s.parent_id = $1 AND s.type = 'memory' AND s.status != 'closed'
			ORDER BY s.created_at
		`, parent.id)
		if err != nil {
			continue
		}

		type childInfo struct {
			id, title, edgeSummary string
		}
		var actualChildren []childInfo
		actualMap := map[string]childInfo{}
		for childRows.Next() {
			var ch childInfo
			if childRows.Scan(&ch.id, &ch.title, &ch.edgeSummary) == nil {
				actualChildren = append(actualChildren, ch)
				actualMap[ch.id] = ch
			}
		}
		childRows.Close()

		// Find missing pointers (child exists but not in block)
		for _, ch := range actualChildren {
			if _, found := blockMap[ch.id]; !found {
				discs = append(discs, SyncDiscrepancy{
					ParentID:   parent.id,
					Type:       "missing_pointer",
					ChildID:    ch.id,
					ChildTitle: ch.title,
				})
			}
		}

		// Find stale pointers (in block but shard doesn't exist)
		for _, entry := range blockEntries {
			if _, found := actualMap[entry.ID]; !found {
				discs = append(discs, SyncDiscrepancy{
					ParentID: parent.id,
					Type:     "stale_pointer",
					ChildID:  entry.ID,
				})
			}
		}
	}

	result.Discrepancies = discs

	// Fix if not dry-run and there are discrepancies
	if !dryRun && len(discs) > 0 {
		fixConn, err := c.Connect(ctx)
		if err != nil {
			return nil, err
		}
		defer fixConn.Close(ctx)

		// Group discrepancies by parent
		parentDiscs := map[string][]SyncDiscrepancy{}
		for _, d := range discs {
			parentDiscs[d.ParentID] = append(parentDiscs[d.ParentID], d)
		}

		for pid, pDiscs := range parentDiscs {
			tx, err := fixConn.Begin(ctx)
			if err != nil {
				continue
			}

			var content string
			err = tx.QueryRow(ctx, `SELECT COALESCE(content, '') FROM shards WHERE id = $1 FOR UPDATE`, pid).Scan(&content)
			if err != nil {
				tx.Rollback(ctx)
				continue
			}

			_, currentEntries, _ := pointer.ParseSubMemories(content)

			for _, d := range pDiscs {
				switch d.Type {
				case "missing_pointer":
					// Get summary from edge metadata if available
					summary := "No summary — update this memory's pointer block manually or re-add with add-sub"
					var edgeSummary *string
					_ = tx.QueryRow(ctx, `
						SELECT metadata->>'summary' FROM edges
						WHERE from_id = $1 AND to_id = $2 AND edge_type = 'child-of'
					`, d.ChildID, pid).Scan(&edgeSummary)
					if edgeSummary != nil && *edgeSummary != "" {
						summary = *edgeSummary
					}

					currentEntries = append(currentEntries, pointer.SubMemoryEntry{
						ID:      d.ChildID,
						Title:   d.ChildTitle,
						Summary: summary,
					})

					// Ensure edge metadata matches
					edgeMeta := fmt.Sprintf(`{"summary": %s}`, jsonQuote(summary))
					_, _ = tx.Exec(ctx, `
						INSERT INTO edges (from_id, to_id, edge_type, metadata)
						VALUES ($1, $2, 'child-of', $3::jsonb)
						ON CONFLICT (from_id, to_id, edge_type) DO UPDATE SET metadata = $3::jsonb
					`, d.ChildID, pid, edgeMeta)

				case "stale_pointer":
					// Remove from entries
					var filtered []pointer.SubMemoryEntry
					for _, e := range currentEntries {
						if e.ID != d.ChildID {
							filtered = append(filtered, e)
						}
					}
					currentEntries = filtered
				}
			}

			// Write back
			newContent, err := pointer.ReplaceSubMemories(content, currentEntries)
			if err != nil {
				tx.Rollback(ctx)
				continue
			}

			_, _ = tx.Exec(ctx, `UPDATE shards SET content = $1, updated_at = now() WHERE id = $2`, newContent, pid)
			tx.Commit(ctx)
		}

		result.Fixed = true
	}

	return result, nil
}

// PrecomputeEmbedding generates an embedding vector for the given content.
// Returns nil vector if embedding is not configured (non-fatal).
func (c *Client) PrecomputeEmbedding(ctx context.Context, title, body string) []float32 {
	if c.EmbedProvider == nil {
		return nil
	}
	text := embedding.BuildEmbeddingText("memory", title, body)
	if text == "" {
		return nil
	}
	vec, _ := c.EmbedProvider.Embed(ctx, text)
	return vec
}

// jsonQuote returns a JSON-safe quoted string value.
func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// sortStrings sorts a string slice in place (simple insertion sort for small slices).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
