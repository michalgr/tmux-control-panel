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
	StatusLine       string
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

	return sessions, nil
}

func (c *Client) fetchSessions() ([]Session, error) {
	res, err := c.runner.Run("tmux", "list-sessions", "-F", "#{session_name};#{session_windows};#{session_attached};#{session_created};#{session_path};#{@worktree_path};#{@status_line}")
	if err != nil {
		if strings.Contains(res.Stderr, "no server running") || strings.Contains(res.Stderr, "error connecting to") {
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

	var worktreePath string
	if len(parts) > 5 {
		worktreePath = parts[5]
	}

	var statusLine string
	if len(parts) > 6 {
		statusLine = parts[6]
	}

	return Session{
		Name:         name,
		Windows:      windows,
		Attached:     attached,
		Created:      time.Unix(createdUnix, 0),
		Path:         path,
		WorktreePath: worktreePath,
		StatusLine:   statusLine,
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

// CreateSession creates a standard Tmux session.
func (c *Client) CreateSession(name, path string) error {
	res, err := c.runner.Run("tmux", "new-session", "-d", "-s", name, "-c", path)
	if err != nil {
		return fmt.Errorf("failed to create tmux session: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return nil
}

// SetSessionOption sets a session-specific option in Tmux.
func (c *Client) SetSessionOption(sessionName, option, value string) error {
	res, err := c.runner.Run("tmux", "set-option", "-t", sessionName, option, value)
	if err != nil {
		return fmt.Errorf("failed to set tmux option: %s: %w", strings.TrimSpace(res.Stderr), err)
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

// SetupHook registers a hook in Tmux.
func (c *Client) SetupHook(global bool, target, name, command string) error {
	args := []string{"set-hook"}
	if global {
		args = append(args, "-g")
	}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, name, command)

	res, err := c.runner.Run("tmux", args...)
	if err != nil {
		if strings.Contains(res.Stderr, "no server running") || strings.Contains(res.Stderr, "error connecting to") {
			log.Printf("[tmux] Skipping hook setup: tmux server not running")
			return nil
		}
		return fmt.Errorf("failed to set hook: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return nil
}

// GetSessionOption queries a session-specific option from Tmux.
func (c *Client) GetSessionOption(sessionName, option string) (string, error) {
	res, err := c.runner.Run("tmux", "show-options", "-q", "-t", sessionName, "-v", option)
	if err != nil {
		return "", fmt.Errorf("failed to get tmux option: %s: %w", strings.TrimSpace(res.Stderr), err)
	}
	return strings.TrimSpace(res.Stdout), nil
}
