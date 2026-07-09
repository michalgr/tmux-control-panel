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
			case "show-options":
				if len(args) >= 6 && args[1] == "-q" && args[2] == "-t" && args[3] == "my-session" && args[4] == "-v" {
					if args[5] == "@worktree_path" {
						return run.CommandResult{
							Stdout: "/some/worktree\n",
						}, nil
					}
					if args[5] == "@status_line" {
						return run.CommandResult{
							Stdout: "running build\n",
						}, nil
					}
				}
				return run.CommandResult{}, errors.New("unexpected show-options args")
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
	if sess.StatusLine != "running build" {
		t.Errorf("expected status line 'running build', got %q", sess.StatusLine)
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
	err := c.CreateWorktreeSession("my-wt-sess", "/my/wt/path")
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

	// Verify second command is tmux set-option
	expectedSetOption := []string{
		"tmux", "set-option", "-t", "my-wt-sess", "@worktree_path", "/my/wt/path",
	}
	if len(calledCommands[1]) != len(expectedSetOption) {
		t.Fatalf("expected second command length %d, got %d", len(expectedSetOption), len(calledCommands[1]))
	}
	for i, val := range expectedSetOption {
		if calledCommands[1][i] != val {
			t.Errorf("expected arg %d of second command to be %q, got %q", i, val, calledCommands[1][i])
		}
	}
}

func TestSetupGlobalClosedHook(t *testing.T) {
	var calledCommands [][]string
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		cmd := append([]string{name}, args...)
		calledCommands = append(calledCommands, cmd)
		return run.CommandResult{}, nil
	})

	c := NewClient(mock)
	err := c.SetupGlobalClosedHook("/my/worktrees/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calledCommands) != 1 {
		t.Fatalf("expected 1 tmux command, got %d", len(calledCommands))
	}

	expectedCmd := []string{
		"tmux", "set-hook", "-g", "session-closed[99]",
		`run-shell 'if [ -d "/my/worktrees/dir/#{hook_session_name}" ]; then git -C "/my/worktrees/dir/#{hook_session_name}" worktree remove --force "/my/worktrees/dir/#{hook_session_name}"; fi'`,
	}

	if len(calledCommands[0]) != len(expectedCmd) {
		t.Fatalf("expected command length %d, got %d", len(expectedCmd), len(calledCommands[0]))
	}

	for i, val := range expectedCmd {
		if calledCommands[0][i] != val {
			t.Errorf("expected arg %d of command to be %q, got %q", i, val, calledCommands[0][i])
		}
	}
}

func TestListSessions_EmptyAndError(t *testing.T) {
	t.Run("empty_worktree_path", func(t *testing.T) {
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
				case "show-options":
					return run.CommandResult{
						Stdout: "\n",
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
		if sessions[0].WorktreePath != "" {
			t.Errorf("expected empty worktree path, got %q", sessions[0].WorktreePath)
		}
	})

	t.Run("display_message_error", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			if name == "tmux" {
				switch args[0] {
				case "list-sessions":
					return run.CommandResult{
						Stdout: "my-session;2;0;1609459200;/some/path\n",
					}, nil
				case "list-windows":
					return run.CommandResult{
						Stdout: "my-session;main-win;1\n",
					}, nil
				case "show-options":
					return run.CommandResult{
						Stderr: "some error\n",
					}, errors.New("command failed")
				}
			}
			return run.CommandResult{}, errors.New("unexpected command")
		})

		c := NewClient(mock)
		_, err := c.ListSessions()
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}

