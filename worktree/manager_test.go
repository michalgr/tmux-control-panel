package worktree

import (
	"errors"
	"testing"
	"tmux-control-panel/git"
	"tmux-control-panel/run"
	"tmux-control-panel/tmux"
)

func TestCreateWorktreeSession_Success(t *testing.T) {
	var calledCommands [][]string
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		calledCommands = append(calledCommands, append([]string{name}, args...))
		return run.CommandResult{}, nil
	})

	gc := git.NewClient(mock)
	tc := tmux.NewClient(mock)
	mgr := NewManager(gc, tc, "/my/worktrees/dir")

	err := mgr.CreateWorktreeSession("/my/repo", "my-branch", "my-sess", "/my/worktree/path", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCmds := [][]string{
		{"git", "-C", "/my/repo", "worktree", "add", "/my/worktree/path", "my-branch"},
		{"tmux", "new-session", "-d", "-s", "my-sess", "-c", "/my/worktree/path"},
		{"tmux", "set-option", "-t", "my-sess", "@worktree_path", "/my/worktree/path"},
		{"tmux", "set-hook", "-g", "session-closed[99]", "run-shell 'if [ -d \"/my/worktrees/dir/#{hook_session_name}\" ]; then git -C \"/my/worktrees/dir/#{hook_session_name}\" worktree remove --force \"/my/worktrees/dir/#{hook_session_name}\"; fi'"},
	}

	if len(calledCommands) != len(expectedCmds) {
		t.Fatalf("expected %d commands, got %d", len(expectedCmds), len(calledCommands))
	}

	for i, expected := range expectedCmds {
		if len(calledCommands[i]) != len(expected) {
			t.Fatalf("command %d length mismatch: expected %d, got %d", i, len(expected), len(calledCommands[i]))
		}
		for j, val := range expected {
			if calledCommands[i][j] != val {
				t.Errorf("command %d arg %d mismatch: expected %q, got %q", i, j, val, calledCommands[i][j])
			}
		}
	}
}

func TestCreateWorktreeSession_GitFailure(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "git" && args[3] == "add" {
			return run.CommandResult{Stderr: "git error"}, errors.New("git failed")
		}
		return run.CommandResult{}, nil
	})

	gc := git.NewClient(mock)
	tc := tmux.NewClient(mock)
	mgr := NewManager(gc, tc, "/my/worktrees/dir")

	err := mgr.CreateWorktreeSession("/my/repo", "my-branch", "my-sess", "/my/worktree/path", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateWorktreeSession_TmuxFailure(t *testing.T) {
	var calledCommands [][]string
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		calledCommands = append(calledCommands, append([]string{name}, args...))
		if name == "tmux" && args[0] == "new-session" {
			return run.CommandResult{Stderr: "tmux error"}, errors.New("tmux failed")
		}
		return run.CommandResult{}, nil
	})

	gc := git.NewClient(mock)
	tc := tmux.NewClient(mock)
	mgr := NewManager(gc, tc, "/my/worktrees/dir")

	err := mgr.CreateWorktreeSession("/my/repo", "my-branch", "my-sess", "/my/worktree/path", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	cleanedUp := false
	for _, cmd := range calledCommands {
		if cmd[0] == "git" && len(cmd) > 6 && cmd[4] == "remove" {
			cleanedUp = true
			if cmd[6] != "/my/worktree/path" {
				t.Errorf("expected remove of /my/worktree/path, got %s", cmd[6])
			}
		}
	}
	if !cleanedUp {
		t.Error("expected git worktree cleanup command, but not found")
	}
}
