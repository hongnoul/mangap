package style

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	Title      = New().Reverse(true).Bold(true).Padding(0, 1).Render
	ErrorTitle = New().Reverse(true).Bold(true).Padding(0, 1).Render
)

func Tag(foreground, background lipgloss.Color) func(string) string {
	return NewColored(foreground, background).Padding(0, 1).Render
}
