package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Artifact operations",
	Long:  `Commands for adding artifacts to tasks.`,
}

var artifactAddCmd = &cobra.Command{
	Use:   "add <shard-id> <type> <reference> <description>",
	Short: "Add an artifact to a task",
	Long: `Add an artifact to a task.

Artifact types: commit, file, pr, url, deploy, or any custom type.`,
	Args: cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		shardID := args[0]
		artifactType := args[1]
		reference := args[2]
		description := args[3]

		err := addArtifactDB(ctx, shardID, artifactType, reference, description)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Printf(`{"success": true, "task_id": "%s", "type": "%s", "reference": "%s"}`+"\n",
				shardID, artifactType, reference)
			return nil
		}

		fmt.Printf("Added %s artifact to %s: %s\n", artifactType, shardID, reference)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(artifactCmd)
	artifactCmd.AddCommand(artifactAddCmd)

	artifactAddCmd.Example = `  palace artifact add pf-123 commit abc123 "Fixed null pointer bug"
  palace artifact add pf-123 file services/oauth.go "Modified refresh logic"
  palace artifact add pf-123 pr https://github.com/org/repo/pull/42 "PR link"`
}
