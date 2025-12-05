package cxpdir

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/otherjamesbrown/context-palace/internal/config"
	"github.com/otherjamesbrown/context-palace/internal/memo"
	"gopkg.in/yaml.v3"
)

const (
	DirName    = ".cxp"
	ConfigFile = "config.yaml"
	MemosDir   = "memos"
	LogsDir    = "logs"
)

var ErrNotInitialized = errors.New(".cxp not initialized. Run 'cxp init' first")

// FindRoot walks up from cwd to find .cxp directory.
// Returns path to directory containing .cxp, or error if not found.
func FindRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		if Exists(dir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", ErrNotInitialized
		}
		dir = parent
	}
}

// Exists checks if .cxp directory exists in given path.
func Exists(path string) bool {
	info, err := os.Stat(filepath.Join(path, DirName))
	return err == nil && info.IsDir()
}

// Initialize creates .cxp directory structure with default config.
// Idempotent - safe to call multiple times.
func Initialize(path string) error {
	cxpPath := filepath.Join(path, DirName)

	// Create directories
	dirs := []string{
		cxpPath,
		filepath.Join(cxpPath, MemosDir),
		filepath.Join(cxpPath, LogsDir),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Write default config if it doesn't exist
	configPath := filepath.Join(cxpPath, ConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

// GetMemoPath returns the file path for a memo by category.
// Handles dot notation: "ci-cd.docker" -> ".cxp/memos/ci-cd/docker.yaml"
func GetMemoPath(root, category string) string {
	parts := strings.Split(category, ".")
	if len(parts) == 1 {
		return filepath.Join(root, DirName, MemosDir, category+".yaml")
	}
	// Child memo: parent directory + child file
	parentPath := strings.Join(parts[:len(parts)-1], string(filepath.Separator))
	child := parts[len(parts)-1]
	return filepath.Join(root, DirName, MemosDir, parentPath, child+".yaml")
}

// GetParentCategory returns the parent category for a child memo.
// "ci-cd.docker" -> "ci-cd", "build" -> ""
func GetParentCategory(category string) string {
	parts := strings.Split(category, ".")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}

// IsChildCategory returns true if category has a parent.
func IsChildCategory(category string) bool {
	return strings.Contains(category, ".")
}

// LoadMemo reads and parses a memo file.
func LoadMemo(path string) (*memo.Memo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	m := &memo.Memo{
		Content: make(map[string]interface{}),
	}

	if err := yaml.Unmarshal(data, m); err != nil {
		return nil, err
	}

	// Also unmarshal into Content for inline fields
	if err := yaml.Unmarshal(data, &m.Content); err != nil {
		return nil, err
	}

	// Remove known fields from Content (they're in struct fields)
	delete(m.Content, "parent")
	delete(m.Content, "source_doc")

	return m, nil
}

// SaveMemo writes a memo to disk.
func SaveMemo(path string, m *memo.Memo) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Build output map with struct fields first, then content
	output := make(map[string]interface{})

	if m.Parent != "" {
		output["parent"] = m.Parent
	}
	if m.SourceDoc != "" {
		output["source_doc"] = m.SourceDoc
	}

	// Add content fields
	for k, v := range m.Content {
		output[k] = v
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// MemoExists checks if a memo exists for given category.
func MemoExists(root, category string) bool {
	path := GetMemoPath(root, category)
	_, err := os.Stat(path)
	return err == nil
}

// ParentExists checks if parent memo exists for child category.
func ParentExists(root, category string) bool {
	parent := GetParentCategory(category)
	if parent == "" {
		return true // No parent needed
	}
	return MemoExists(root, parent)
}

// LoadConfig reads .cxp/config.yaml.
func LoadConfig(root string) (*config.Config, error) {
	path := filepath.Join(root, DirName, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// AppendLog appends a JSON line to a log file.
func AppendLog(root, logFile string, entry interface{}) error {
	path := filepath.Join(root, DirName, logFile)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(string(data) + "\n")
	return err
}
