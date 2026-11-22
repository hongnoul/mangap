package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangaz/source"
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
	bar := ""
	title := rowTitle(manga.Name, index == m.Index(), m.Width()-thumbCols-2)

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
		lines[r] = bar + d.th.Cells(coverURL(manga), r) + "  " + text
	}
	fmt.Fprint(w, strings.Join(lines, "\n"))
}

// chapterDelegate renders a chapter row as its first page thumbnail with
// the chapter title (marks, volume, downloaded icon) beside it. Visible
// rows request their thumbnail as they are drawn, so scrolling a long
// list only fetches what is on screen.
type chapterDelegate struct {
	th *thumbs
}

func (d chapterDelegate) Height() int {
	if d.th.enabled {
		return thumbRows
	}
	return 1
}
func (d chapterDelegate) Spacing() int                        { return 1 }
func (d chapterDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d chapterDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	li, ok := item.(*listItem)
	if !ok {
		return
	}
	ch, _ := li.internal.(*source.Chapter)
	if ch == nil {
		return
	}
	bar := ""
	title := rowTitle(li.Title(), index == m.Index(), m.Width()-thumbCols-2)

	if !d.th.enabled {
		fmt.Fprint(w, bar+title)
		return
	}

	d.th.WantChapter(ch)
	lines := make([]string, thumbRows)
	for r := 0; r < thumbRows; r++ {
		text := ""
		if r == thumbRows/2-1 {
			text = title
		}
		lines[r] = bar + d.th.Cells(ch.URL, r) + "  " + text
	}
	fmt.Fprint(w, strings.Join(lines, "\n"))
}
