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
	line1, line2 := formatSidebarItem(sess, width)
	return styleSidebarItem(line1, line2, isSelected)
}

func formatSidebarItem(sess tmux.Session, width int) (string, string) {
	prefix := "○"
	if sess.Attached {
		prefix = "●"
	}
	line1 := fmt.Sprintf("%s %s (%dw)", prefix, sess.Name, sess.Windows)
	line2 := getStatusLine(sess, width)

	return padToWidth(line1, width-2), padToWidth(line2, width-2)
}

func styleSidebarItem(line1, line2 string, isSelected bool) string {
	var style1, style2 lipgloss.Style
	if isSelected {
		style1 = selectedItemStyle
		style2 = lipgloss.NewStyle().Background(violetCol).Foreground(lipgloss.Color("#E0D8FF"))
	} else {
		style1 = normalItemStyle
		style2 = lipgloss.NewStyle().Foreground(slateCol)
	}
	return style1.Render(line1) + "\n" + style2.Render(line2)
}

func getStatusLine(sess tmux.Session, width int) string {
	statusText := sess.StatusLine
	if statusText == "" {
		statusText = "no status set"
	}
	line2 := "  " + statusText

	maxTextWidth := width - 2
	if maxTextWidth < 0 {
		maxTextWidth = 0
	}
	runes := []rune(line2)
	if len(runes) > maxTextWidth {
		line2 = string(runes[:maxTextWidth])
	}
	return line2
}

func padToWidth(text string, targetWidth int) string {
	padLen := targetWidth - lipgloss.Width(text)
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
