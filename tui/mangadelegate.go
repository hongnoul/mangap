package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hongnoul/mangaz/source"
	"github.com/hongnoul/mangaz/style"
	"github.com/muesli/reflow/truncate"
)

// mangaDelegate renders a manga row as a cover thumbnail with the title
// beside it. Without kitty graphics it is a plain title row.
type mangaDelegate struct {
	th *thumbs
}

func (d mangaDelegate) Height() int {
	if d.th.enabled {
		return thumbRows
	}
	return 1
}
func (d mangaDelegate) Spacing() int                        { return 1 }
func (d mangaDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d mangaDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	li, ok := item.(*listItem)
	if !ok {
		return
	}
	manga, _ := li.internal.(*source.Manga)
	if manga == nil {
		return
	}
	selected := index == m.Index()

	bar := "  "
	title := manga.Name
	if selected {
		bar = "┃ "
		title = style.Bold(title)
	}
	textW := m.Width() - 2 - thumbCols - 2
	if textW < 4 {
		textW = 4
	}
	title = truncate.StringWithTail(title, uint(textW), "…")

	if !d.th.enabled {
		fmt.Fprint(w, bar+title)
		return
	}

	lines := make([]string, thumbRows)
	for r := 0; r < thumbRows; r++ {
		text := ""
		if r == thumbRows/2-1 {
			text = title
		}
		lines[r] = bar + d.th.Cells(manga, r) + "  " + text
	}
	fmt.Fprint(w, strings.Join(lines, "\n"))
}
