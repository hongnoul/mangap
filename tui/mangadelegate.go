package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

// rowTitle truncates a row title to textW and, when focused, renders it
// as a reverse-video highlight (the ascii color profile drops backgrounds).
func rowTitle(title string, selected bool, textW int) string {
	if textW < 6 {
		textW = 6
	}
	title = truncate.StringWithTail(title, uint(textW-2), "…")
	if selected {
		return lipgloss.NewStyle().Reverse(true).Bold(true).Padding(0, 1).Render(title)
	}
	return " " + title + " "
}
