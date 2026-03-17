package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/context-palace/cxp/internal/embedding"
	"github.com/otherjamesbrown/context-palace/cxp/internal/generation"
)

// KnowledgeBaseConfig holds knowledge base search settings
type KnowledgeBaseConfig struct {
	Root          string `yaml:"root"`
	Unsorted      string `yaml:"unsorted"`
	IncludeClosed bool   `yaml:"include_closed"`
	DefaultMode   string `yaml:"default_mode"` // text, semantic, hybrid
}

// DefaultsConfig holds default flag values
type DefaultsConfig struct {
	Output string `yaml:"output"` // default output format (text|json|yaml)
}

// Config holds the cxp CLI configuration
type Config struct {
	Connection    ConnectionConfig             `yaml:"connection"`
	Agent         string                       `yaml:"agent"`
	Project       string                       `yaml:"project"`
	Embedding     *embedding.EmbeddingConfig   `yaml:"embedding,omitempty"`
	Generation    *generation.GenerationConfig `yaml:"generation,omitempty"`
	KnowledgeBase *KnowledgeBaseConfig         `yaml:"knowledge_base,omitempty"`
	Defaults      *DefaultsConfig              `yaml:"defaults,omitempty"`
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

const (
	projectConfigName     = ".cxp.yaml"
	legacyProjectConfig   = ".cp.yaml"
	globalConfigDirName   = ".cxp"
	legacyGlobalConfigDir = ".cp"
)

// LoadConfig loads configuration with precedence:
// env vars > .cxp.yaml (project) > ~/.cxp/config.yaml (global) > defaults
func LoadConfig(configOverride string) (*Config, error) {
	cfg := &Config{
		Connection: ConnectionConfig{
			Host:     "dev02.brown.chat",
			Database: "contextpalace",
			SSLMode:  "verify-full",
		},
	}

	// Load global config: ~/.cxp/config.yaml (fallback: ~/.cp/config.yaml)
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, globalConfigDirName, "config.yaml")
		if configOverride != "" {
			globalPath = configOverride
			if err := loadYAML(globalPath, cfg); err != nil {
				return nil, err
			}
		} else {
			legacyGlobalPath := filepath.Join(home, legacyGlobalConfigDir, "config.yaml")
			if err := loadYAML(legacyGlobalPath, cfg); err != nil {
				return nil, err
			}
			if err := loadYAML(globalPath, cfg); err != nil {
				return nil, err
			}
		}
	}

	// Load project config: walk up to find .cxp.yaml (fallback: .cp.yaml)
	if configOverride == "" {
		if projectPath := findProjectConfig(); projectPath != "" {
			if err := loadProjectConfig(projectPath, cfg); err != nil {
				return nil, err
			}
		}
	}

	// Environment variables override everything. Prefer CXP_* names, but accept CP_* for compatibility.
	if v := firstEnv("CXP_HOST", "CP_HOST"); v != "" {
		cfg.Connection.Host = v
	}
	if v := firstEnv("CXP_DATABASE", "CP_DATABASE"); v != "" {
		cfg.Connection.Database = v
	}
	if v := firstEnv("CXP_USER", "CP_USER"); v != "" {
		cfg.Connection.User = v
	}
	if v := firstEnv("CXP_PROJECT", "CP_PROJECT"); v != "" {
		cfg.Project = v
	}
	if v := firstEnv("CXP_AGENT", "CP_AGENT"); v != "" {
		cfg.Agent = v
	}

	// Validate
	if cfg.Connection.User == "" {
		return nil, fmt.Errorf("database user is required (set via CXP_USER, .cxp.yaml, or ~/.cxp/config.yaml)")
	}
	if cfg.Agent == "" {
		return nil, fmt.Errorf("agent identity is required (set via CXP_AGENT, .cxp.yaml, or ~/.cxp/config.yaml)")
	}

	return cfg, nil
}

// projectConfig is the structure for .cxp.yaml.
type projectConfig struct {
	Project string `yaml:"project"`
	Agent   string `yaml:"agent"`
}

// findProjectConfig walks up directories to find .cxp.yaml, falling back to .cp.yaml.
func findProjectConfig() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, projectConfigName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		legacyPath := filepath.Join(dir, legacyProjectConfig)
		if _, err := os.Stat(legacyPath); err == nil {
			return legacyPath
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// loadYAML loads a YAML file into the config.
func loadYAML(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("error reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("error parsing %s: %w", path, err)
	}
	return nil
}

// loadProjectConfig loads project config and applies it to config.
func loadProjectConfig(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("error reading %s: %w", path, err)
	}
	var pc projectConfig
	if err := yaml.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("error parsing %s: %w", path, err)
	}
	if pc.Project != "" {
		cfg.Project = pc.Project
	}
	if pc.Agent != "" {
		cfg.Agent = pc.Agent
	}
	return nil
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
