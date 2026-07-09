package git

import (
	"fmt"
	"strings"

	"tmux-control-panel/run"
)

type Client struct {
	runner run.Runner
}

// NewClient creates a new Git client with the given runner.
func NewClient(r run.Runner) *Client {
	return &Client{runner: r}
}

// InsideWorkTree checks if the path is inside a Git working tree.
// It returns true if the path is part of a working tree (including subdirectories),
// and false if it is not a Git repository or if it is inside the repository's
// internal administrative directory (e.g., the .git folder).
func (c *Client) InsideWorkTree(path string) (bool, error) {
	res, err := c.runner.Run("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		// If git command fails or is not a git repo, returns false
		return false, nil
	}
	return strings.TrimSpace(res.Stdout) == "true", nil
}

// IsGitRepository checks if the path belongs to a Git repository (bare or standard).
func (c *Client) IsGitRepository(path string) (bool, error) {
	_, err := c.runner.Run("git", "-C", path, "rev-parse", "--git-dir")
	if err != nil {
		return false, nil
	}
	return true, nil
}

// GetBranches returns a list of local and remote branch names for the repository.
func (c *Client) GetBranches(repoPath string) ([]string, error) {
	res, err := c.runner.Run("git", "-C", repoPath, "branch", "--format=%(refname:short)", "-a")
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %s: %w", strings.TrimSpace(res.Stderr), err)
	}

	var branches []string
	seen := make(map[string]bool)

	for _, line := range res.Lines() {
		if name, ok := parseBranchName(line); ok {
			if !seen[name] {
				seen[name] = true
				branches = append(branches, name)
			}
		}
	}

	return branches, nil
}

// parseBranchName extracts and cleans the branch name from a raw output line.
func parseBranchName(line string) (string, bool) {
	name := strings.TrimSpace(line)
	if name == "" {
		return "", false
	}
	// Clean up remote branch prefixes like "origin/"
	if strings.HasPrefix(name, "remotes/") {
		name = strings.TrimPrefix(name, "remotes/")
		// Skip HEAD pointer references
		if strings.Contains(name, "/HEAD") {
			return "", false
		}
		parts := strings.SplitN(name, "/", 2)
		if len(parts) > 1 {
			name = parts[1]
		}
	}
	return name, true
}

// CreateWorktree creates a new Git worktree at worktreePath using the specified branch.
// If the branch does not exist, it can be created.
func (c *Client) CreateWorktree(repoPath, branchName, worktreePath string, createBranch bool) error {
	args := []string{"-C", repoPath, "worktree", "add", worktreePath}
	if createBranch {
		args = append(args, "-b", branchName)
	} else {
		args = append(args, branchName)
	}

	res, err := c.runner.Run("git", args...)
	if err != nil {
		return fmt.Errorf("failed to create worktree: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return nil
}

// RemoveWorktree forcefully removes a Git worktree.
func (c *Client) RemoveWorktree(repoPath, worktreePath string) error {
	res, err := c.runner.Run("git", "-C", repoPath, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return nil
}

// Status represents the Git repository working directory state.
type Status struct {
	Modified  int
	Untracked int
}

// String formats the Status as a user-friendly string (e.g., "Clean" or "3 modified, 2 untracked").
func (s Status) String() string {
	if s.Modified == 0 && s.Untracked == 0 {
		return "Clean"
	}
	var parts []string
	if s.Modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", s.Modified))
	}
	if s.Untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", s.Untracked))
	}
	return strings.Join(parts, ", ")
}

// GetStatus gets a brief status summary of the git directory.
func (c *Client) GetStatus(path string) (Status, error) {
	res, err := c.runner.Run("git", "-C", path, "status", "--porcelain")
	if err != nil {
		return Status{}, fmt.Errorf("failed to get git status: %s: %w", strings.TrimSpace(res.Stderr), err)
	}

	lines := res.Lines()
	if len(lines) == 0 {
		return Status{}, nil
	}
	return parseStatusLines(lines), nil
}

// parseStatusLines parses git status lines into a Status object.
func parseStatusLines(lines []string) Status {
	var s Status
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		if line[:2] == "??" {
			s.Untracked++
		} else {
			s.Modified++
		}
	}
	return s
}
