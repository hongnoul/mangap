package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangaz/color"
	"github.com/hongnoul/mangaz/style"
)

// homeKeys lists every TUI keybind, grouped, for the home screen.
var homeKeys = [][2]string{
	{"enter", "search / open"},
	{"tab", "complete suggestion"},
	{"ctrl+j / k", "pick suggestion"},
	{"ctrl+s", "source settings"},
	{"j / k", "down / up"},
	{"l / h", "open / back"},
	{"hjkl", "move in grid"},
	{"← / →", "prev / next page"},
	{"[ / ]", "1 / 2 columns"},
	{"g / G", "top / bottom"},
	{"/", "filter list"},
	{"esc", "back"},
	{"r", "read chapter"},
	{"space", "select chapter"},
	{"tab / *", "select all"},
	{"v", "select volume"},
	{"backspace", "clear selection"},
	{"a", "anilist match"},
	{"o", "open in browser"},
	{"?", "more help"},
	{"ctrl+c", "quit"},
}

// viewHome is the search screen: art, a boxed search bar, and the keys,
// centered in the terminal. Pieces drop out on short terminals.
func (b *statefulBubble) viewHome() string {
	w, h := b.width, b.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	boxW := imin(60, w-4)
	b.inputC.Width = imax(boxW-6-lipgloss.Width(b.inputC.Prompt), 10)
	box := b.searchBox(boxW)

	keys := keysGrid(w)
	art := homeArtColored()

	// Fit what we can in the full terminal height: box always, then keys,
	// then art.
	full := h + 2
	center := []string{box}
	used := lipgloss.Height(lipgloss.JoinVertical(lipgloss.Center, center...))
	showKeys := len(b.searchSuggestions) == 0 && used+1+lipgloss.Height(keys) <= full
	if showKeys {
		used += 1 + lipgloss.Height(keys)
	}
	showArt := used+len(homeArt)+1 <= full && w >= lipgloss.Width(homeArt[0])

	var parts []string
	if showArt {
		parts = append(parts, art, "")
	}
	parts = append(parts, center...)
	if showKeys {
		parts = append(parts, "", keys)
	}

	block := lipgloss.JoinVertical(lipgloss.Center, parts...)
	return lipgloss.Place(w+4, h+2, lipgloss.Center, lipgloss.Center, block)
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
		rows = append(rows, style.Faint("↑/↓ select · tab complete · enter search"))
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
	if cols == 1 && len(homeKeys) > 10 {
		cols = 2 // two tight columns beat a list taller than the screen
	}
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
