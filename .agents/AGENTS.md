# Agent Guidelines: Tmux Control Panel (tmux-cp)

These rules apply to any agent working on the `tmux-control-panel` codebase.

## 🛠️ Codebase Architecture

1. **Packages**: Keep code modular.
   * `main.go`: Initialization and execution entry point.
   * `tmux`: Handles execution of `tmux` commands and parsing output.
   * `git`: Handles running git commands, checking branches, and worktrees.
   * `run`: Shared execution wrapper (`CommandResult` and `Run`) for commands.
   * `ui`: Holds Bubble Tea models, views, update logic, and view states.
2. **Go Best Practices**:
   * Use Go 1.26 features if appropriate.
   * Always check error returns. Never ignore errors (`_ = ...`) when executing external commands.

## 📺 TUI & Bubble Tea Guidelines

1. **Elm Architecture & ViewState**:
   * Strictly separate Model state, Update messages, and View formatting.
   * Use the `ViewState` interface state-machine pattern for UI modes (Normal, Error, and form wizard steps).
   * Delegate rendering (`state.View(...)`) and message processing (`state.Update(...)`) to the active state.
   * Keep state-specific form properties (like input fields and choices) inside the concrete state structs to keep the central `Model` struct clean.
2. **Standard Output & Logging**:
   * **CRITICAL**: Never print to `stdout` or `stderr` inside the UI or utility libraries while the Bubble Tea program is running. Doing so will corrupt the terminal screen.
   * All logging must go to a file (in the user's cache directory, e.g., `~/.cache/tmux-cp/debug.log`). Use `log.Println` or write a custom logger that redirects output to this log path.
3. **Responsive Layouts**:
   * The TUI will receive window resize messages (`tea.WindowSizeMsg`). You must handle this message and recalculate all component widths and heights dynamically.
   * Use Lip Gloss's padding, borders, and margins to size panes relative to the terminal size.
4. **Visual Aesthetics**:
   * Use high-quality dark mode colors (e.g., `#7D56F4` for violet, `#FF06B3` for magenta, `#00E5FF` for cyan, `#2B2A3A` for dark grey background, `#4E4D63` for slate).
   * Active selection must have a distinct background/foreground color.
   * Keep borders clean using Lip Gloss's built-in border styles (e.g., `lipgloss.RoundedBorder()`).

## 🪝 Safe Tmux & Git Worktree Execution

1. **Command Execution**:
   * Wrap external commands using the shared `run` package.
   * If a command fails (e.g., `tmux` is not running, or a session name already exists), capture its `stderr` output via `run.CommandResult` and return a detailed error.
2. **Tmux Hooks for Worktrees**:
   * When establishing a Git worktree session, immediately set the `session-closed` hook:
     ```sh
     tmux set-hook -t <session_name> session-closed "run-shell 'git worktree remove --force <worktree_path>'"
     ```
   * Log the hook execution details in the user cache directory's debug log to aid in debugging.
3. **Session Attachment**:
   * Because attaching to a tmux session via the TUI replaces the TUI process (`exec` style) or runs nested tmux sessions, design the UI to either:
     1. Exit the TUI and execute `tmux attach-session -t <session>` directly (replacing the current process).
     2. Or run tmux as a subprocess while temporarily pausing the Bubble Tea loop (using `tea.ExecProcess`). Using `tea.ExecProcess` is highly recommended because it allows the user to return to the TUI when they detach from the tmux session.

## 🧪 Testing Guidelines

1. **Mocking External Commands**: Never execute real commands on the system inside unit tests. Always override the package-level `runner` using the `run.Runner` interface to mock inputs and outputs.
2. **High Test Coverage**: Maintain good test coverage for command parsing, business logic, formatting helpers, and state transitions.
3. **In-Memory UI Verification**: Verify `ViewState` transition flows in unit tests by calling their `.Update(...)` and `.View(...)` methods directly, bypassing the need for a live TUI process loop.
4. **Validation**: Always run `go test ./...` to verify all tests pass after making any changes.

## 📐 Code Style Guidelines

1. **Function Length**: Keep functions short, around 25 lines or less.
2. **Helper Functions**: Break down more complex logic into well-named helper functions to enhance readability.

## 🤖 Agent Workflow Guidelines

1. **Autoformatting**: After making changes to any Go files, always run `go fmt ./...` to ensure correct formatting.
2. **Subagent Review**: Spawn a subagent to review your changes against the spec, user prompt, and AGENTS.md. Repeat the formatting and review process recursively until no further changes are needed.
