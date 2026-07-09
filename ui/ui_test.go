package ui

import (
	"io"
	"log"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"tmux-control-panel/git"
	"tmux-control-panel/run"
	"tmux-control-panel/tmux"
)

func TestViewStateTransition(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	mockRunner := run.NewMockRunner(func(name string, args ...string) (run.CommandResult, error) {
		return run.CommandResult{}, nil
	})
	tmuxClient := tmux.NewClient(mockRunner)
	gitClient := git.NewClient(mockRunner)

	model, err := NewModel(logger, tmuxClient, gitClient)
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
