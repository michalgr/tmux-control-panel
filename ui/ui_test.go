package ui

import (
	"errors"
	"io"
	"log"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"tmux-control-panel/git"
	"tmux-control-panel/run"
	"tmux-control-panel/tmux"
	"tmux-control-panel/worktree"
)

func TestViewStateTransition(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	mockRunner := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		return run.CommandResult{}, nil
	})
	tmuxClient := tmux.NewClient(mockRunner)
	gitClient := git.NewClient(mockRunner)
	wtManager := worktree.NewManager(gitClient, tmuxClient, "/tmp/worktrees")

	model, err := NewModel(logger, tmuxClient, gitClient, wtManager)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// 1. Initially should be NormalState
	if _, ok := model.state.(NormalState); !ok {
		t.Errorf("expected state to be NormalState, got %T", model.state)
	}

	// 2. Transition to CreateSessionNameState via "n" keypress in NormalState
	nextState, _ := model.state.Update(&model, tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("n"),
	})

	nameState, ok := nextState.(CreateSessionNameState)
	if !ok {
		t.Fatalf("expected state to transition to CreateSessionNameState, got %T", nextState)
	}

	// Set standard name inside the state's textInput
	nameState.textInput.SetValue("test-session")
	model.state = nameState

	// 3. Hit enter to transition to CreateSessionPathState
	nextState, _ = model.state.Update(&model, tea.KeyMsg{
		Type: tea.KeyEnter,
	})

	pathState, ok := nextState.(CreateSessionPathState)
	if !ok {
		t.Fatalf("expected state to transition to CreateSessionPathState, got %T", nextState)
	}

	if pathState.name != "test-session" {
		t.Errorf("expected session name in next state to be 'test-session', got %q", pathState.name)
	}
}

func TestSidebarView_StatusLine(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	mockRunner := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		return run.CommandResult{}, nil
	})
	tmuxClient := tmux.NewClient(mockRunner)
	gitClient := git.NewClient(mockRunner)
	wtManager := worktree.NewManager(gitClient, tmuxClient, "/tmp/worktrees")

	model, err := NewModel(logger, tmuxClient, gitClient, wtManager)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	model.sessions = []tmux.Session{
		{
			Name:           "session-with-status",
			Windows:        3,
			ActivePaneName: "nvim",
			Attached:       true,
			StatusLine:     "Agent is busy writing code",
		},
		{
			Name:           "session-no-status",
			Windows:        1,
			ActivePaneName: "zsh",
			Attached:       false,
			StatusLine:     "",
		},
	}
	model.selectedIndex = 0

	view := model.sidebarView(40, 20)

	if !strings.Contains(view, "session-with-status") {
		t.Errorf("expected view to contain session-with-status, got:\n%s", view)
	}
	if !strings.Contains(view, "Agent is busy writing code") {
		t.Errorf("expected view to contain status line 'Agent is busy writing code', got:\n%s", view)
	}
	if !strings.Contains(view, "no status set") {
		t.Errorf("expected view to contain fallback 'no status set', got:\n%s", view)
	}
	if !strings.Contains(view, "nvim") {
		t.Errorf("expected view to contain active pane name 'nvim', got:\n%s", view)
	}
	if !strings.Contains(view, "zsh") {
		t.Errorf("expected view to contain active pane name 'zsh', got:\n%s", view)
	}
}

func TestGetStatusLine_Truncation(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		width    int
		expected string
	}{
		{
			name:     "no status fallback, normal width",
			status:   "",
			width:    20,
			expected: "  no status set",
		},
		{
			name:     "normal status, normal width",
			status:   "active",
			width:    20,
			expected: "  active",
		},
		{
			name:     "truncation case",
			status:   "running tests",
			width:    10, // maxTextWidth = 8, "  running tests" (15 chars) -> "  runnin" (8 chars)
			expected: "  runnin",
		},
		{
			name:     "width too small to fit padding",
			status:   "running",
			width:    2, // maxTextWidth = 0
			expected: "",
		},
		{
			name:     "zero width",
			status:   "running",
			width:    0, // maxTextWidth = 0
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := tmux.Session{StatusLine: tt.status}
			actual := getStatusLine(sess, tt.width)
			if actual != tt.expected {
				t.Errorf("getStatusLine(sess, %d) = %q, expected %q", tt.width, actual, tt.expected)
			}
		})
	}
}

func TestInitialSelection_CurrentActiveSession(t *testing.T) {
	logger := log.New(io.Discard, "", 0)

	t.Run("when inside tmux and session exists", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
		mockRunner := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			if name == "tmux" && args[0] == "display-message" && args[1] == "-p" && args[2] == "#S" {
				return run.CommandResult{Stdout: "session-2\n"}, nil
			}
			return run.CommandResult{}, nil
		})
		tmuxClient := tmux.NewClient(mockRunner)
		gitClient := git.NewClient(mockRunner)
		wtManager := worktree.NewManager(gitClient, tmuxClient, "/tmp/worktrees")

		model, err := NewModel(logger, tmuxClient, gitClient, wtManager)
		if err != nil {
			t.Fatalf("failed to create model: %v", err)
		}

		sessions := []tmux.Session{
			{Name: "session-1"},
			{Name: "session-2"},
			{Name: "session-3"},
		}

		// Send sessions list to simulate first load
		resModel, _ := model.Update(sessions)
		model = resModel.(Model)

		if model.selectedIndex != 1 {
			t.Errorf("expected selectedIndex to be 1 (session-2), got %d", model.selectedIndex)
		}
		if !model.initialLoadDone {
			t.Error("expected initialLoadDone to be true")
		}

		// Simulate second load with same/different sessions, selectedIndex should NOT change automatically
		model.selectedIndex = 2
		resModel, _ = model.Update(sessions)
		model = resModel.(Model)
		if model.selectedIndex != 2 {
			t.Errorf("expected selectedIndex to remain 2, got %d", model.selectedIndex)
		}
	})

	t.Run("when outside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "")
		mockRunner := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
			return run.CommandResult{}, errors.New("should not be called")
		})
		tmuxClient := tmux.NewClient(mockRunner)
		gitClient := git.NewClient(mockRunner)
		wtManager := worktree.NewManager(gitClient, tmuxClient, "/tmp/worktrees")

		model, err := NewModel(logger, tmuxClient, gitClient, wtManager)
		if err != nil {
			t.Fatalf("failed to create model: %v", err)
		}

		sessions := []tmux.Session{
			{Name: "session-1"},
			{Name: "session-2"},
		}

		resModel, _ := model.Update(sessions)
		model = resModel.(Model)

		if model.selectedIndex != 0 {
			t.Errorf("expected selectedIndex to be 0, got %d", model.selectedIndex)
		}
	})
}
