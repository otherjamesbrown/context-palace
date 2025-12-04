package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "cxp",
		Short:   "ContextPalace - AI coding agent memory management",
		Long:    "ContextPalace provides persistent, retrievable project context for AI coding agents",
		Version: version,
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}