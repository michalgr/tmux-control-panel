package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tmux-control-panel/git"
	"tmux-control-panel/tmux"
	"tmux-control-panel/worktree"
)

var sessionNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func isValidSessionName(name string) bool {
	return sessionNameRegex.MatchString(name)
}

type ViewState interface {
	View(m *Model, width, height int) string
	Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd)
}

// Messages
type tickMsg time.Time
type sessionDetachedMsg struct{ err error }

// Styles
var (
	bgCol      = lipgloss.Color("#2B2A3A") // Dark Slate Grey
	slateCol   = lipgloss.Color("#4E4D63") // Slate
	violetCol  = lipgloss.Color("#7D56F4") // Violet
	magentaCol = lipgloss.Color("#FF06B3") // Magenta
	cyanCol    = lipgloss.Color("#00E5FF") // Cyan
	greenCol   = lipgloss.Color("#00FF66") // Green
	redCol     = lipgloss.Color("#FF5555") // Red

	docStyle = lipgloss.NewStyle().
			Background(bgCol).
			Foreground(lipgloss.Color("#F8F8F2"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(magentaCol).
			MarginLeft(1).
			MarginRight(1)

	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(slateCol).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(slateCol).
			Padding(0, 1)

	activePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(violetCol).
			Padding(0, 1)

	inactivePaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(slateCol).
				Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Background(violetCol).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D9D9D9"))

	metaLabelStyle = lipgloss.NewStyle().
			Foreground(cyanCol).
			Bold(true)

	metaValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	errorStyle = lipgloss.NewStyle().
			Foreground(redCol).
			Bold(true)
)

type Model struct {
	sessions        []tmux.Session
	selectedIndex   int
	width, height   int
	state           ViewState
	initialLoadDone bool

	// Project environment
	cwd string

	// Clients
	tmuxClient *tmux.Client
	gitClient  *git.Client
	wtManager  *worktree.Manager

	// Logger
	logger *log.Logger
}

func NewModel(logger *log.Logger, tmuxClient *tmux.Client, gitClient *git.Client, wtManager *worktree.Manager) (*Model, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	return &Model{
		selectedIndex: 0,
		state:         NormalState{},
		cwd:           cwd,
		tmuxClient:    tmuxClient,
		gitClient:     gitClient,
		wtManager:     wtManager,
		logger:        logger,
	}, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshSessionsCmd(),
		m.tickCmd(),
	)
}

func (m *Model) refreshSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.tmuxClient.ListSessions()
		if err != nil {
			if err == tmux.ErrNoServer {
				return []tmux.Session{}
			}
			return err
		}
		return sessions
	}
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if cmd, handled := m.handleSystemMessage(msg); handled {
		return m, cmd
	}

	nextState, cmd := m.state.Update(m, msg)
	m.state = nextState
	return m, cmd
}

func (m *Model) handleSystemMessage(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return nil, true
	case tickMsg:
		return tea.Batch(m.refreshSessionsCmd(), m.tickCmd()), true
	case []tmux.Session:
		m.sessions = msg
		if !m.initialLoadDone {
			m.initialLoadDone = true
			m.selectCurrentSession()
		}
		m.adjustSelectedIndex()
		return nil, true
	case sessionDetachedMsg:
		m.logger.Println("Detached from tmux session. Refreshing UI...")
		return m.refreshSessionsCmd(), true
	case error:
		m.state = ErrorState{err: msg.Error()}
		return nil, true
	}
	return nil, false
}

func (m *Model) adjustSelectedIndex() {
	if m.selectedIndex >= len(m.sessions) {
		m.selectedIndex = len(m.sessions) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
}

func (m *Model) selectCurrentSession() {
	currentSessName, err := m.tmuxClient.CurrentSessionName()
	if err != nil || currentSessName == "" {
		return
	}
	for i, sess := range m.sessions {
		if sess.Name == currentSessName {
			m.selectedIndex = i
			break
		}
	}
}

func (m *Model) attachOrSwitchSession() tea.Cmd {
	if len(m.sessions) == 0 {
		return nil
	}
	sessName := m.sessions[m.selectedIndex].Name

	if os.Getenv("TMUX") != "" {
		m.logger.Printf("Inside tmux. Switching client to session: %s", sessName)
		if err := m.tmuxClient.SwitchClient(sessName); err != nil {
			return func() tea.Msg { return err }
		}
		return tea.Quit
	}

	m.logger.Printf("Attaching to tmux session: %s", sessName)
	c := exec.Command("tmux", "attach-session", "-t", sessName)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return sessionDetachedMsg{err}
	})
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing Tmux Control Panel..."
	}

	headerView := m.headerView()
	footerView := m.footerView()
	mainHeight := m.height - lipgloss.Height(headerView) - lipgloss.Height(footerView) - 2

	sidebarWidth := m.getSidebarWidth()
	sidebarView := m.sidebarView(sidebarWidth, mainHeight)

	rightWidth := m.width - sidebarWidth - 4
	rightView := m.getRightPaneView(rightWidth, mainHeight)

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, rightView)
	mainBox := activePaneStyle.
		Width(m.width - 2).
		Height(mainHeight).
		Render(mainView)

	return docStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, headerView, mainBox, footerView),
	)
}

