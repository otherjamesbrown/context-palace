package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	initProject string
	initAgent   string
	initForce   bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .cp.yaml in current directory",
	Long: `Initialize a Context Palace project config in the current directory.

If --project is not specified, attempts to detect from git remote or directory name.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ".cp.yaml"

		// Check if already exists
		if _, err := os.Stat(configPath); err == nil && !initForce {
			return fmt.Errorf("config already exists. Use --force to overwrite")
		}

		// Detect project name if not provided
		project := initProject
		if project == "" {
			project = detectProjectName()
		}
		if project == "" {
			return fmt.Errorf("could not detect project name. Use --project <name>")
		}

		// Build config
		type initConfig struct {
			Project string `yaml:"project"`
			Agent   string `yaml:"agent,omitempty"`
		}
		cfg := initConfig{
			Project: project,
			Agent:   initAgent,
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %v", err)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %v", configPath, err)
		}

		fmt.Printf("Created %s\n", configPath)
		fmt.Printf("  project: %s\n", project)
		if initAgent != "" {
			fmt.Printf("  agent:   %s\n", initAgent)
		}
		return nil
	},
}

// detectProjectName tries to detect the project name from git or directory
func detectProjectName() string {
	// Try git remote
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err == nil {
		remote := strings.TrimSpace(string(out))
		// Extract repo name from URL
		// git@github.com:org/repo.git → repo
		// https://github.com/org/repo.git → repo
		parts := strings.Split(remote, "/")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			name = strings.TrimSuffix(name, ".git")
			if name != "" {
				return name
			}
		}
	}

	// Fall back to directory name
	dir, err := os.Getwd()
	if err == nil {
		return filepath.Base(dir)
	}

	return ""
}

func init() {
	initCmd.Flags().StringVar(&initProject, "project", "", "Project name")
	initCmd.Flags().StringVar(&initAgent, "agent", "", "Agent identity")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing config")

	rootCmd.AddCommand(initCmd)
}
