package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AssembleContext builds the content for a CLAUDE.md from the configured context layers.
// mode is "interactive" or "dispatch" (or a gate name like "gate:readiness-review").
// repoRoot is the repo directory.
// extras are additional key-value pairs to inject (e.g. "dispatch-prompt" → prompt text).
func AssembleContext(ctx context.Context, cfg *PipelineConfig, repoRoot, mode string, extras map[string]string, c *Client) (string, error) {
	if cfg == nil || len(cfg.Context.Layers) == 0 {
		return assembleDefaultContext(cfg, repoRoot, mode, extras)
	}

	var sections []string

	for _, layer := range cfg.Context.Layers {
		if !layerActive(layer.When, mode) {
			continue
		}

		content, err := resolveLayer(ctx, layer, cfg, repoRoot, extras, c)
		if err != nil {
			// Non-fatal — skip this layer
			sections = append(sections, fmt.Sprintf("<!-- context layer %q failed: %v -->", layer.Name, err))
			continue
		}
		if content == "" {
			continue
		}

		sections = append(sections, fmt.Sprintf("<!-- context: %s -->\n%s", layer.Name, content))
	}

	return strings.Join(sections, "\n\n---\n\n"), nil
}

// layerActive checks if a layer should be included for the given mode.
func layerActive(when, mode string) bool {
	switch when {
	case "always", "":
		return true
	case "interactive":
		return mode == "interactive"
	case "dispatch":
		return mode == "dispatch"
	default:
		// "gate:readiness-review" matches mode "gate:readiness-review"
		return when == mode
	}
}

// resolveLayer fetches the content for a single context layer.
func resolveLayer(ctx context.Context, layer ContextLayer, cfg *PipelineConfig, repoRoot string, extras map[string]string, c *Client) (string, error) {
	source := layer.Source

	switch {
	case strings.HasPrefix(source, "file:"):
		path := strings.TrimPrefix(source, "file:")
		if !filepath.IsAbs(path) {
			path = filepath.Join(repoRoot, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil

	case strings.HasPrefix(source, "shard:"):
		shardID := strings.TrimPrefix(source, "shard:")
		if c == nil {
			return "", fmt.Errorf("no client for shard lookup")
		}
		shard, err := c.GetShard(ctx, shardID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("# %s\n\n%s", shard.Title, shard.Content), nil

	case strings.HasPrefix(source, "skills:"):
		skillName := strings.TrimPrefix(source, "skills:")
		skillPath, err := ResolveSkill(repoRoot, skillName)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(skillPath)
		if err != nil {
			return "", err
		}
		return string(data), nil

	case source == "skills-dir":
		return resolveSkillsDir(cfg, repoRoot, layer.Filter)

	case source == "claude-md":
		path := filepath.Join(repoRoot, "CLAUDE.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return "", nil // CLAUDE.md is optional
		}
		return string(data), nil

	case source == "dispatch-prompt":
		return extras["dispatch-prompt"], nil

	case source == "parent-design":
		return extras["parent-design"], nil

	case strings.HasPrefix(source, "hook:"):
		// Hooks are handled by Claude Code itself, not by us
		return "", nil

	default:
		// Check extras
		if val, ok := extras[source]; ok {
			return val, nil
		}
		return "", fmt.Errorf("unknown context source: %s", source)
	}
}

// resolveSkillsDir loads all skills from the skills directory, optionally filtered.
func resolveSkillsDir(cfg *PipelineConfig, repoRoot string, filter []string) (string, error) {
	skillsDir := "skills"
	if cfg != nil && cfg.SkillsDir != "" {
		skillsDir = cfg.SkillsDir
	}
	dir := filepath.Join(repoRoot, skillsDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil // skills dir is optional
	}

	filterSet := make(map[string]bool)
	for _, f := range filter {
		filterSet[f] = true
	}

	var parts []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if len(filterSet) > 0 && !filterSet[e.Name()] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		parts = append(parts, string(data))
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// assembleDefaultContext provides sensible defaults when no context layers are configured.
func assembleDefaultContext(cfg *PipelineConfig, repoRoot, mode string, extras map[string]string) (string, error) {
	var sections []string

	switch mode {
	case "dispatch":
		// For dispatch: task prompt + completion instructions only
		if prompt, ok := extras["dispatch-prompt"]; ok {
			sections = append(sections, prompt)
		}
		if design, ok := extras["parent-design"]; ok && design != "" {
			sections = append(sections, design)
		}

	case "interactive":
		// For interactive: load repo CLAUDE.md
		claudeMD := filepath.Join(repoRoot, "CLAUDE.md")
		if data, err := os.ReadFile(claudeMD); err == nil {
			sections = append(sections, string(data))
		}
	}

	return strings.Join(sections, "\n\n---\n\n"), nil
}

// WriteWorktreeCLAUDEMD generates a CLAUDE.md for a worktree based on context config.
func WriteWorktreeCLAUDEMD(ctx context.Context, cfg *PipelineConfig, repoRoot, worktreePath, mode string, extras map[string]string, c *Client) error {
	content, err := AssembleContext(ctx, cfg, repoRoot, mode, extras, c)
	if err != nil {
		return fmt.Errorf("assembling context: %w", err)
	}

	if content == "" {
		return nil // nothing to write
	}

	path := filepath.Join(worktreePath, "CLAUDE.md")
	return os.WriteFile(path, []byte(content), 0644)
}
