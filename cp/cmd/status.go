package cmd

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection and project info",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		counts, err := cpClient.GetShardCounts(ctx)
		if err != nil {
			return fmt.Errorf("Cannot connect to Context Palace at %s. Check config.",
				cpClient.Config.Connection.Host)
		}

		if outputFormat == "json" {
			type statusOutput struct {
				Host     string              `json:"host"`
				Database string              `json:"database"`
				Project  string              `json:"project"`
				Agent    string              `json:"agent"`
				Status   string              `json:"status"`
				Shards   *client.ShardCounts `json:"shards"`
			}
			out := statusOutput{
				Host:     cpClient.Config.Connection.Host,
				Database: cpClient.Config.Connection.Database,
				Project:  cpClient.Config.Project,
				Agent:    cpClient.Config.Agent,
				Status:   "connected",
				Shards:   counts,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Println("Context Palace")
		fmt.Printf("  Host:     %s\n", cpClient.Config.Connection.Host)
		fmt.Printf("  Database: %s\n", cpClient.Config.Connection.Database)
		fmt.Printf("  Project:  %s\n", cpClient.Config.Project)
		fmt.Printf("  Agent:    %s\n", cpClient.Config.Agent)
		fmt.Printf("  Status:   connected\n")
		fmt.Printf("  Shards:   %d (%d open, %d closed, %d other)\n",
			counts.Total, counts.Open, counts.Closed, counts.Other)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
