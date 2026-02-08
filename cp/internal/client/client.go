package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/context-palace/cp/internal/embedding"
	"github.com/otherjamesbrown/context-palace/cp/internal/generation"
)

// Config holds the cp CLI configuration
type Config struct {
	Connection ConnectionConfig              `yaml:"connection"`
	Agent      string                        `yaml:"agent"`
	Project    string                        `yaml:"project"`
	Embedding  *embedding.EmbeddingConfig    `yaml:"embedding,omitempty"`
	Generation *generation.GenerationConfig  `yaml:"generation,omitempty"`
}

// ConnectionConfig holds database connection settings
type ConnectionConfig struct {
	Host     string `yaml:"host"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	SSLMode  string `yaml:"sslmode"`
}

// Client provides database operations for Context Palace
type Client struct {
	Config        *Config
	EmbedProvider embedding.Provider
	Generator     generation.Generator
}

// NewClient creates a new client with the given config
func NewClient(cfg *Config) *Client {
	return &Client{Config: cfg}
}

// Connect opens a database connection
func (c *Client) Connect(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, c.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Context Palace at %s: %v", c.Config.Connection.Host, err)
	}
	// Best-effort pgvector type registration (silent failure if extension not installed)
	_ = pgxvec.RegisterTypes(ctx, conn)
	return conn, nil
}

// ConnectionString returns the PostgreSQL connection string
func (c *Client) ConnectionString() string {
	cfg := c.Config.Connection
	sslmode := cfg.SSLMode
	if sslmode == "" {
		sslmode = "verify-full"
	}
	return fmt.Sprintf(
		"host=%s dbname=%s user=%s sslmode=%s",
		cfg.Host, cfg.Database, cfg.User, sslmode,
	)
}

// LoadConfig loads configuration with precedence:
// env vars > .cp.yaml (project) > ~/.cp/config.yaml (global) > defaults
func LoadConfig(configOverride string) (*Config, error) {
	cfg := &Config{
		Connection: ConnectionConfig{
			Host:     "dev02.brown.chat",
			Database: "contextpalace",
			SSLMode:  "verify-full",
		},
	}

	// Load global config: ~/.cp/config.yaml
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".cp", "config.yaml")
		if configOverride != "" {
			globalPath = configOverride
		}
		loadYAML(globalPath, cfg)
	}

	// Load project config: walk up to find .cp.yaml
	if configOverride == "" {
		if projectPath := findProjectConfig(); projectPath != "" {
			loadProjectConfig(projectPath, cfg)
		}
	}

	// Environment variables override everything
	if v := os.Getenv("CP_HOST"); v != "" {
		cfg.Connection.Host = v
	}
	if v := os.Getenv("CP_DATABASE"); v != "" {
		cfg.Connection.Database = v
	}
	if v := os.Getenv("CP_USER"); v != "" {
		cfg.Connection.User = v
	}
	if v := os.Getenv("CP_PROJECT"); v != "" {
		cfg.Project = v
	}
	if v := os.Getenv("CP_AGENT"); v != "" {
		cfg.Agent = v
	}

	// Validate
	if cfg.Connection.User == "" {
		return nil, fmt.Errorf("database user is required (set via CP_USER, .cp.yaml, or ~/.cp/config.yaml)")
	}
	if cfg.Agent == "" {
		return nil, fmt.Errorf("agent identity is required (set via CP_AGENT, .cp.yaml, or ~/.cp/config.yaml)")
	}

	return cfg, nil
}

// projectConfig is the structure for .cp.yaml
type projectConfig struct {
	Project string `yaml:"project"`
	Agent   string `yaml:"agent"`
}

// findProjectConfig walks up directories to find .cp.yaml
func findProjectConfig() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, ".cp.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// loadYAML loads a YAML file into the config
func loadYAML(path string, cfg *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	yaml.Unmarshal(data, cfg)
}

// loadProjectConfig loads .cp.yaml and applies to config
func loadProjectConfig(path string, cfg *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var pc projectConfig
	if err := yaml.Unmarshal(data, &pc); err != nil {
		return
	}
	if pc.Project != "" {
		cfg.Project = pc.Project
	}
	if pc.Agent != "" {
		cfg.Agent = pc.Agent
	}
}
