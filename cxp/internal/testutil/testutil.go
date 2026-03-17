package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TempConfigDir creates a temp directory with a config file for testing LoadConfig.
func TempConfigDir(t *testing.T, configYAML string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TempProjectDir creates a temp directory with .cxp.yaml for testing findProjectConfig.
func TempProjectDir(t *testing.T, projectYAML string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".cxp.yaml")
	if err := os.WriteFile(path, []byte(projectYAML), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}
