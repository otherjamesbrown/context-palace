package client

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GateFieldConfig describes a single field within a gate.
type GateFieldConfig struct {
	Type     string `yaml:"type"`
	Min      *int   `yaml:"min,omitempty"`
	Max      *int   `yaml:"max,omitempty"`
	Required bool   `yaml:"required,omitempty"`
}

// GateConfig describes a quality gate within a pipeline phase.
type GateConfig struct {
	Name          string                     `yaml:"name"`
	Skill         string                     `yaml:"skill,omitempty"`
	Model         string                     `yaml:"model,omitempty"`
	Fields        map[string]GateFieldConfig `yaml:"fields,omitempty"`
	RequiresLabel string                     `yaml:"requires_label,omitempty"`
}

// PhaseConfig describes a pipeline phase and its gates.
type PhaseConfig struct {
	Name  string       `yaml:"name"`
	Model string       `yaml:"model,omitempty"`
	Gates []GateConfig `yaml:"gates,omitempty"`
}

// WorkflowConfig defines a pipeline path for a specific shard type.
type WorkflowConfig struct {
	Phases        []string       `yaml:"phases"`
	ContextLayers []ContextLayer `yaml:"context_layers,omitempty"`
}

// ContextLayer defines a piece of context that can be injected into agent sessions.
type ContextLayer struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"` // "file:<path>", "shard:<id>", "skills:<name>", "dispatch-prompt", "parent-design"
	When   string `yaml:"when"`   // "interactive", "dispatch", "always", "gate:<name>"
	Filter []string `yaml:"filter,omitempty"` // for skills source: only load these files
}

// ContextConfig controls what context is injected into agent sessions.
type ContextConfig struct {
	Layers []ContextLayer `yaml:"layers,omitempty"`
}

// PipelineConfig holds pipeline configuration loaded from YAML files.
type PipelineConfig struct {
	Build           []string            `yaml:"build,omitempty"`
	Test            []string            `yaml:"test,omitempty"`
	CompletionSteps []string            `yaml:"completion_steps,omitempty"`
	Agents          map[string]AgentCfg `yaml:"agents,omitempty"`
	Dispatch        DispatchCfg         `yaml:"dispatch,omitempty"`
	Monitoring      MonitoringCfg       `yaml:"monitoring,omitempty"`
	Review          ReviewCfg           `yaml:"review,omitempty"`
	Context         ContextConfig                `yaml:"context,omitempty"`
	Workflows       map[string]WorkflowConfig   `yaml:"workflows,omitempty"`
	GitHub          GitHubCfg                    `yaml:"github,omitempty"`
	SkillsDir       string              `yaml:"skills_dir,omitempty"`
	Phases          []PhaseConfig       `yaml:"phases,omitempty"`
}

// AgentCfg defines an agent's domain capabilities.
type AgentCfg struct {
	Domains []string `yaml:"domains"`
}

// DispatchCfg controls how work is dispatched to agents.
type DispatchCfg struct {
	MaxConcurrent int    `yaml:"max_concurrent,omitempty"`
	TmuxSession   string `yaml:"tmux_session,omitempty"`
	ClaudeFlags   string `yaml:"claude_flags,omitempty"`
	DefaultModel  string `yaml:"default_model,omitempty"` // fallback model
}

// ModelForPhase resolves the model to use for a given phase.
// Priority: gate model → phase model → dispatch default_model → ""
func (c *PipelineConfig) ModelForPhase(phaseName, gateName string) string {
	if c == nil {
		return ""
	}
	// Check gate-level model
	if gateName != "" {
		if gate := c.FindGate(phaseName, gateName); gate != nil && gate.Model != "" {
			return gate.Model
		}
	}
	// Check phase-level model
	if phase := c.FindPhase(phaseName); phase != nil && phase.Model != "" {
		return phase.Model
	}
	// Check review-level model
	if c.Review.Model != "" && (phaseName == "review") {
		return c.Review.Model
	}
	// Check monitoring-level model
	if c.Monitoring.Model != "" && (phaseName == "monitoring" || gateName == "stall-check") {
		return c.Monitoring.Model
	}
	// Fallback to dispatch default
	return c.Dispatch.DefaultModel
}

// GitHubCfg holds GitHub repository information.
type GitHubCfg struct {
	OwnerRepo string `yaml:"owner_repo"`
}

// CICfg controls how CI results are evaluated.
type CICfg struct {
	Mode string `yaml:"mode,omitempty"` // "pr-only", "all-pass", or "ignore"
	Wait bool   `yaml:"wait,omitempty"` // wait for CI to complete before reviewing
}

