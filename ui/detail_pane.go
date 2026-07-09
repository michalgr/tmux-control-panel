package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"tmux-control-panel/tmux"
)

func (m *Model) detailPaneView(width, height int) string {
	var lines []string

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(magentaCol).
		Underline(true).
		Render("SESSION METADATA")
	lines = append(lines, header, "")

	if len(m.sessions) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(slateCol).Render("Select or create a session to view details."))
		return fillVertical(lines, width, height)
	}

	sess := m.sessions[m.selectedIndex]
	lines = appendBasicMetadata(lines, sess)
	lines = m.appendGitStatus(lines, sess.Path)
	lines = m.appendWorktreeDetails(lines, sess)

	return fillVertical(lines, width, height)
}

func appendBasicMetadata(lines []string, sess tmux.Session) []string {
	statusStr := renderStatus(sess.Attached)
	actWin := fallbackVal(sess.ActiveWindowName, "unknown")
	agentStatus := fallbackVal(sess.StatusLine, "none")
	uptime := formatUptime(sess.Created)

	return append(lines,
		renderMetaLine("Session Name:", sess.Name),
		renderMetaLine("Status:      ", statusStr),
		renderMetaLine("Windows:     ", fmt.Sprintf("%d", sess.Windows)),
		renderMetaLine("Active Win:  ", actWin),
		renderMetaLine("Agent Status:", agentStatus),
		renderMetaLine("Directory:   ", sess.Path),
		renderMetaLine("Created At:  ", sess.Created.Format("2006-01-02 15:04:05")),
		renderMetaLine("Uptime:      ", uptime),
	)
}

func renderStatus(attached bool) string {
	if attached {
		return lipgloss.NewStyle().Foreground(cyanCol).Bold(true).Render("Attached")
	}
	return lipgloss.NewStyle().Foreground(slateCol).Render("Detached")
}

func fallbackVal(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}

func renderMetaLine(label, val string) string {
	return fmt.Sprintf("%s %s", metaLabelStyle.Render(label), metaValueStyle.Render(val))
}

func (m *Model) appendGitStatus(lines []string, path string) []string {
	isGit, err := m.gitClient.InsideWorkTree(path)
	if err == nil && isGit {
		gitStatus, err := m.gitClient.GetStatus(path)
		if err == nil {
			lines = append(lines, fmt.Sprintf("%s %s", metaLabelStyle.Render("Git Status:  "), metaValueStyle.Render(gitStatus.String())))
		}
	}
	return lines
}

func (m *Model) appendWorktreeDetails(lines []string, sess tmux.Session) []string {
	if sess.WorktreePath == "" {
		return lines
	}

	lines = append(lines,
		"",
		lipgloss.NewStyle().Bold(true).Foreground(greenCol).Render("🌳 GIT WORKTREE ASSOCIATION"),
		fmt.Sprintf("%s %s", metaLabelStyle.Render("WT Path:     "), metaValueStyle.Render(sess.WorktreePath)),
	)

	wtStatus, err := m.gitClient.GetStatus(sess.WorktreePath)
	if err == nil {
		lines = append(lines, fmt.Sprintf("%s %s", metaLabelStyle.Render("WT Status:   "), metaValueStyle.Render(wtStatus.String())))
	}

	return append(lines,
		"",
		lipgloss.NewStyle().Foreground(slateCol).Italic(true).Render("Note: This worktree will be destroyed automatically"),
		lipgloss.NewStyle().Foreground(slateCol).Italic(true).Render("when the tmux session is closed."),
	)
}

func formatUptime(created time.Time) string {
	d := time.Since(created)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
