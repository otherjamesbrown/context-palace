package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cxp",
	Short: "ContextPalace - AI coding agent memory management",
	Long:  "ContextPalace provides persistent, retrievable project context for AI coding agents",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion sets the version string.
func SetVersion(v string) {
	rootCmd.Version = v
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(memoCmd)
	rootCmd.AddCommand(memosCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(addTriggerCmd)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(logCmd)
}
