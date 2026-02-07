package cmd

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Project context",
	Long:  `Commands for viewing project context — status, history, morning briefing, and project overview.`,
}

var contextStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Project context overview",
	Example: "  cp context status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		conn, err := cpClient.Connect(ctx)
		if err != nil {
			return err
		}
		defer conn.Close(ctx)

		project := cpClient.Config.Project
		agent := cpClient.Config.Agent

		// Get shard counts
		counts, err := cpClient.GetShardCounts(ctx)
		if err != nil {
			return err
		}

		// Get unread message count
		messages, err := cpClient.GetInbox(ctx)
		if err != nil {
			messages = nil
		}

		// Get open task count
		var openTasks int
		conn.QueryRow(ctx, `
			SELECT count(*) FROM shards
			WHERE project = $1 AND type = 'task' AND owner = $2 AND status != 'closed'
		`, project, agent).Scan(&openTasks)

		if outputFormat == "json" {
			type contextStatus struct {
				Project      string              `json:"project"`
				Agent        string              `json:"agent"`
				Shards       *client.ShardCounts `json:"shards"`
				UnreadCount  int                 `json:"unread_count"`
				OpenTaskCount int                `json:"open_task_count"`
			}
			out := contextStatus{
				Project:      project,
				Agent:        agent,
				Shards:       counts,
				UnreadCount:  len(messages),
				OpenTaskCount: openTasks,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Project: %s\n", project)
		fmt.Printf("Agent:   %s\n", agent)
		fmt.Printf("Shards:  %d total (%d open, %d closed)\n", counts.Total, counts.Open, counts.Closed)
		fmt.Printf("Unread:  %d message(s)\n", len(messages))
		fmt.Printf("Tasks:   %d open\n", openTasks)

		return nil
	},
}

var contextHistoryCmd = &cobra.Command{
	Use:     "history",
	Short:   "Recent project activity",
	Example: "  cp context history",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		conn, err := cpClient.Connect(ctx)
		if err != nil {
			return err
		}
		defer conn.Close(ctx)

		rows, err := conn.Query(ctx, `
			SELECT id, type, title, creator, created_at
			FROM shards
			WHERE project = $1
			ORDER BY created_at DESC
			LIMIT $2
		`, cpClient.Config.Project, limitFlag)
		if err != nil {
			return fmt.Errorf("failed to get history: %v", err)
		}
		defer rows.Close()

		type historyEntry struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Title   string `json:"title"`
			Creator string `json:"creator"`
			Date    string `json:"created_at"`
		}

		var entries []historyEntry
		for rows.Next() {
			var e historyEntry
			var createdAt interface{}
			if err := rows.Scan(&e.ID, &e.Type, &e.Title, &e.Creator, &createdAt); err != nil {
				continue
			}
			if t, ok := createdAt.(interface{ Format(string) string }); ok {
				e.Date = t.Format("01-02 15:04")
			}
			entries = append(entries, e)
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(entries)
			fmt.Println(s)
			return nil
		}

		if len(entries) == 0 {
			fmt.Println("No recent activity.")
			return nil
		}

		tbl := client.NewTable("DATE", "TYPE", "CREATOR", "ID", "TITLE")
		for _, e := range entries {
			tbl.AddRow(e.Date, e.Type, client.Truncate(e.Creator, 15), e.ID, client.Truncate(e.Title, 40))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var contextMorningCmd = &cobra.Command{
	Use:   "morning",
	Short: "Morning briefing",
	Long:  `Shows unread messages, open tasks, and recent activity for a quick start to the day.`,
	Example: "  cp context morning",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		project := cpClient.Config.Project
		agent := cpClient.Config.Agent

		fmt.Printf("Good morning, %s!\n", agent)
		fmt.Printf("Project: %s\n\n", project)

		// Unread messages
		messages, err := cpClient.GetInbox(ctx)
		if err == nil && len(messages) > 0 {
			fmt.Printf("--- Unread Messages (%d) ---\n", len(messages))
			for _, m := range messages {
				fmt.Printf("  %s  %s  %s\n", m.ID, m.Creator, m.Title)
			}
			fmt.Println()
		} else {
			fmt.Println("--- No unread messages ---\n")
		}

		// Open tasks
		conn, err := cpClient.Connect(ctx)
		if err != nil {
			return err
		}
		defer conn.Close(ctx)

		rows, err := conn.Query(ctx, `
			SELECT id, title, priority, status FROM shards
			WHERE project = $1 AND type = 'task' AND owner = $2 AND status != 'closed'
			ORDER BY priority NULLS LAST, created_at
		`, project, agent)
		if err == nil {
			defer rows.Close()
			fmt.Println("--- Open Tasks ---")
			count := 0
			for rows.Next() {
				var id, title, status string
				var priority *int
				if rows.Scan(&id, &title, &priority, &status) == nil {
					pri := "-"
					if priority != nil {
						pri = fmt.Sprintf("%d", *priority)
					}
					fmt.Printf("  [%s] %s  %s  %s\n", pri, id, status, client.Truncate(title, 50))
					count++
				}
			}
			if count == 0 {
				fmt.Println("  (no open tasks)")
			}
			fmt.Println()
		}

		return nil
	},
}

var contextProjectCmd = &cobra.Command{
	Use:     "project",
	Short:   "Project overview",
	Example: "  cp context project",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		conn, err := cpClient.Connect(ctx)
		if err != nil {
			return err
		}
		defer conn.Close(ctx)

		project := cpClient.Config.Project

		// Count by type
		rows, err := conn.Query(ctx, `
			SELECT type, count(*), count(*) FILTER (WHERE status = 'open' OR status = 'in_progress')
			FROM shards WHERE project = $1
			GROUP BY type ORDER BY count(*) DESC
		`, project)
		if err != nil {
			return fmt.Errorf("failed to get project info: %v", err)
		}
		defer rows.Close()

		type typeStat struct {
			Type  string `json:"type"`
			Total int    `json:"total"`
			Open  int    `json:"open"`
		}
		var stats []typeStat
		for rows.Next() {
			var s typeStat
			if rows.Scan(&s.Type, &s.Total, &s.Open) == nil {
				stats = append(stats, s)
			}
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(stats)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Project: %s\n\n", project)
		tbl := client.NewTable("TYPE", "TOTAL", "OPEN")
		for _, s := range stats {
			tbl.AddRow(s.Type, fmt.Sprintf("%d", s.Total), fmt.Sprintf("%d", s.Open))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.AddCommand(contextStatusCmd)
	contextCmd.AddCommand(contextHistoryCmd)
	contextCmd.AddCommand(contextMorningCmd)
	contextCmd.AddCommand(contextProjectCmd)
}