func (m *Model) getSidebarWidth() int {
	w := m.width / 3
	if w < 25 {
		return 25
	}
	return w
}

func (m *Model) getRightPaneView(width, height int) string {
	return m.state.View(m, width, height)
}

func (m *Model) headerView() string {
	title := titleStyle.Render("⚡ TMUX CONTROL PANEL (tmux-cp) ⚡")
	desc := lipgloss.NewStyle().Foreground(slateCol).Render("Manage sessions & worktrees")

	header := lipgloss.JoinHorizontal(lipgloss.Center, title, desc)
	return headerStyle.Width(m.width - 2).Render(header)
}

func (m *Model) footerView() string {
	keys := []string{
		"↑/↓,j/k: Navigate",
		"Enter/a: Attach",
		"n: New Session",
		"w: Git Worktree",
		"x/d: Kill Session",
		"r: Refresh",
		"q/esc: Quit",
	}

	footerText := strings.Join(keys, "  •  ")
	return footerStyle.Width(m.width - 2).Render(
		lipgloss.NewStyle().Foreground(slateCol).Render(footerText),
	)
}

// ==========================================
// View States Implementations
// ==========================================

// --- Normal State ---
type NormalState struct{}

func (s NormalState) View(m *Model, width, height int) string {
	return m.detailPaneView(width, height)
}

func (s NormalState) Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKeys(m, msg)
	}
	return s, nil
}

func (s NormalState) handleKeys(m *Model, msg tea.KeyMsg) (ViewState, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return s, tea.Quit
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case "down", "j":
		if m.selectedIndex < len(m.sessions)-1 {
			m.selectedIndex++
		}
	case "enter", "a":
		return s, m.attachOrSwitchSession()
	case "n":
		return s.initNewSession(m)
	case "w":
		return s.initWorktree(m)
	case "x", "d":
		return s.initKillConfirm(m)
	case "r":
		return s, m.refreshSessionsCmd()
	}
	return s, nil
}

func (s NormalState) initNewSession(m *Model) (ViewState, tea.Cmd) {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 30
	ti.SetValue(fmt.Sprintf("session-%d", len(m.sessions)+1))
	return CreateSessionNameState{textInput: ti}, nil
}

func (s NormalState) initWorktree(m *Model) (ViewState, tea.Cmd) {
	isGit, err := m.gitClient.IsGitRepository(m.cwd)
	if err != nil || !isGit {
		return ErrorState{err: "Current directory is not a Git repository. Cannot create Git worktree."}, nil
	}
	branches, err := m.gitClient.GetBranches(m.cwd)
	if err != nil {
		return ErrorState{err: fmt.Sprintf("Failed to list Git branches: %v", err)}, nil
	}

	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 30
	return CreateWorktreeBranchState{gitBranches: branches, textInput: ti}, nil
}

func (s NormalState) initKillConfirm(m *Model) (ViewState, tea.Cmd) {
	if len(m.sessions) > 0 {
		ti := textinput.New()
		ti.Focus()
		ti.CharLimit = 156
		ti.Width = 30
		return ConfirmKillState{textInput: ti}, nil
	}
	return s, nil
}

// --- Error State ---
type ErrorState struct {
	err string
}

func (s ErrorState) View(m *Model, width, height int) string {
	var lines []string
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(redCol).
		Underline(true).
		Render("ERROR ENCOUNTERED")
	lines = append(lines, header, "", errorStyle.Render(s.err), "")

	lines = append(lines,
		"",
		lipgloss.NewStyle().Foreground(slateCol).Render("[Esc] / [Enter] Dismiss"),
	)
	return fillVertical(lines, width, height)
}

func (s ErrorState) Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" || msg.String() == "enter" {
			return NormalState{}, m.refreshSessionsCmd()
		}
	}
	return s, nil
}

// --- Confirm Kill State ---
type ConfirmKillState struct {
	textInput textinput.Model
}

