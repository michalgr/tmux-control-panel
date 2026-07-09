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
					Stdout: "my-session;2;0;1609459200;/some/path;/some/worktree;running build\n",
				}, nil
			case "list-windows":
				return run.CommandResult{
					Stdout: "my-session;main-win;1;zsh\nmy-session;other-win;0;nvim\n",
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
	if sess.ActivePaneName != "zsh" {
		t.Errorf("expected active pane name 'zsh', got %q", sess.ActivePaneName)
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

func TestSetSessionOption(t *testing.T) {
	var calledCommands [][]string
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		cmd := append([]string{name}, args...)
		calledCommands = append(calledCommands, cmd)
		return run.CommandResult{}, nil
	})

	c := NewClient(mock)
	err := c.SetSessionOption("my-sess", "@worktree_path", "/my/wt/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calledCommands) != 1 {
		t.Fatalf("expected 1 tmux command, got %d", len(calledCommands))
	}

	expected := []string{"tmux", "set-option", "-t", "my-sess", "@worktree_path", "/my/wt/path"}
	for i, val := range expected {
		if calledCommands[0][i] != val {
			t.Errorf("expected arg %d of command to be %q, got %q", i, val, calledCommands[0][i])
		}
	}
}

func TestGetSessionOption(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var calledCommands [][]string
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			cmd := append([]string{name}, args...)
			calledCommands = append(calledCommands, cmd)
			if name == "tmux" && args[0] == "show-options" {
				return run.CommandResult{
					Stdout: "/my/wt/path\n",
				}, nil
			}
			return run.CommandResult{}, errors.New("unexpected command")
		})

		c := NewClient(mock)
		val, err := c.GetSessionOption("my-sess", "@worktree_path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if val != "/my/wt/path" {
			t.Errorf("expected option value '/my/wt/path', got %q", val)
		}

		if len(calledCommands) != 1 {
			t.Fatalf("expected 1 tmux command, got %d", len(calledCommands))
		}

		expected := []string{"tmux", "show-options", "-q", "-t", "my-sess", "-v", "@worktree_path"}
		for i, val := range expected {
			if calledCommands[0][i] != val {
				t.Errorf("expected arg %d of command to be %q, got %q", i, val, calledCommands[0][i])
			}
		}
	})

	t.Run("failure", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{Stderr: "unknown option"}, errors.New("command failed")
		})

		c := NewClient(mock)
		_, err := c.GetSessionOption("my-sess", "@invalid_option")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSetupHook(t *testing.T) {
	var calledCommands [][]string
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		cmd := append([]string{name}, args...)
		calledCommands = append(calledCommands, cmd)
		return run.CommandResult{}, nil
	})

	c := NewClient(mock)
	err := c.SetupHook(true, "sess-name", "session-closed", "echo test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calledCommands) != 1 {
		t.Fatalf("expected 1 tmux command, got %d", len(calledCommands))
	}

	expectedCmd := []string{
		"tmux", "set-hook", "-g", "-t", "sess-name", "session-closed", "echo test",
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
						Stdout: "my-session;2;0;1609459200;/some/path;;\n",
					}, nil
				case "list-windows":
					return run.CommandResult{
						Stdout: "my-session;main-win;1;zsh\nmy-session;other-win;0;nvim\n",
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
}

func TestSetSessionOption_Failure(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		return run.CommandResult{Stderr: "failed setting option"}, errors.New("set-option error")
	})

	c := NewClient(mock)
	err := c.SetSessionOption("my-sess", "@worktree_path", "/my/wt/path")
	if err == nil {
		t.Fatal("expected error, got nil")
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
					return run.CommandResult{Stdout: "my-session;2;0;1609459200;/some/path;;\n"}, nil
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

func TestSetupHook_NoServer(t *testing.T) {
	t.Run("no server running", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{Stderr: "no server running on /tmp/tmux-1000/default\n"}, errors.New("exit status 1")
		})
		c := NewClient(mock)
		err := c.SetupHook(true, "", "session-closed", "echo test")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("error connecting to", func(t *testing.T) {
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{Stderr: "error connecting to /tmp/tmux-1001/test-socket (No such file or directory)\n"}, errors.New("exit status 1")
		})
		c := NewClient(mock)
		err := c.SetupHook(true, "", "session-closed", "echo test")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

func TestListSessions_ErrorConnectingTo(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		return run.CommandResult{Stderr: "error connecting to /tmp/tmux-1001/test-socket (No such file or directory)\n"}, errors.New("exit status 1")
	})

	c := NewClient(mock)
	_, err := c.ListSessions()
	if !errors.Is(err, ErrNoServer) {
		t.Errorf("expected ErrNoServer, got %v", err)
	}
}

func TestCurrentSessionName(t *testing.T) {
	t.Run("outside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "")
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{}, errors.New("should not be called")
		})
		c := NewClient(mock)
		name, err := c.CurrentSessionName()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "" {
			t.Errorf("expected empty session name, got %q", name)
		}
	})

	t.Run("inside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
		mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			if name == "tmux" && args[0] == "display-message" && args[1] == "-p" && args[2] == "#S" {
				return run.CommandResult{Stdout: "my-current-session\n"}, nil
			}
			return run.CommandResult{}, errors.New("unexpected command")
		})
		c := NewClient(mock)
		name, err := c.CurrentSessionName()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "my-current-session" {
			t.Errorf("expected session name 'my-current-session', got %q", name)
		}
	})
}

func TestSwitchClient(t *testing.T) {
	var calledCommands [][]string
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		calledCommands = append(calledCommands, append([]string{name}, args...))
		return run.CommandResult{}, nil
	})

	c := NewClient(mock)
	err := c.SwitchClient("target-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calledCommands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(calledCommands))
	}

	expected := []string{"tmux", "switch-client", "-t", "target-session"}
	for i, val := range expected {
		if calledCommands[0][i] != val {
			t.Errorf("arg %d mismatch: expected %q, got %q", i, val, calledCommands[0][i])
		}
	}
}
