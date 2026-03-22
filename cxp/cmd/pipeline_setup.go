package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var pipelineSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Register current repo for pipeline automation",
	Long: `Run in a git repo to register it for pipeline automation.

Auto-detects language, build commands, GitHub remote, and project name.
Creates local .cxp/pipeline.yaml and updates ~/.cxp/repos.yaml registry.`,
	RunE: runPipelineSetup,
}

func init() {
	pipelineSetupCmd.Flags().String("project", "", "Override project name detection")
	pipelineSetupCmd.Flags().Bool("force", false, "Overwrite existing .cxp/pipeline.yaml")
	pipelineSetupCmd.Flags().Bool("dry-run", false, "Show what would be written without writing files")

	pipelineCmd.AddCommand(pipelineSetupCmd)
}

func runPipelineSetup(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	projectOverride, _ := cmd.Flags().GetString("project")

	// 1. Find git root
	gitRootOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("not a git repository (or git is not installed)")
	}
	repoRoot := strings.TrimSpace(string(gitRootOut))

	// 2. Detect project name
	project := detectSetupProject(projectOverride, repoRoot)
	if project == "" {
		return fmt.Errorf("could not detect project name. Use --project <name>")
	}

	// 3. Detect GitHub owner/repo
	ownerRepo := detectOwnerRepo()

	// 4. Detect language/build system
	buildCmds, testCmds := detectBuildSystem(repoRoot)

	// 5. Detect default branch
	defaultBranch := detectDefaultBranch()

	// 6. Check for existing config
	pipelineDir := filepath.Join(repoRoot, ".cxp")
	pipelinePath := filepath.Join(pipelineDir, "pipeline.yaml")
	if _, err := os.Stat(pipelinePath); err == nil && !force {
		return fmt.Errorf("already configured. Use --force to overwrite")
	}

	// 7. Build pipeline config YAML
	pipelineYAML := buildPipelineYAML(project, ownerRepo, buildCmds, testCmds)

	// 8. Prepare repos registry update
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %v", err)
	}
	reposPath := filepath.Join(homeDir, ".cxp", "repos.yaml")

	reg, err := client.LoadRepoRegistry()
	if err != nil {
		// Start fresh if we can't load
		reg = &client.RepoRegistry{Repos: make(map[string]client.RepoEntry)}
	}
	reg.Repos[project] = client.RepoEntry{
		Path:          repoRoot,
		DefaultBranch: defaultBranch,
	}

	reposData, err := yaml.Marshal(reg)
	if err != nil {
		return fmt.Errorf("failed to marshal repos registry: %v", err)
	}

	// Dry-run: print and exit
	if dryRun {
		fmt.Printf("Dry run — no files written.\n\n")
		fmt.Printf("Would write %s:\n", pipelinePath)
		fmt.Printf("---\n%s\n", pipelineYAML)
		fmt.Printf("Would write %s:\n", reposPath)
		fmt.Printf("---\n%s\n", string(reposData))
		return nil
	}

	// Write .cxp/pipeline.yaml
	if err := os.MkdirAll(pipelineDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %v", pipelineDir, err)
	}
	if err := os.WriteFile(pipelinePath, []byte(pipelineYAML), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %v", pipelinePath, err)
	}

	// Write ~/.cxp/repos.yaml via the client helper
	if err := client.SaveRepoRegistry(reg); err != nil {
		return fmt.Errorf("failed to save repo registry: %v", err)
	}

	// 9. Print summary
	fmt.Printf("Pipeline configured for %s\n", project)
	fmt.Printf("  Config:   %s\n", pipelinePath)
	fmt.Printf("  Registry: %s\n", reposPath)
	fmt.Printf("  Build:    %s\n", formatCmdList(buildCmds))
	fmt.Printf("  Test:     %s\n", formatCmdList(testCmds))
	fmt.Printf("  GitHub:   %s\n", ownerRepo)
	fmt.Printf("  Branch:   %s\n", defaultBranch)

	return nil
}

