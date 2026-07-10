package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"tmux-control-panel/git"
	"tmux-control-panel/run"
	"tmux-control-panel/tmux"
	"tmux-control-panel/ui"
	"tmux-control-panel/worktree"
)

func main() {
	logFile, logger := initLogging()
	if logFile != nil {
		defer logFile.Close()
	}

	tmuxClient, gitClient := initClients()
	verifyTmuxInstalled(tmuxClient)

	worktreesDir := filepath.Join(getCacheDir(), "tmux-cp", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		logger.Printf("Failed to create worktrees directory: %v\n", err)
	}

	wtManager := worktree.NewManager(gitClient, tmuxClient, worktreesDir)

	m := initModel(logger, tmuxClient, gitClient, wtManager)
	runProgram(m, logger)
}

func initClients() (*tmux.Client, *git.Client) {
	runner := run.DefaultRunner{}
	return tmux.NewClient(runner), git.NewClient(runner)
}

func verifyTmuxInstalled(c *tmux.Client) {
	if !c.IsTmuxInstalled() {
		fmt.Fprintln(os.Stderr, "Error: tmux is not installed or not found in PATH.")
		os.Exit(1)
	}
}

func initModel(logger *log.Logger, tc *tmux.Client, gc *git.Client, wt *worktree.Manager) *ui.Model {
	m, err := ui.NewModel(logger, tc, gc, wt)
	if err != nil {
		logger.Printf("Failed to initialize model: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error initializing TUI model: %v\n", err)
		os.Exit(1)
	}
	return m
}

func runProgram(m *ui.Model, logger *log.Logger) {
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		logger.Printf("Fatal TUI runtime error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Alas, there's been a fatal error: %v\n", err)
		os.Exit(1)
	}
	logger.Println("Tmux Control Panel exited cleanly.")
}

func initLogging() (*os.File, *log.Logger) {
	logDir := filepath.Join(getCacheDir(), "tmux-cp")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		os.Exit(1)
	}
	logFilePath := filepath.Join(logDir, "debug.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	log.SetOutput(logFile)
	logger := log.New(logFile, "[tmux-cp] ", log.LstdFlags|log.Lshortfile)
	logger.Printf("Starting Tmux Control Panel TUI... Logging to: %s\n", logFilePath)
	return logFile, logger
}

func getCacheDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return os.TempDir()
	}
	return cacheDir
}
