package run

import (
	"bytes"
	"os/exec"
	"strings"
)

// CommandResult encapsulates the stdout and stderr output of a command.
type CommandResult struct {
	Stdout string
	Stderr string
}

// Lines returns the stdout split into non-empty lines.
func (r CommandResult) Lines() []string {
	s := strings.TrimSpace(r.Stdout)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Runner defines the interface for running commands.
type Runner interface {
	Run(name string, args ...string) (CommandResult, error)
}

// DefaultRunner executes real commands on the system.
type DefaultRunner struct{}

// Run executes the command with the given name and arguments.
func (d DefaultRunner) Run(name string, args ...string) (CommandResult, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, err
}

// MockRunner allows passing a custom execution function for testing.
type MockRunner struct {
	RunFunc func(name string, args ...string) (CommandResult, error)
}

// Run executes the custom RunFunc.
func (m MockRunner) Run(name string, args ...string) (CommandResult, error) {
	return m.RunFunc(name, args...)
}

// NewMockRunner creates a Runner from a custom function.
func NewMockRunner(f func(name string, args ...string) (CommandResult, error)) Runner {
	return MockRunner{RunFunc: f}
}