// detectSetupProject resolves project name by precedence.
func detectSetupProject(flagValue, repoRoot string) string {
	// 1. --project flag
	if flagValue != "" {
		return flagValue
	}

	// 2. .cxp.yaml or .cp.yaml in repo root
	for _, name := range []string{".cxp.yaml", ".cp.yaml"} {
		data, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err != nil {
			continue
		}
		var parsed struct {
			Project string `yaml:"project"`
		}
		if err := yaml.Unmarshal(data, &parsed); err == nil && parsed.Project != "" {
			return parsed.Project
		}
	}

	// 3. Global cxp config project
	if cpClient != nil && cpClient.Config.Project != "" {
		return cpClient.Config.Project
	}

	// 4. Directory name
	return filepath.Base(repoRoot)
}

// detectOwnerRepo parses GitHub owner/repo from git remote origin.
func detectOwnerRepo() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	remote := strings.TrimSpace(string(out))

	// SSH: git@github.com:owner/repo.git
	sshRe := regexp.MustCompile(`git@github\.com:([^/]+/[^/]+?)(?:\.git)?$`)
	if m := sshRe.FindStringSubmatch(remote); len(m) > 1 {
		return m[1]
	}

	// HTTPS: https://github.com/owner/repo.git
	httpsRe := regexp.MustCompile(`https://github\.com/([^/]+/[^/]+?)(?:\.git)?$`)
	if m := httpsRe.FindStringSubmatch(remote); len(m) > 1 {
		return m[1]
	}

	return ""
}

// detectBuildSystem checks for language markers in the repo root and one level deep.
func detectBuildSystem(repoRoot string) (build []string, test []string) {
	// Check root and immediate subdirs for build markers
	findMarker := func(name string) string {
		// Check root first
		if _, err := os.Stat(filepath.Join(repoRoot, name)); err == nil {
			return ""
		}
		// Check one level deep
		entries, err := os.ReadDir(repoRoot)
		if err != nil {
			return ""
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				if _, err := os.Stat(filepath.Join(repoRoot, e.Name(), name)); err == nil {
					return e.Name()
				}
			}
		}
		return ""
	}

	if subdir := findMarker("go.mod"); subdir != "" {
		prefix := "cd " + subdir + " && "
		return []string{prefix + "go build ./..."}, []string{prefix + "go test ./...", prefix + "go vet ./..."}
	} else if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
		return []string{"go build ./..."}, []string{"go test ./...", "go vet ./..."}
	}

	if _, err := os.Stat(filepath.Join(repoRoot, "package.json")); err == nil {
		return []string{"npm run build"}, []string{"npm test"}
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "Cargo.toml")); err == nil {
		return []string{"cargo build"}, []string{"cargo test"}
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "pyproject.toml")); err == nil {
		return []string{}, []string{"pytest"}
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "setup.py")); err == nil {
		return []string{}, []string{"pytest"}
	}
	return []string{}, []string{}
}

// detectDefaultBranch returns the default branch name from the remote.
func detectDefaultBranch() string {
	out, err := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		// refs/remotes/origin/main → main
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return "main"
}

// buildPipelineYAML generates the pipeline.yaml content as a string.
func buildPipelineYAML(project, ownerRepo string, buildCmds, testCmds []string) string {
	cfg := client.PipelineConfig{
		Build: buildCmds,
		Test:  testCmds,
		Agents: map[string]client.AgentCfg{
			"agent-steve":   {Domains: []string{"cli", "migrations"}},
			"agent-mycroft": {Domains: []string{"backend", "services"}},
		},
		Dispatch: client.DispatchCfg{
			MaxConcurrent: 3,
			TmuxSession:   "main",
		},
		GitHub: client.GitHubCfg{
			OwnerRepo: ownerRepo,
		},
		SkillsDir: "skills",
	}

	data, _ := yaml.Marshal(cfg)
	header := fmt.Sprintf("# Pipeline configuration for %s\n# See: cxp pipeline setup --help\n\n", project)
	return header + string(data)
}

// formatCmdList formats a command list for display.
func formatCmdList(cmds []string) string {
	if len(cmds) == 0 {
		return "(none)"
	}
	return strings.Join(cmds, ", ")
}
