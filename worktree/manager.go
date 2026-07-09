package worktree

import (
	"fmt"
	"log"
	"tmux-control-panel/git"
	"tmux-control-panel/tmux"
)

// Manager coordinates operations between Git and Tmux for worktree sessions.
type Manager struct {
	gitClient    *git.Client
	tmuxClient   *tmux.Client
	worktreesDir string
}

// NewManager creates a new worktree Manager.
func NewManager(gc *git.Client, tc *tmux.Client, worktreesDir string) *Manager {
	return &Manager{
		gitClient:    gc,
		tmuxClient:   tc,
		worktreesDir: worktreesDir,
	}
}

// CreateWorktreeSession sets up a git worktree, starts a tmux session in it,
// configures its metadata, and registers the global session-closed cleanup hook.
func (m *Manager) CreateWorktreeSession(repoPath, branchName, sessionName, worktreePath string, createBranch bool) error {
	if err := m.gitClient.CreateWorktree(repoPath, branchName, worktreePath, createBranch); err != nil {
		return err
	}

	if err := m.tmuxClient.CreateSession(sessionName, worktreePath); err != nil {
		if errRm := m.gitClient.RemoveWorktree(repoPath, worktreePath); errRm != nil {
			log.Printf("[worktree-manager] Failed to clean up worktree path %s: %v", worktreePath, errRm)
		}
		return err
	}

	if err := m.tmuxClient.SetSessionOption(sessionName, "@worktree_path", worktreePath); err != nil {
		log.Printf("[worktree-manager] Failed to set @worktree_path: %v", err)
		if errKill := m.tmuxClient.KillSession(sessionName); errKill != nil {
			log.Printf("[worktree-manager] Failed to kill session %s: %v", sessionName, errKill)
		}
		if errRm := m.gitClient.RemoveWorktree(repoPath, worktreePath); errRm != nil {
			log.Printf("[worktree-manager] Failed to remove worktree %s: %v", worktreePath, errRm)
		}
		return fmt.Errorf("failed to set tmux worktree metadata: %w", err)
	}

	if err := m.SetupClosedHook(); err != nil {
		log.Printf("[worktree-manager] Failed to setup global hook: %v", err)
	}

	return nil
}

// SetupClosedHook registers the global session-closed hook in tmux to clean up
// temporary Git worktrees when a session is closed.
func (m *Manager) SetupClosedHook() error {
	cmd := fmt.Sprintf(`if [ -d "%s/#{hook_session_name}" ]; then git -C "%s/#{hook_session_name}" worktree remove --force "%s/#{hook_session_name}"; fi`, m.worktreesDir, m.worktreesDir, m.worktreesDir)
	log.Printf("[worktree-manager] Registering global session-closed[99] hook to clean up worktrees in: %s", m.worktreesDir)
	return m.tmuxClient.SetupHook(true, "", "session-closed[99]", fmt.Sprintf("run-shell '%s'", cmd))
}
