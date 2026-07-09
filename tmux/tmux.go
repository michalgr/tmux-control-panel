package tmux

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"tmux-control-panel/run"
)

// Session represents a Tmux session with its metadata.
type Session struct {
	Name             string
	Windows          int
	Attached         bool
	Created          time.Time
	Path             string
	WorktreePath     string
	ActiveWindowName string
}

// ErrNoServer is returned when tmux server is not running.
var ErrNoServer = errors.New("tmux server not running")

type Client struct {
	runner run.Runner
}

// NewClient creates a new Tmux client with the given runner.
func NewClient(r run.Runner) *Client {
	return &Client{runner: r}
}

// IsTmuxInstalled checks if tmux is present in the PATH.
func (c *Client) IsTmuxInstalled() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// ListSessions queries tmux for all active sessions and compiles their metadata.
func (c *Client) ListSessions() ([]Session, error) {
	if !c.IsTmuxInstalled() {
		return nil, fmt.Errorf("tmux is not installed or not in PATH")
	}

	sessions, err := c.fetchSessions()
	if err != nil {
		return nil, err
	}

	sessionMap := make(map[string]*Session)
	for i := range sessions {
		sessionMap[sessions[i].Name] = &sessions[i]
	}

	if err := c.populateActiveWindows(sessionMap); err != nil {
		return nil, err
	}
	if err := c.populateWorktreePaths(sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (c *Client) fetchSessions() ([]Session, error) {
	res, err := c.runner.Run("tmux", "list-sessions", "-F", "#{session_name};#{session_windows};#{session_attached};#{session_created};#{session_path}")
	if err != nil {
		if strings.Contains(res.Stderr, "no server running") {
			return nil, ErrNoServer
		}
		return nil, fmt.Errorf("failed to list sessions: %s: %w", strings.TrimSpace(res.Stderr), err)
	}

	var sessions []Session
	for _, line := range res.Lines() {
		if sess, ok := parseSession(line); ok {
			sessions = append(sessions, sess)
		}
	}
	return sessions, nil
}

func (c *Client) populateActiveWindows(sessionMap map[string]*Session) error {
	res, err := c.runner.Run("tmux", "list-windows", "-a", "-F", "#{session_name};#{window_name};#{window_active}")
	if err != nil {
		return fmt.Errorf("failed to list windows: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	for _, line := range res.Lines() {
		if win, ok := parseWindowInfo(line); ok && win.active {
			if sess, ok := sessionMap[win.sessionName]; ok {
				sess.ActiveWindowName = win.windowName
			}
		}
	}
	return nil
}

func (c *Client) populateWorktreePaths(sessions []Session) error {
	for i := range sessions {
		sess := &sessions[i]
		res, err := c.runner.Run("tmux", "show-hooks", "-t", sess.Name)
		if err != nil {
			return fmt.Errorf("failed to show hooks for session %s: %s: %w", sess.Name, strings.TrimSpace(res.Stderr), err)
		}
		for _, hLine := range res.Lines() {
			if wtPath, ok := parseHookForWorktree(hLine); ok {
				sess.WorktreePath = wtPath
				break
			}
		}
	}
	return nil
}

// parseSession parses a Session struct out of a raw list-sessions format line.
func parseSession(line string) (Session, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Session{}, false
	}
	parts := strings.Split(line, ";")
	if len(parts) < 5 {
		return Session{}, false
	}

	name := parts[0]
	windows, _ := strconv.Atoi(parts[1])
	attached := parts[2] != "0"
	createdUnix, _ := strconv.ParseInt(parts[3], 10, 64)
	path := parts[4]

	return Session{
		Name:     name,
		Windows:  windows,
		Attached: attached,
		Created:  time.Unix(createdUnix, 0),
		Path:     path,
	}, true
}

type windowInfo struct {
	sessionName string
	windowName  string
	active      bool
}

// parseWindowInfo parses window metadata from a raw list-windows format line.
func parseWindowInfo(line string) (windowInfo, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return windowInfo{}, false
	}
	parts := strings.Split(line, ";")
	if len(parts) < 3 {
		return windowInfo{}, false
	}
	return windowInfo{
		sessionName: parts[0],
		windowName:  parts[1],
		active:      parts[2] == "1",
	}, true
}

// parseHookForWorktree parses a session-closed hook to extract the Git worktree path.
func parseHookForWorktree(line string) (string, bool) {
	if strings.Contains(line, "session-closed") && strings.Contains(line, "worktree remove") {
		return parseWorktreePathFromHook(line), true
	}
	return "", false
}

// parseWorktreePathFromHook extracts the path from a hook command like:
// session-closed[...] run-shell "git -C /repo/path worktree remove --force /path/to/worktree"
func parseWorktreePathFromHook(hookLine string) string {
	idx := strings.Index(hookLine, "worktree remove --force ")
	if idx == -1 {
		idx = strings.Index(hookLine, "worktree remove ")
		if idx == -1 {
			return ""
		}
		idx += len("worktree remove ")
	} else {
		idx += len("worktree remove --force ")
	}

	pathPart := hookLine[idx:]
	// Clean up quotes and trailing characters
	pathPart = strings.TrimSpace(pathPart)
	pathPart = strings.Trim(pathPart, `"'`)
	return pathPart
}

// CreateSession creates a standard Tmux session.
func (c *Client) CreateSession(name, path string) error {
	res, err := c.runner.Run("tmux", "new-session", "-d", "-s", name, "-c", path)
	if err != nil {
		return fmt.Errorf("failed to create tmux session: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return nil
}

// CreateWorktreeSession creates a Git worktree first and then launches a tmux session rooted there.
// It sets a session-closed hook to delete the worktree when the session is closed.
func (c *Client) CreateWorktreeSession(name, repoPath, branchName, worktreePath string, createBranch bool) error {
	res, err := c.runner.Run("tmux", "new-session", "-d", "-s", name, "-c", worktreePath)
	if err != nil {
		return fmt.Errorf("failed to start tmux session in worktree: %s: %w", strings.TrimSpace(res.Stderr), err)
	}

	return c.setSessionClosedHook(name, repoPath, worktreePath)
}

func (c *Client) setSessionClosedHook(name, repoPath, worktreePath string) error {
	cleanupCmd := fmt.Sprintf(`git -C "%s" worktree remove --force "%s"`, repoPath, worktreePath)
	log.Printf("[tmux] Setting session-closed hook for session %s to remove worktree: %s", name, worktreePath)
	res, err := c.runner.Run("tmux", "set-hook", "-t", name, "session-closed", fmt.Sprintf("run-shell '%s'", cleanupCmd))
	if err != nil {
		log.Printf("[tmux] Failed to set session-closed hook: %v", err)
		if _, errKill := c.runner.Run("tmux", "kill-session", "-t", name); errKill != nil {
			log.Printf("[tmux] Failed to kill session %s during hook failure cleanup: %v", name, errKill)
		}
		return fmt.Errorf("failed to set tmux session-closed hook: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return nil
}

// KillSession terminates a tmux session.
func (c *Client) KillSession(name string) error {
	res, err := c.runner.Run("tmux", "kill-session", "-t", name)
	if err != nil {
		return fmt.Errorf("failed to kill tmux session: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return nil
}

// HasSession checks if a session name exists.
func (c *Client) HasSession(name string) (bool, error) {
	_, err := c.runner.Run("tmux", "has-session", "-t", name)
	if err == nil {
		return true, nil
	}
	// If exit code is not 0, tmux indicates the session does not exist.
	return false, nil
}

// SetupGlobalClosedHook registers a global session-closed hook (at index 99)
// to automatically clean up temporary Git worktrees when a session is closed.
func (c *Client) SetupGlobalClosedHook(worktreesDir string) error {
	cmd := fmt.Sprintf(`if [ -d "%s/#{hook_session_name}" ]; then git -C "%s/#{hook_session_name}" worktree remove --force "%s/#{hook_session_name}"; fi`, worktreesDir, worktreesDir, worktreesDir)
	log.Printf("[tmux] Registering global session-closed[99] hook to clean up worktrees in: %s", worktreesDir)
	res, err := c.runner.Run("tmux", "set-hook", "-g", "session-closed[99]", fmt.Sprintf("run-shell '%s'", cmd))
	if err != nil {
		return fmt.Errorf("failed to set global session-closed hook: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return nil
}
