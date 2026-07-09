package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"tmux-control-panel/tmux"
)

func (m Model) sidebarView(width, height int) string {
	var lines []string

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(cyanCol).
		Underline(true).
		Render("SESSIONS")
	lines = append(lines, header, "")

	if len(m.sessions) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(slateCol).Render("  No active sessions"), "  Press 'n' to create")
		return fillVertical(lines, width, height)
	}

	for i, sess := range m.sessions {
		renderedItem := renderSidebarItem(sess, i == m.selectedIndex, width)
		lines = append(lines, renderedItem)
	}

	return fillVertical(lines, width, height)
}

func renderSidebarItem(sess tmux.Session, isSelected bool, width int) string {
	prefix := formatStatusPrefix(sess.Attached) + " "
	wtTag := formatWorktreeTag(sess.WorktreePath != "")
	itemText := fmt.Sprintf("%s%s (%dw)%s", prefix, sess.Name, sess.Windows, wtTag)

	if !isSelected {
		return normalItemStyle.Render(itemText)
	}
	return selectedItemStyle.Render(padItemText(itemText, width))
}

func formatStatusPrefix(attached bool) string {
	if attached {
		return lipgloss.NewStyle().Foreground(cyanCol).Bold(true).Render("●")
	}
	return lipgloss.NewStyle().Foreground(slateCol).Render("○")
}

func formatWorktreeTag(hasWorktree bool) string {
	if hasWorktree {
		return lipgloss.NewStyle().Foreground(greenCol).Render(" [wt]")
	}
	return ""
}

func padItemText(text string, width int) string {
	padLen := width - lipgloss.Width(text) - 2
	if padLen < 0 {
		padLen = 0
	}
	return text + strings.Repeat(" ", padLen)
}

// Helper to pad lines vertically to keep the layout snappy and consistent
func fillVertical(lines []string, width, height int) string {
	res := strings.Join(lines, "\n")
	h := lipgloss.Height(res)
	if h < height {
		res += strings.Repeat("\n", height-h)
	}
	return lipgloss.NewStyle().Width(width).MaxHeight(height).Render(res)
}
