package tmux

import (
	"errors"
	"testing"
	"time"

	"tmux-control-panel/run"
)

func TestListSessions(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "tmux" {
			switch args[0] {
			case "list-sessions":
				return run.CommandResult{
					Stdout: "my-session;2;0;1609459200;/some/path\n",
				}, nil
			case "list-windows":
				return run.CommandResult{
					Stdout: "my-session;main-win;1\nmy-session;other-win;0\n",
				}, nil
			case "show-hooks":
				return run.CommandResult{
					Stdout: "session-closed run-shell 'git worktree remove --force /some/worktree'\n",
				}, nil
			}
		}
		return run.CommandResult{}, errors.New("unexpected command")
	})

	c := NewClient(mock)
	sessions, err := c.ListSessions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	sess := sessions[0]
	if sess.Name != "my-session" {
		t.Errorf("expected session name 'my-session', got %q", sess.Name)
	}
	if sess.Windows != 2 {
		t.Errorf("expected 2 windows, got %d", sess.Windows)
	}
	if sess.Attached {
		t.Error("expected session to be detached")
	}
	if sess.Path != "/some/path" {
		t.Errorf("expected path '/some/path', got %q", sess.Path)
	}
	if sess.ActiveWindowName != "main-win" {
		t.Errorf("expected active window name 'main-win', got %q", sess.ActiveWindowName)
	}
	if sess.WorktreePath != "/some/worktree" {
		t.Errorf("expected worktree path '/some/worktree', got %q", sess.WorktreePath)
	}

	expectedTime := time.Unix(1609459200, 0)
	if !sess.Created.Equal(expectedTime) {
		t.Errorf("expected created time %v, got %v", expectedTime, sess.Created)
	}
}

func TestHasSession(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "tmux" && args[0] == "has-session" && args[2] == "existing" {
			return run.CommandResult{}, nil
		}
		return run.CommandResult{}, errors.New("session not found")
	})

	c := NewClient(mock)
	has, err := c.HasSession("existing")
	if err != nil {
		t.Errorf("unexpected error for existing session: %v", err)
	}
	if !has {
		t.Error("expected HasSession to return true for existing session")
	}

	has, err = c.HasSession("missing")
	if has {
		t.Error("expected HasSession to return false for missing session")
	}
}

func TestCreateWorktreeSession(t *testing.T) {
	var calledCommands [][]string
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		cmd := append([]string{name}, args...)
		calledCommands = append(calledCommands, cmd)
		return run.CommandResult{}, nil
	})

	c := NewClient(mock)
	err := c.CreateWorktreeSession("my-wt-sess", "/my/repo", "my-branch", "/my/wt/path", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calledCommands) != 2 {
		t.Fatalf("expected 2 tmux commands, got %d", len(calledCommands))
	}

	// Verify first command is tmux new-session
	expectedNewSess := []string{"tmux", "new-session", "-d", "-s", "my-wt-sess", "-c", "/my/wt/path"}
	for i, val := range expectedNewSess {
		if calledCommands[0][i] != val {
			t.Errorf("expected arg %d of first command to be %q, got %q", i, val, calledCommands[0][i])
		}
	}

	// Verify second command is tmux set-hook
	expectedSetHook := []string{
		"tmux", "set-hook", "-t", "my-wt-sess", "session-closed",
		`run-shell 'git -C "/my/repo" worktree remove --force "/my/wt/path"'`,
	}
	for i, val := range expectedSetHook {
		if calledCommands[1][i] != val {
			t.Errorf("expected arg %d of second command to be %q, got %q", i, val, calledCommands[1][i])
		}
	}
}

func TestParseHookForWorktree(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		expectedPath string
		expectedOk   bool
	}{
		{
			name:         "Old style with force",
			line:         "session-closed run-shell 'git worktree remove --force /some/worktree'",
			expectedPath: "/some/worktree",
			expectedOk:   true,
		},
		{
			name:         "Old style without force",
			line:         "session-closed run-shell 'git worktree remove /some/worktree'",
			expectedPath: "/some/worktree",
			expectedOk:   true,
		},
		{
			name:         "New style with -C flag and quotes",
			line:         `session-closed run-shell 'git -C "/repo/path" worktree remove --force "/some/worktree"'`,
			expectedPath: "/some/worktree",
			expectedOk:   true,
		},
		{
			name:         "Non-matching hook",
			line:         "session-closed run-shell 'echo hello'",
			expectedPath: "",
			expectedOk:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, ok := parseHookForWorktree(tc.line)
			if ok != tc.expectedOk {
				t.Errorf("expected ok to be %t, got %t", tc.expectedOk, ok)
			}
			if path != tc.expectedPath {
				t.Errorf("expected path to be %q, got %q", tc.expectedPath, path)
			}
		})
	}
}