// ReviewCfg controls how PRs are reviewed.
type ReviewCfg struct {
	CI                CICfg    `yaml:"ci,omitempty"`
	Strategy          string   `yaml:"strategy,omitempty"`           // "external" or "agent"
	ExternalReviewers []string `yaml:"external_reviewers,omitempty"` // GitHub bot usernames
	ProcessSkill      string   `yaml:"process_skill,omitempty"`      // skill for processing external reviews
	ReviewSkill       string   `yaml:"review_skill,omitempty"`       // skill for agent-based reviews
	ReviewAgent       string   `yaml:"review_agent,omitempty"`       // who does agent reviews
	Model             string   `yaml:"model,omitempty"`              // model for review tasks
}

// MonitoringCfg controls health monitoring for dispatched agents.
type MonitoringCfg struct {
	StallTimeout string            `yaml:"stall_timeout,omitempty"` // e.g. "30m"
	CrashCheck   bool              `yaml:"crash_check,omitempty"`
	MaxRetries   int               `yaml:"max_retries,omitempty"`
	Cooldown     string            `yaml:"cooldown,omitempty"` // e.g. "5m"
	Model        string            `yaml:"model,omitempty"`    // model for health checks
	Actions      MonitoringActions `yaml:"actions,omitempty"`
}

// MonitoringActions defines what to do for each health event.
type MonitoringActions struct {
	OnStall      string `yaml:"on_stall,omitempty"`       // e.g. "skill:m-stall-check"
	OnCrash      string `yaml:"on_crash,omitempty"`       // e.g. "redispatch"
	OnMaxRetries string `yaml:"on_max_retries,omitempty"` // e.g. "escalate"
}

// RepoRegistry maps project names to local repository paths.
type RepoRegistry struct {
	Repos map[string]RepoEntry `yaml:"repos"`
}

// RepoEntry describes a registered repository.
type RepoEntry struct {
	Path          string `yaml:"path"`
	DefaultBranch string `yaml:"default_branch,omitempty"`
}

// DefaultPipelineConfig returns a PipelineConfig with sensible defaults.
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		Dispatch: DispatchCfg{
			MaxConcurrent: 3,
			TmuxSession:   "main",
			ClaudeFlags:   "--print",
			DefaultModel:  "sonnet",
		},
		Monitoring: MonitoringCfg{
			StallTimeout: "30m",
			CrashCheck:   true,
			MaxRetries:   3,
			Cooldown:     "5m",
			Model:        "haiku",
			Actions: MonitoringActions{
				OnStall:      "skill:m-stall-check",
				OnCrash:      "redispatch",
				OnMaxRetries: "escalate",
			},
		},
		Review: ReviewCfg{
			Model: "haiku",
		},
		SkillsDir: "skills",
	}
}

// LoadPipelineConfig loads pipeline configuration by merging the global config
// (~/.cxp/pipeline.yaml) with a repo-level override (<repoRoot>/.cxp/pipeline.yaml).
// Both files are optional; missing files are silently ignored.
func LoadPipelineConfig(repoRoot string) (*PipelineConfig, error) {
	base := DefaultPipelineConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	globalPath := filepath.Join(home, ".cxp", "pipeline.yaml")
	globalCfg, err := loadPipelineFile(globalPath)
	if err != nil {
		return nil, fmt.Errorf("loading global pipeline config: %w", err)
	}
	if globalCfg != nil {
		base = MergePipelineConfig(base, globalCfg)
	}

	if repoRoot != "" {
		repoPath := filepath.Join(repoRoot, ".cxp", "pipeline.yaml")
		repoCfg, err := loadPipelineFile(repoPath)
		if err != nil {
			return nil, fmt.Errorf("loading repo pipeline config: %w", err)
		}
		if repoCfg != nil {
			base = MergePipelineConfig(base, repoCfg)
		}
	}

	return base, nil
}

