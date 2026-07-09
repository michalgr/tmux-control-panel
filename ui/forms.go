package ui

import (
	"github.com/charmbracelet/lipgloss"
)

type Form struct {
	Title     string
	Label     string
	InputView string
	Completed []string
	Hints     []string
}

func (f Form) Render(width, height int) string {
	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(magentaCol).Underline(true).Render(f.Title),
		"",
	}

	lines = append(lines, f.Completed...)
	if len(f.Completed) > 0 {
		lines = append(lines, "")
	}

	lines = append(lines, lipgloss.NewStyle().Foreground(cyanCol).Bold(true).Render(f.Label), "", f.InputView, "")
	lines = append(lines, f.Hints...)
	lines = append(lines, "", lipgloss.NewStyle().Foreground(slateCol).Render("[Enter] Confirm  •  [Esc] Cancel"))

	return fillVertical(lines, width, height)
}