func (s ConfirmKillState) View(m *Model, width, height int) string {
	f := Form{
		Title:     "KILL SESSION CONFIRMATION",
		Label:     fmt.Sprintf("Type 'y' to confirm killing session '%s':", m.sessions[m.selectedIndex].Name),
		InputView: s.textInput.View(),
		Hints: []string{
			lipgloss.NewStyle().Foreground(redCol).Bold(true).Render("WARNING: This will immediately close all windows and programs"),
			lipgloss.NewStyle().Foreground(redCol).Bold(true).Render("running in this tmux session!"),
			"",
			lipgloss.NewStyle().Foreground(slateCol).Italic(true).Render("Type 'y' / 'yes' to confirm, any other key to cancel."),
		},
	}
	return f.Render(width, height)
}

func (s ConfirmKillState) Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return NormalState{}, m.refreshSessionsCmd()
		case "enter":
			return s.handleEnter(m)
		default:
			var cmd tea.Cmd
			s.textInput, cmd = s.textInput.Update(msg)
			return s, cmd
		}
	}
	return s, nil
}

func (s ConfirmKillState) handleEnter(m *Model) (ViewState, tea.Cmd) {
	val := strings.TrimSpace(s.textInput.Value())
	if strings.ToLower(val) == "y" || strings.ToLower(val) == "yes" {
		target := m.sessions[m.selectedIndex].Name
		m.logger.Printf("Killing session: %s", target)
		err := m.tmuxClient.KillSession(target)
		if err != nil {
			return ErrorState{err: err.Error()}, nil
		}
	}
	return NormalState{}, m.refreshSessionsCmd()
}

// --- Create Session Name State ---
type CreateSessionNameState struct {
	textInput textinput.Model
}

func (s CreateSessionNameState) View(m *Model, width, height int) string {
	f := Form{
		Title:     "CREATE STANDARD SESSION",
		Label:     "Enter session name:",
		InputView: s.textInput.View(),
	}
	return f.Render(width, height)
}

func (s CreateSessionNameState) Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return NormalState{}, m.refreshSessionsCmd()
		case "enter":
			return s.handleEnter(m)
		default:
			var cmd tea.Cmd
			s.textInput, cmd = s.textInput.Update(msg)
			return s, cmd
		}
	}
	return s, nil
}

func (s CreateSessionNameState) handleEnter(m *Model) (ViewState, tea.Cmd) {
	val := strings.TrimSpace(s.textInput.Value())
	if val == "" {
		return s, nil
	}
	if !isValidSessionName(val) {
		return ErrorState{err: "Invalid session name. Only alphanumeric, hyphens, and underscores are allowed."}, nil
	}
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 30
	ti.SetValue(m.cwd)
	return CreateSessionPathState{name: val, textInput: ti}, nil
}

// --- Create Session Path State ---
type CreateSessionPathState struct {
	name      string
	textInput textinput.Model
}

func (s CreateSessionPathState) View(m *Model, width, height int) string {
	f := Form{
		Title:     "CREATE STANDARD SESSION",
		Label:     "Enter base directory path:",
		InputView: s.textInput.View(),
		Completed: []string{
			fmt.Sprintf("✓ Session Name: %s", lipgloss.NewStyle().Foreground(greenCol).Render(s.name)),
		},
		Hints: []string{
			lipgloss.NewStyle().Foreground(slateCol).Italic(true).Render("Tip: Default is the current directory."),
		},
	}
	return f.Render(width, height)
}

func (s CreateSessionPathState) Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return NormalState{}, m.refreshSessionsCmd()
		case "enter":
			return s.handleEnter(m)
		default:
			var cmd tea.Cmd
			s.textInput, cmd = s.textInput.Update(msg)
			return s, cmd
		}
	}
	return s, nil
}

func (s CreateSessionPathState) handleEnter(m *Model) (ViewState, tea.Cmd) {
	val := strings.TrimSpace(s.textInput.Value())
	if val == "" {
		val = m.cwd
	}
	m.logger.Printf("Creating standard session: %s in %s", s.name, val)
	err := m.tmuxClient.CreateSession(s.name, val)
	if err != nil {
		return ErrorState{err: err.Error()}, nil
	}
	return NormalState{}, m.refreshSessionsCmd()
}

// --- Create Worktree Branch State ---
type CreateWorktreeBranchState struct {
	gitBranches []string
	textInput   textinput.Model
}

func (s CreateWorktreeBranchState) View(m *Model, width, height int) string {
	var hints []string
	if len(s.gitBranches) > 0 {
		hints = []string{
			"",
			lipgloss.NewStyle().Foreground(cyanCol).Bold(true).Render("Available Git Branches:"),
			lipgloss.NewStyle().Foreground(slateCol).Render(strings.Join(s.gitBranches, ", ")),
		}
	}
	f := Form{
		Title:     "CREATE GIT WORKTREE SESSION",
		Label:     "Enter Git branch name (to checkout/create):",
		InputView: s.textInput.View(),
		Hints:     hints,
	}
	return f.Render(width, height)
}

