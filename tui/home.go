package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/color"
	"github.com/hongnoul/mangap/style"
)

// homeKeys lists every TUI keybind, grouped, for the home screen.
var homeKeys = [][2]string{
	{"enter", "search / open"},
	{"tab / shift+tab", "next / prev result"},
	{"↑ / ↓", "pick suggestion"},
	{"ctrl+j / k", "next / prev result"},
	{"ctrl+s", "source settings"},
	{"ctrl+o", "switch source"},
	{"j / k", "down / up"},
	{"l / h", "open / back"},
	{"hjkl", "move in grid"},
	{"← / →", "prev / next page"},
	{"esc", "back"},
	{"g / G", "top / bottom"},
	{"r", "read chapter"},
	{"v", "select mode"},
	{"d", "download"},
	{"?", "more help"},
	{"ctrl+c", "quit"},
}

// searchBox renders the search bar. With suggestions it expands downward
// like a browser search box: the input on top, a divider, then one row per
// suggestion, all inside the same border.
func (b *statefulBubble) searchBox(boxW int) string {
	inner := boxW - 2 // Width includes padding, not border
	rows := []string{b.inputC.View()}

	if len(b.searchSuggestions) > 0 {
		rows = append(rows, style.Fg(color.Purple)(strings.Repeat("─", inner)))
		for i, s := range b.searchSuggestions {
			if max := inner - 4; lipgloss.Width(s) > max {
				r := []rune(s)
				if len(r) > max-1 {
					r = r[:imax(max-1, 1)]
				}
				s = string(r) + "…"
			}
			var line string
			if i == b.suggestionIdx {
				line = lipgloss.NewStyle().
					Reverse(true).
					Bold(true).
					Width(inner).
					Render(s)
			} else {
				line = style.Faint(s)
			}
			rows = append(rows, line)
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color.Purple).
		Padding(0, 1).
		Width(boxW).
		Render(strings.Join(rows, "\n"))
}

// keysGrid lays homeKeys out in as many columns as fit (max 3).
func keysGrid(width int) string {
	keyW, descW := 0, 0
	for _, kv := range homeKeys {
		keyW = imax(keyW, lipgloss.Width(kv[0]))
		descW = imax(descW, len(kv[1]))
	}
	cellW := keyW + 2 + descW + 4
	cols := imax(1, imin(3, width/cellW))
	rows := (len(homeKeys) + cols - 1) / cols

	var columns []string
	for c := 0; c < cols; c++ {
		var lines []string
		for r := 0; r < rows; r++ {
			i := c*rows + r
			if i >= len(homeKeys) {
				break
			}
			k := homeKeys[i]
			pad := strings.Repeat(" ", keyW-lipgloss.Width(k[0]))
			lines = append(lines, fmt.Sprintf("%s%s  %s",
				pad, style.Bold(k[0]), style.Faint(fmt.Sprintf("%-*s", descW, k[1]))))
		}
		columns = append(columns, strings.Join(lines, "\n"))
		if c < cols-1 {
			columns = append(columns, "    ")
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
}

func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// homeBody is what shows under the search bar before anything is typed:
// art and keys, centered in w x h, dropping pieces that do not fit.
func (b *statefulBubble) homeBody(w, h int) string {
	keys := keysGrid(w - 4)
	var parts []string
	used := 0
	if lipgloss.Height(keys) <= h && lipgloss.Width(keys) <= w && len(b.searchSuggestions) == 0 {
		used = lipgloss.Height(keys)
	} else {
		keys = ""
	}
	if used+len(homeArt)+1 <= h && w >= lipgloss.Width(homeArt[0]) {
		parts = append(parts, strings.Join(homeArt, "\n"))
		if keys != "" {
			parts = append(parts, "")
		}
	}
	if keys != "" {
		parts = append(parts, keys)
	}
	if len(parts) == 0 {
		return ""
	}
	return lipgloss.Place(w, imax(h, 1), lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, parts...))
}
