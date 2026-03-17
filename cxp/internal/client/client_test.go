package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionString_Defaults(t *testing.T) {
	c := NewClient(&Config{
		Connection: ConnectionConfig{
			Host:     "myhost",
			Database: "mydb",
			User:     "myuser",
		},
	})
	cs := c.ConnectionString()
	assert.Contains(t, cs, "host=myhost")
	assert.Contains(t, cs, "dbname=mydb")
	assert.Contains(t, cs, "user=myuser")
	assert.Contains(t, cs, "sslmode=verify-full")
}

func TestConnectionString_CustomSSL(t *testing.T) {
	c := NewClient(&Config{
		Connection: ConnectionConfig{
			Host:     "h",
			Database: "d",
			User:     "u",
			SSLMode:  "disable",
		},
	})
	cs := c.ConnectionString()
	assert.Contains(t, cs, "sslmode=disable")
	assert.NotContains(t, cs, "verify-full")
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	// Create a config file with known values
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yaml := `connection:
  host: filehost
  database: filedb
  user: fileuser
agent: fileagent
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yaml), 0644))

	t.Setenv("CXP_HOST", "envhost")
	t.Setenv("CXP_DATABASE", "")
	t.Setenv("CXP_USER", "")
	t.Setenv("CXP_PROJECT", "")
	t.Setenv("CXP_AGENT", "")

	cfg, err := LoadConfig(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "envhost", cfg.Connection.Host)
	assert.Equal(t, "filedb", cfg.Connection.Database)
}

func TestLoadConfig_MissingUser(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yaml := `connection:
  host: h
  database: d
agent: a
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yaml), 0644))

	// Clear env vars that could provide user
	t.Setenv("CXP_USER", "")
	t.Setenv("CXP_HOST", "")
	t.Setenv("CXP_DATABASE", "")
	t.Setenv("CXP_PROJECT", "")
	t.Setenv("CXP_AGENT", "")

	_, err := LoadConfig(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database user is required")
}

func TestLoadConfig_MissingAgent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yaml := `connection:
  host: h
  database: d
  user: u
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yaml), 0644))

	t.Setenv("CXP_USER", "")
	t.Setenv("CXP_HOST", "")
	t.Setenv("CXP_DATABASE", "")
	t.Setenv("CXP_PROJECT", "")
	t.Setenv("CXP_AGENT", "")

	_, err := LoadConfig(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent identity is required")
}

func TestLoadConfig_AllEnvVars(t *testing.T) {
	// Create a minimal config file (just so LoadConfig doesn't fail on file read)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("{}"), 0644))

	t.Setenv("CXP_HOST", "envhost")
	t.Setenv("CXP_DATABASE", "envdb")
	t.Setenv("CXP_USER", "envuser")
	t.Setenv("CXP_PROJECT", "envproject")
	t.Setenv("CXP_AGENT", "envagent")

	cfg, err := LoadConfig(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "envhost", cfg.Connection.Host)
	assert.Equal(t, "envdb", cfg.Connection.Database)
	assert.Equal(t, "envuser", cfg.Connection.User)
	assert.Equal(t, "envproject", cfg.Project)
	assert.Equal(t, "envagent", cfg.Agent)
}

func TestLoadConfig_InvalidGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("connection: ["), 0644))

	t.Setenv("CXP_USER", "")
	t.Setenv("CXP_HOST", "")
	t.Setenv("CXP_DATABASE", "")
	t.Setenv("CXP_PROJECT", "")
	t.Setenv("CXP_AGENT", "")

	_, err := LoadConfig(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing")
	assert.Contains(t, err.Error(), cfgPath)
}
