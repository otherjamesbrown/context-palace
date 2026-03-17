package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/tui"
	"github.com/spf13/cobra"
)

func main() {
	var includeClosed bool
	var configPath string

	rootCmd := &cobra.Command{
		Use:   "cxpv",
		Short: "Interactive TUI shard browser for Context Palace",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := client.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}

			c := client.NewClient(cfg)

			model := tui.NewBrowseModel(c, includeClosed)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
			return nil
		},
	}

	rootCmd.Flags().BoolVar(&includeClosed, "include-closed", false, "Include closed shards")
	rootCmd.Flags().StringVar(&configPath, "config", "", "Config file path (default ~/.cxp/config.yaml)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
