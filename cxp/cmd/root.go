package cmd

import (
	"fmt"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/embedding"
	"github.com/otherjamesbrown/context-palace/cxp/internal/generation"
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	projectFlag  string
	agentFlag    string
	limitFlag    int
	debugFlag    bool
	configFlag   string
	cpClient     *client.Client
)

var Version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "cxp",
	Short: "Context Palace CLI",
	Long: `Context Palace — project-agnostic developer tooling for knowledge management,
semantic search, agent memory, and work tracking.

COMMANDS:
  status                                 Connection + project info
  version                                CLI version

  shard list|show|create|update|close|   Shard operations
        reopen|assign|next|board|status
  shard edges|link|unlink                Edge navigation & management
  shard label add|remove|list            Label management
  shard metadata get|set|delete          Shard metadata ops
  shard query                            Query by metadata

  design create                          Create a design (plan/feature)
  bug create                             Create a bug (defect)
  task get|create|claim|progress|close   Task management
  knowledge create|list|show|update|     Knowledge documents
            append|history|diff
  kb search|tree                         Knowledge base search & browse

  memory add|list|search|resolve|defer   Agent memory
  message send|inbox|show|read           Agent messaging
  recall "query"                         Semantic search

CONFIGURATION:
  Precedence: env vars > .cxp.yaml > ~/.cxp/config.yaml > defaults

  Environment variables:
    CXP_HOST      Database host (default: dev02.brown.chat)
    CXP_DATABASE  Database name (default: contextpalace)
    CXP_USER      Database user
    CXP_PROJECT   Project name
    CXP_AGENT     Agent identity

EXAMPLES:
  cxp status
  cxp design create "Auth redesign" --body "## Overview"
  cxp bug create "Missing names" --severity high
  cxp task create "Fix auth" --parent pf-xxx --assign mycroft
  cxp message inbox
  cxp recall "pipeline timeout issues"
  cxp --output json shard list`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		if cmd.Name() == "version" || (cmd.Name() == "init" && cmd.Parent() != nil && cmd.Parent().Name() == "cxp") {
			return nil
		}

		cfg, err := client.LoadConfig(configFlag)
		if err != nil {
			return err
		}

		// Apply flag overrides
		if projectFlag != "" {
			cfg.Project = projectFlag
		}
		if agentFlag != "" {
			cfg.Agent = agentFlag
		}

		// Apply default output format from config if -o not explicitly set
		if cfg.Defaults != nil && cfg.Defaults.Output != "" && !cmd.Flags().Changed("output") {
			outputFormat = cfg.Defaults.Output
		}

		cpClient = client.NewClient(cfg)

		// Initialize embedding provider (warn on failure, don't block)
		if cfg.Embedding != nil {
			provider, err := embedding.NewProvider(cfg.Embedding)
			if err != nil {
				if debugFlag {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: embedding provider init failed: %v\n", err)
				}
			} else {
				cpClient.EmbedProvider = provider
			}
		}

		// Initialize generation provider (warn on failure, don't block)
		if cfg.Generation != nil {
			gen, err := generation.NewGenerator(cfg.Generation)
			if err != nil {
				if debugFlag {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: generation provider init failed: %v\n", err)
				}
			} else {
				cpClient.Generator = gen
			}
		}

		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cxp version %s\n", Version)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text|json|yaml)")
	rootCmd.PersistentFlags().StringVar(&projectFlag, "project", "", "Override project from config")
	rootCmd.PersistentFlags().StringVar(&agentFlag, "agent", "", "Override agent identity")
	rootCmd.PersistentFlags().IntVar(&limitFlag, "limit", 20, "Pagination limit")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Verbose logging")
	rootCmd.PersistentFlags().StringVar(&configFlag, "config", "", "Override config file path")

	rootCmd.AddCommand(versionCmd)
}
