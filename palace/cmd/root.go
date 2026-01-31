package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	jsonOutput bool
	cfg        *Config
)

// Config holds the palace CLI configuration
type Config struct {
	Host     string `yaml:"host"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Project  string `yaml:"project"`
	Agent    string `yaml:"agent"`
}

// LoadConfig loads configuration from environment variables and config file
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Host:     "dev02.brown.chat",
		Database: "contextpalace",
		Project:  "penfold",
	}

	// Try to load from config file
	home, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(home, ".palace.yaml")
		if data, err := os.ReadFile(configPath); err == nil {
			yaml.Unmarshal(data, cfg)
		}
	}

	// Environment variables override config file
	if v := os.Getenv("PALACE_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("PALACE_DB"); v != "" {
		cfg.Database = v
	}
	if v := os.Getenv("PALACE_USER"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("PALACE_PROJECT"); v != "" {
		cfg.Project = v
	}
	if v := os.Getenv("PALACE_AGENT"); v != "" {
		cfg.Agent = v
	}

	// Validate required fields
	if cfg.User == "" {
		return nil, fmt.Errorf("PALACE_USER is required (set via environment or ~/.palace.yaml)")
	}
	if cfg.Agent == "" {
		return nil, fmt.Errorf("PALACE_AGENT is required (set via environment or ~/.palace.yaml)")
	}

	return cfg, nil
}

// ConnectionString returns the PostgreSQL connection string
func (c *Config) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s dbname=%s user=%s sslmode=verify-full",
		c.Host, c.Database, c.User,
	)
}

var rootCmd = &cobra.Command{
	Use:   "palace",
	Short: "Context-Palace CLI for sub-agents",
	Long:  `A simplified CLI for sub-agents to interact with context-palace tasks and artifacts.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = LoadConfig()
		if err != nil {
			return err
		}
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}
