# Tmux Control Panel (tmux-cp)

A tailored, snappy, and modern Terminal User Interface (TUI) designed to seamlessly manage tmux sessions and temporary Git worktrees, filling a key utility gap in michalgr's personal terminal environment and developer workflow.

## 🚀 Tech Stack

* **Language**: Go (v1.26+)
* **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (for Elm-architecture based UI cycle)
* **Styling & Layout**: [Lip Gloss](https://github.com/charmbracelet/lipgloss) (for beautiful colors, borders, and typography)
* **Components**: [Bubbles](https://github.com/charmbracelet/bubbles) (for inputs, lists, and viewports)
* **Tmux Interface**: Executing `tmux` CLI commands directly for robust compatibility.

---

## 🎨 Visual Design

* **Color Palette**: Dark mode-first palette utilizing high-contrast, premium accents (Deep Violet, Electric Magenta, Bright Cyan, Slate Grey, and Soft Amber).
* **Layout**: A split-pane layout:
  * **Left Sidebar**: List of active tmux sessions with status indicators (`●` Attached, `○` Detached) and quick info.
  * **Right Panel**: Detailed metadata pane showing windows/panes, uptime, active directory, and associated Git worktree details (branch, path, git status).
  * **Bottom Footer**: Snappy hotkey bar mapping operations to simple keys.
* **Modals**: Inline overlay panels for inputs (e.g., creating a new session, setting up a Git worktree, confirmation dialogs).

---

## ⚙️ Features & Architecture

### 1. Session Manager
* List active tmux sessions with details:
  * Session Name
  * Number of windows
  * Active window/pane name
  * Attachment status (attached clients count)
  * Creation time
* Kill selected sessions (with safety confirmation).
* Attach to a session directly (by replacing the process or printing instructions/running nested tmux).

### 2. Session Creation (Standard & Git Worktree)
* **Standard Session**: Create a normal session from a specified directory.
* **Git Worktree Session**:
  * Create a temporary Git worktree from a base repository.
  * Launch a new tmux session rooted inside the newly created worktree.
  * Automatically clean up the Git worktree when the tmux session is closed.

### 3. Native Worktree Cleanup (via Tmux Hooks)
To keep the application stateless and robust without requiring a daemon, we utilize tmux's built-in event hooks:
```sh
# Create session inside the worktree
tmux new-session -d -s <session_name> -c <worktree_path>

# Set a session-specific hook to delete the worktree when the session closes
tmux set-hook -t <session_name> session-closed "run-shell 'git worktree remove --force <worktree_path>'"
```
This ensures that the worktree is cleaned up immediately whenever the user exits the last shell in that tmux session, regardless of whether the control panel TUI is running.

---

## 📂 Repository Layout

```
.
├── README.md
├── .agents/
│   └── AGENTS.md            # Agent instructions and rules
├── go.mod
├── go.sum
├── main.go                  # Application entry point
├── ui/                      # Bubble Tea UI components
│   ├── ui.go                # Main model and update loop
│   ├── ui_test.go           # In-memory ViewState transition unit tests
│   ├── session_list.go      # Sidebar widget
│   ├── detail_pane.go       # Details widget
│   └── forms.go             # Generic form layout rendering
├── tmux/                    # Wrapper library for Tmux CLI
│   ├── tmux.go              # Client methods (list, create, kill sessions)
│   └── tmux_test.go         # Mocked unit tests for Tmux commands
├── git/                     # Git worktree helpers
│   ├── git.go               # Client methods (worktree add/remove, status)
│   └── git_test.go          # Mocked unit tests for Git commands
└── run/                     # Shared command execution package
    └── run.go               # Runner interface, DefaultRunner, and MockRunner helpers
```

## 🧪 Testing

To run the unit test suite (which tests all Git commands, Tmux commands, and UI State transitions in-memory via mock runners):

```sh
go test -v ./...
```