// loadPipelineFile reads and parses a single pipeline YAML file.
// Returns (nil, nil) if the file does not exist.
func loadPipelineFile(path string) (*PipelineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg PipelineConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// MergePipelineConfig merges an override config into a base config, returning
// a new PipelineConfig without mutating either input.
//
// Merge rules:
//   - Slice fields: override replaces entirely if non-nil
//   - Map fields (Agents): override keys replace, base keys kept if not in override
//   - Scalar fields: override replaces if non-zero
func MergePipelineConfig(base, override *PipelineConfig) *PipelineConfig {
	out := &PipelineConfig{}

	// Copy base
	out.Build = copyStrings(base.Build)
	out.Test = copyStrings(base.Test)
	out.CompletionSteps = copyStrings(base.CompletionSteps)
	out.Agents = copyAgents(base.Agents)
	out.Dispatch = base.Dispatch
	out.Monitoring = base.Monitoring
	out.Review = base.Review
	out.GitHub = base.GitHub
	out.SkillsDir = base.SkillsDir
	out.Workflows = base.Workflows
	out.Phases = copyPhases(base.Phases)

	// Apply overrides — slices
	if override.Build != nil {
		out.Build = copyStrings(override.Build)
	}
	if override.Test != nil {
		out.Test = copyStrings(override.Test)
	}
	if override.CompletionSteps != nil {
		out.CompletionSteps = copyStrings(override.CompletionSteps)
	}
	if override.Phases != nil {
		out.Phases = copyPhases(override.Phases)
	}
	if override.Workflows != nil {
		out.Workflows = override.Workflows
	}

	// Agents: merge maps
	if override.Agents != nil {
		if out.Agents == nil {
			out.Agents = make(map[string]AgentCfg)
		}
		for k, v := range override.Agents {
			out.Agents[k] = v
		}
	}

	// Scalars
	if override.Dispatch.MaxConcurrent != 0 {
		out.Dispatch.MaxConcurrent = override.Dispatch.MaxConcurrent
	}
	if override.Dispatch.TmuxSession != "" {
		out.Dispatch.TmuxSession = override.Dispatch.TmuxSession
	}
	if override.Dispatch.ClaudeFlags != "" {
		out.Dispatch.ClaudeFlags = override.Dispatch.ClaudeFlags
	}
	if override.Dispatch.DefaultModel != "" {
		out.Dispatch.DefaultModel = override.Dispatch.DefaultModel
	}
	if override.Review.Model != "" {
		out.Review.Model = override.Review.Model
	}
	if override.Review.Strategy != "" {
		out.Review.Strategy = override.Review.Strategy
	}
	if override.Review.ReviewSkill != "" {
		out.Review.ReviewSkill = override.Review.ReviewSkill
	}
	if override.Review.ReviewAgent != "" {
		out.Review.ReviewAgent = override.Review.ReviewAgent
	}
	if override.Review.ProcessSkill != "" {
		out.Review.ProcessSkill = override.Review.ProcessSkill
	}
	if override.Review.ExternalReviewers != nil {
		out.Review.ExternalReviewers = copyStrings(override.Review.ExternalReviewers)
	}
	if override.GitHub.OwnerRepo != "" {
		out.GitHub.OwnerRepo = override.GitHub.OwnerRepo
	}
	if override.SkillsDir != "" {
		out.SkillsDir = override.SkillsDir
	}

	// Monitoring scalars
	if override.Monitoring.StallTimeout != "" {
		out.Monitoring.StallTimeout = override.Monitoring.StallTimeout
	}
	if override.Monitoring.CrashCheck {
		out.Monitoring.CrashCheck = true
	}
	if override.Monitoring.MaxRetries != 0 {
		out.Monitoring.MaxRetries = override.Monitoring.MaxRetries
	}
	if override.Monitoring.Cooldown != "" {
		out.Monitoring.Cooldown = override.Monitoring.Cooldown
	}
	if override.Monitoring.Actions.OnStall != "" {
		out.Monitoring.Actions.OnStall = override.Monitoring.Actions.OnStall
	}
	if override.Monitoring.Actions.OnCrash != "" {
		out.Monitoring.Actions.OnCrash = override.Monitoring.Actions.OnCrash
	}
	if override.Monitoring.Actions.OnMaxRetries != "" {
		out.Monitoring.Actions.OnMaxRetries = override.Monitoring.Actions.OnMaxRetries
	}
	if override.Monitoring.Model != "" {
		out.Monitoring.Model = override.Monitoring.Model
	}

	return out
}

// LoadRepoRegistry loads the repo registry from ~/.cxp/repos.yaml.
// Returns an empty registry if the file does not exist.
func LoadRepoRegistry() (*RepoRegistry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	path := filepath.Join(home, ".cxp", "repos.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RepoRegistry{Repos: make(map[string]RepoEntry)}, nil
		}
		return nil, fmt.Errorf("reading repo registry: %w", err)
	}

	var reg RepoRegistry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing repo registry: %w", err)
	}
	if reg.Repos == nil {
		reg.Repos = make(map[string]RepoEntry)
	}
	return &reg, nil
}

