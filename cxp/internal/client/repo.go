package client

import (
	"fmt"
	"os/exec"
	"strings"
)

// DetectOwnerRepo returns the "owner/repo" identifier by reading the
// origin remote of the git repository at dir. Supports both SSH and
// HTTPS GitHub remotes. Returns an error if no origin is configured
// or the URL is not a github.com remote.
func DetectOwnerRepo(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("reading git origin in %s: %w", dir, err)
	}
	return parseGitHubOwnerRepo(strings.TrimSpace(string(out)))
}

// parseGitHubOwnerRepo extracts "owner/repo" from a GitHub remote URL.
// Supported forms:
//
//	git@github.com:owner/repo.git
//	git@github.com:owner/repo
//	https://github.com/owner/repo.git
//	https://github.com/owner/repo
//	ssh://git@github.com/owner/repo.git
func parseGitHubOwnerRepo(url string) (string, error) {
	idx := strings.Index(url, "github.com")
	if idx < 0 {
		return "", fmt.Errorf("not a github.com remote: %q", url)
	}
	rest := url[idx+len("github.com"):]
	if len(rest) == 0 || (rest[0] != ':' && rest[0] != '/') {
		return "", fmt.Errorf("not a github.com remote: %q", url)
	}
	rest = rest[1:]
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 {
		return "", fmt.Errorf("remote missing owner/repo: %q", url)
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSuffix(parts[1], ".git")
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return "", fmt.Errorf("remote missing owner/repo: %q", url)
	}
	return owner + "/" + repo, nil
}
