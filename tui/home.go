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
	{"ctrl+s", "pick source"},
	{"ctrl+l", "pick language"},
	{"ctrl+o", "next source"},
	{"ctrl+d", "downloads"},
	{"ctrl+g", "toggle keybinds"},
	{"ctrl+r", "terminal / external viewer"},
	{"ctrl+t", "toggle thumbnails"},
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

// searchBox renders the search bar as a single bordered input row.
func (b *statefulBubble) searchBox(boxW int) string {
	rows := []string{b.inputC.View()}

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

// homeBody is what shows under the search bar before anything is typed.
// The art is pinned to the center of w x h so it never moves.
func (b *statefulBubble) homeBody(w, h int) string {
	h = imax(h, 1)
	art := ""
	if len(homeArt) <= h && w >= lipgloss.Width(homeArt[0]) {
		art = strings.Join(homeArt, "\n")
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, art)
}

// viewKeysBox is the ctrl+g cheatsheet, boxed like the dropdowns.
func (b *statefulBubble) viewKeysBox(w int) string {
	lines := keysGrid(imax(w-4, 10)) + "\n" + style.Faint("ctrl+g / esc close")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(0, 1).
		Width(imax(w-2, 4)).
		Align(lipgloss.Center).
		Render(lines)
}