// SaveRepoRegistry writes the repo registry to ~/.cxp/repos.yaml,
// creating the directory if needed.
func SaveRepoRegistry(reg *RepoRegistry) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	dir := filepath.Join(home, ".cxp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(reg)
	if err != nil {
		return fmt.Errorf("marshaling repo registry: %w", err)
	}

	path := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing repo registry: %w", err)
	}
	return nil
}

// RepoForProject looks up a project in the repo registry and returns its path.
func RepoForProject(project string) (string, error) {
	reg, err := LoadRepoRegistry()
	if err != nil {
		return "", err
	}

	entry, ok := reg.Repos[project]
	if !ok {
		return "", fmt.Errorf("project %q not found in repo registry", project)
	}
	return entry.Path, nil
}

// ResolveSkill finds a skill by name, checking the repo-level skills directory
// first, then the global ~/.cxp/skills/ directory.
func ResolveSkill(repoRoot, skillName string) (string, error) {
	cfg, err := LoadPipelineConfig(repoRoot)
	if err != nil {
		return "", fmt.Errorf("loading pipeline config for skill resolution: %w", err)
	}

	skillsDir := cfg.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}

	// Check repo-level first
	if repoRoot != "" {
		repoSkill := filepath.Join(repoRoot, skillsDir, skillName)
		if _, err := os.Stat(repoSkill); err == nil {
			return repoSkill, nil
		}
	}

	// Check global
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	globalSkill := filepath.Join(home, ".cxp", "skills", skillName)
	if _, err := os.Stat(globalSkill); err == nil {
		return globalSkill, nil
	}

	return "", fmt.Errorf("skill %q not found in repo (%s) or global (~/.cxp/skills/)", skillName, filepath.Join(repoRoot, skillsDir))
}

func copyStrings(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func copyAgents(m map[string]AgentCfg) map[string]AgentCfg {
	if m == nil {
		return nil
	}
	out := make(map[string]AgentCfg, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyPhases(p []PhaseConfig) []PhaseConfig {
	if p == nil {
		return nil
	}
	out := make([]PhaseConfig, len(p))
	copy(out, p)
	return out
}

// PhaseNames returns the phase names from config, falling back to
// ValidPipelinePhases if Phases is empty.
func (cfg *PipelineConfig) PhaseNames() []string {
	if len(cfg.Phases) == 0 {
		return ValidPipelinePhases
	}
	names := make([]string, len(cfg.Phases))
	for i, p := range cfg.Phases {
		names[i] = p.Name
	}
	return names
}

// FindPhase finds a phase by name. Returns nil if not found.
func (cfg *PipelineConfig) FindPhase(name string) *PhaseConfig {
	for i := range cfg.Phases {
		if cfg.Phases[i].Name == name {
			return &cfg.Phases[i]
		}
	}
	return nil
}

// FindGate finds a gate within a phase. Returns nil if not found.
func (cfg *PipelineConfig) FindGate(phaseName, gateName string) *GateConfig {
	phase := cfg.FindPhase(phaseName)
	if phase == nil {
		return nil
	}
	for i := range phase.Gates {
		if phase.Gates[i].Name == gateName {
			return &phase.Gates[i]
		}
	}
	return nil
}

// NextPhase returns the next phase name after current, or "" if current is
// the last phase or not found.
func (cfg *PipelineConfig) NextPhase(current string) string {
	names := cfg.PhaseNames()
	for i, name := range names {
		if name == current && i+1 < len(names) {
			return names[i+1]
		}
	}
	return ""
}

// WorkflowForType returns the workflow config for a shard type.
// Falls back to "design" workflow, then the full phase list.
func (cfg *PipelineConfig) WorkflowForType(shardType string) *WorkflowConfig {
	if cfg.Workflows != nil {
		if wf, ok := cfg.Workflows[shardType]; ok {
			return &wf
		}
		if wf, ok := cfg.Workflows["design"]; ok {
			return &wf
		}
	}
	// Default: all phases
	names := cfg.PhaseNames()
	return &WorkflowConfig{Phases: names}
}

// StartPhaseForType returns the first phase for a shard type's workflow.
func (cfg *PipelineConfig) StartPhaseForType(shardType string) string {
	wf := cfg.WorkflowForType(shardType)
	if len(wf.Phases) > 0 {
		return wf.Phases[0]
	}
	return "design"
}

// NextPhaseInWorkflow returns the next phase for a specific workflow.
func (cfg *PipelineConfig) NextPhaseInWorkflow(shardType, current string) string {
	wf := cfg.WorkflowForType(shardType)
	for i, name := range wf.Phases {
		if name == current && i+1 < len(wf.Phases) {
			return wf.Phases[i+1]
		}
	}
	return ""
}