func (s CreateWorktreeBranchState) Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return NormalState{}, m.refreshSessionsCmd()
		case "enter":
			return s.handleEnter(m)
		default:
			var cmd tea.Cmd
			s.textInput, cmd = s.textInput.Update(msg)
			return s, cmd
		}
	}
	return s, nil
}

func (s CreateWorktreeBranchState) handleEnter(m *Model) (ViewState, tea.Cmd) {
	val := strings.TrimSpace(s.textInput.Value())
	if val == "" {
		return s, nil
	}
	exists := false
	for _, b := range s.gitBranches {
		if b == val {
			exists = true
			break
		}
	}
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 30
	if exists {
		ti.SetValue(val)
		return CreateWorktreeSessionNameState{branch: val, createBranch: false, textInput: ti}, nil
	}
	ti.SetValue("y")
	return CreateWorktreeCreateBranchState{branch: val, textInput: ti}, nil
}

// --- Create Worktree Create Branch State ---
type CreateWorktreeCreateBranchState struct {
	branch    string
	textInput textinput.Model
}

func (s CreateWorktreeCreateBranchState) View(m *Model, width, height int) string {
	f := Form{
		Title:     "CREATE GIT WORKTREE SESSION",
		Label:     fmt.Sprintf("Branch '%s' doesn't exist. Create new branch? (y/n):", s.branch),
		InputView: s.textInput.View(),
		Hints: []string{
			lipgloss.NewStyle().Foreground(slateCol).Italic(true).Render("Type 'y' / 'yes' to create it, or 'n' / 'no' to go back."),
		},
	}
	return f.Render(width, height)
}

func (s CreateWorktreeCreateBranchState) Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return NormalState{}, m.refreshSessionsCmd()
		case "enter":
			return s.handleEnter(m)
		default:
			var cmd tea.Cmd
			s.textInput, cmd = s.textInput.Update(msg)
			return s, cmd
		}
	}
	return s, nil
}

func (s CreateWorktreeCreateBranchState) handleEnter(m *Model) (ViewState, tea.Cmd) {
	val := strings.TrimSpace(s.textInput.Value())
	ans := strings.ToLower(val)
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 30
	if ans == "y" || ans == "yes" {
		ti.SetValue(s.branch)
		return CreateWorktreeSessionNameState{branch: s.branch, createBranch: true, textInput: ti}, nil
	}
	branches, err := m.gitClient.GetBranches(m.cwd)
	if err != nil {
		return ErrorState{err: fmt.Sprintf("Failed to list Git branches: %v", err)}, nil
	}
	return CreateWorktreeBranchState{gitBranches: branches, textInput: ti}, nil
}

// --- Create Worktree Session Name State ---
type CreateWorktreeSessionNameState struct {
	branch       string
	createBranch bool
	textInput    textinput.Model
}

func (s CreateWorktreeSessionNameState) View(m *Model, width, height int) string {
	createBranchStr := "No (existing branch)"
	if s.createBranch {
		createBranchStr = "Yes (new branch)"
	}
	f := Form{
		Title:     "CREATE GIT WORKTREE SESSION",
		Label:     "Enter session name:",
		InputView: s.textInput.View(),
		Completed: []string{
			fmt.Sprintf("✓ Git Branch:   %s", lipgloss.NewStyle().Foreground(greenCol).Render(s.branch)),
			fmt.Sprintf("✓ Create Branch: %s", lipgloss.NewStyle().Foreground(greenCol).Render(createBranchStr)),
		},
	}
	return f.Render(width, height)
}

func (s CreateWorktreeSessionNameState) Update(m *Model, msg tea.Msg) (ViewState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return NormalState{}, m.refreshSessionsCmd()
		case "enter":
			return s.handleEnter(m)
		default:
			var cmd tea.Cmd
			s.textInput, cmd = s.textInput.Update(msg)
			return s, cmd
		}
	}
	return s, nil
}

func (s CreateWorktreeSessionNameState) handleEnter(m *Model) (ViewState, tea.Cmd) {
	val := strings.TrimSpace(s.textInput.Value())
	if val == "" {
		return s, nil
	}
	if !isValidSessionName(val) {
		return ErrorState{err: "Invalid session name. Only alphanumeric, hyphens, and underscores are allowed."}, nil
	}
	worktreePath := resolveWorktreePath(val)
	m.logger.Printf("Creating git worktree session via WorktreeManager: session=%s, branch=%s, path=%s", val, s.branch, worktreePath)

	err := m.wtManager.CreateWorktreeSession(m.cwd, s.branch, val, worktreePath, s.createBranch)
	if err != nil {
		return ErrorState{err: err.Error()}, nil
	}

	return NormalState{}, m.refreshSessionsCmd()
}

func resolveWorktreePath(val string) string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "tmux-cp", "worktrees", val)
}