func TestCreateWorktreeSession_Failure(t *testing.T) {
	var calledCommands [][]string
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		cmd := append([]string{name}, args...)
		calledCommands = append(calledCommands, cmd)
		if name == "tmux" && args[0] == "set-option" {
			return run.CommandResult{Stderr: "failed setting option"}, errors.New("set-option error")
		}
		return run.CommandResult{}, nil
	})

	c := NewClient(mock)
	err := c.CreateWorktreeSession("my-wt-sess", "/my/wt/path")
	if err == nil {
		t.Fatal("expected error from CreateWorktreeSession, got nil")
	}

	// We expect 3 commands:
	// 1. new-session
	// 2. set-option
	// 3. kill-session (due to option setup failure)
	if len(calledCommands) != 3 {
		t.Fatalf("expected 3 commands called, got %d", len(calledCommands))
	}

	if calledCommands[2][0] != "tmux" || calledCommands[2][1] != "kill-session" || calledCommands[2][3] != "my-wt-sess" {
		t.Errorf("expected third command to kill the session, got %v", calledCommands[2])
	}
}

func TestCreateSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var calledCommands [][]string
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			cmd := append([]string{name}, args...)
			calledCommands = append(calledCommands, cmd)
			return run.CommandResult{}, nil
		})

		c := NewClient(mock)
		err := c.CreateSession("new-sess", "/some/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(calledCommands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(calledCommands))
		}

		expected := []string{"tmux", "new-session", "-d", "-s", "new-sess", "-c", "/some/path"}
		for i, val := range expected {
			if calledCommands[0][i] != val {
				t.Errorf("expected arg %d to be %q, got %q", i, val, calledCommands[0][i])
			}
		}
	})

	t.Run("failure", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{Stderr: "create session failed"}, errors.New("command failed")
		})

		c := NewClient(mock)
		err := c.CreateSession("new-sess", "/some/path")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestKillSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var calledCommands [][]string
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			cmd := append([]string{name}, args...)
			calledCommands = append(calledCommands, cmd)
			return run.CommandResult{}, nil
		})

		c := NewClient(mock)
		err := c.KillSession("my-sess")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(calledCommands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(calledCommands))
		}

		expected := []string{"tmux", "kill-session", "-t", "my-sess"}
		for i, val := range expected {
			if calledCommands[0][i] != val {
				t.Errorf("expected arg %d to be %q, got %q", i, val, calledCommands[0][i])
			}
		}
	})

	t.Run("failure", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{Stderr: "kill failed"}, errors.New("command failed")
		})

		c := NewClient(mock)
		err := c.KillSession("my-sess")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestListSessions_MoreFailures(t *testing.T) {
	t.Run("no_server_running", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{Stderr: "no server running on /tmp/tmux-1000/default\n"}, errors.New("exit status 1")
		})

		c := NewClient(mock)
		_, err := c.ListSessions()
		if !errors.Is(err, ErrNoServer) {
			t.Errorf("expected ErrNoServer, got %v", err)
		}
	})

	t.Run("list_sessions_general_error", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{Stderr: "permission denied\n"}, errors.New("exit status 1")
		})

		c := NewClient(mock)
		_, err := c.ListSessions()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("list_windows_failure", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			if name == "tmux" {
				switch args[0] {
				case "list-sessions":
					return run.CommandResult{Stdout: "my-session;2;0;1609459200;/some/path\n"}, nil
				case "list-windows":
					return run.CommandResult{Stderr: "list windows failed\n"}, errors.New("command failed")
				}
			}
			return run.CommandResult{}, errors.New("unexpected command")
		})

		c := NewClient(mock)
		_, err := c.ListSessions()
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestCreateWorktreeSession_NewSessionFailure(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "tmux" && args[0] == "new-session" {
			return run.CommandResult{Stderr: "new-session failed"}, errors.New("command failed")
		}
		return run.CommandResult{}, nil
	})

	c := NewClient(mock)
	err := c.CreateWorktreeSession("my-wt-sess", "/my/wt/path")
	if err == nil {
		t.Fatal("expected error from CreateWorktreeSession when new-session fails, got nil")
	}
}

func TestListSessions_EmptyResponse(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "tmux" {
			switch args[0] {
			case "list-sessions":
				return run.CommandResult{Stdout: ""}, nil
			case "list-windows":
				return run.CommandResult{Stdout: ""}, nil
			}
		}
		return run.CommandResult{}, errors.New("unexpected command")
	})

	c := NewClient(mock)
	sessions, err := c.ListSessions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
