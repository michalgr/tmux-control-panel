package git

import (
	"errors"
	"testing"

	"tmux-control-panel/run"
)

func TestInsideWorkTree(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "git" && args[2] == "rev-parse" {
			return run.CommandResult{Stdout: "true\n"}, nil
		}
		return run.CommandResult{}, errors.New("command failed")
	})

	c := NewClient(mock)
	isInside, err := c.InsideWorkTree("/some/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isInside {
		t.Error("expected InsideWorkTree to return true")
	}
}

func TestGetBranches(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "git" && args[2] == "branch" {
			return run.CommandResult{
				Stdout: "  main\n  feature-branch\n  remotes/origin/remote-branch\n  remotes/origin/HEAD -> origin/main\n",
			}, nil
		}
		return run.CommandResult{}, errors.New("command failed")
	})

	c := NewClient(mock)
	branches, err := c.GetBranches("/some/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"main", "feature-branch", "remote-branch"}
	if len(branches) != len(expected) {
		t.Fatalf("expected %d branches, got %d", len(expected), len(branches))
	}

	for i, name := range branches {
		if name != expected[i] {
			t.Errorf("expected branch %d to be %q, got %q", i, expected[i], name)
		}
	}
}

func TestGetStatus(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "git" && args[2] == "status" {
			return run.CommandResult{
				Stdout: " M modified-file.go\n?? untracked-file.go\n?? another-untracked.go\n",
			}, nil
		}
		return run.CommandResult{}, errors.New("command failed")
	})

	c := NewClient(mock)
	status, err := c.GetStatus("/some/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Modified != 1 {
		t.Errorf("expected 1 modified file, got %d", status.Modified)
	}
	if status.Untracked != 2 {
		t.Errorf("expected 2 untracked files, got %d", status.Untracked)
	}

	if status.String() != "1 modified, 2 untracked" {
		t.Errorf("expected string '1 modified, 2 untracked', got %q", status.String())
	}
}

func TestIsGitRepository(t *testing.T) {
	mock := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		if name == "git" && args[2] == "rev-parse" && args[3] == "--git-dir" {
			return run.CommandResult{Stdout: ".git\n"}, nil
		}
		return run.CommandResult{}, errors.New("command failed")
	})

	c := NewClient(mock)
	isRepo, err := c.IsGitRepository("/some/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isRepo {
		t.Error("expected IsGitRepository to return true")
	}
}
