package ui

import (
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
			ActivePaneName: "main-pane",
			Attached:       true,
			StatusLine:     "Agent is busy writing code",
		},
		{
			Name:           "session-no-status",
			Windows:        1,
			ActivePaneName: "zsh-pane",
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
	if !strings.Contains(view, "main-pane") {
		t.Errorf("expected view to contain active pane name 'main-pane', got:\n%s", view)
	}
	if !strings.Contains(view, "zsh-pane") {
		t.Errorf("expected view to contain active pane name 'zsh-pane', got:\n%s", view)
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
