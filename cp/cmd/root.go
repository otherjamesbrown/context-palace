package cmd

import (
	"fmt"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/otherjamesbrown/context-palace/cp/internal/embedding"
	"github.com/otherjamesbrown/context-palace/cp/internal/generation"
	"github.com/spf13/cobra"
)

var (
	outputFormat   string
	projectFlag    string
	agentFlag      string
	limitFlag      int
	debugFlag      bool
	configFlag     string
	cpClient       *client.Client
)

var Version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "cp",
	Short: "Context Palace CLI",
	Long: `Context Palace — project-agnostic developer tooling for requirements management,
knowledge documents, semantic search, agent memory, and work tracking.

COMMANDS:
  status                             Connection + project info
  init                               Create .cp.yaml in current directory
  version                            CLI version

  memory add|list|search|resolve|defer   Agent memory
  backlog add|list|show|update|close     Dev backlog
  message send|inbox|show|read           Agent messaging
  session start|checkpoint|show|end      Work sessions
  context status|history|morning|project Project context
  task get|claim|progress|close          Task management
  artifact add                           Artifact tracking
  requirement create|list|show|approve|   Requirement lifecycle
              verify|reopen|link|unlink|
              dashboard
  knowledge create|list|show|update|     Knowledge documents
            append|history|diff
  recall "query"                         Semantic search
  epic create|show|list                  Epic management
  focus [set|clear]                      Active epic focus
  shard list|show|create|update|         Shard operations
        close|reopen|assign|next|board
  shard edges|link|unlink                Edge navigation & management
  shard label add|remove|list            Label management
  shard metadata get|set|delete          Shard metadata ops
  shard query                            Query by metadata
  admin embed-backfill                   Backfill embeddings

CONFIGURATION:
  Precedence: env vars > .cp.yaml > ~/.cp/config.yaml > defaults

  Environment variables:
    CP_HOST       Database host (default: dev02.brown.chat)
    CP_DATABASE   Database name (default: contextpalace)
    CP_USER       Database user
    CP_PROJECT    Project name
    CP_AGENT      Agent identity

EXAMPLES:
  cp status
  cp task get pf-123
  cp message inbox
  cp memory add "Lesson learned about timeouts"
  cp recall "pipeline timeout issues"
  cp --output json task get pf-123`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "init" {
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
		fmt.Printf("cp version %s\n", Version)
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
