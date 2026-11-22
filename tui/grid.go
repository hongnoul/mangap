package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangaz/source"
	"github.com/muesli/reflow/truncate"
)

// Grid layout for the manga and chapter lists: thumbnail cards in two
// columns by default (each 50% of the width), with as many rows as fit
// the height, 1 to 4. One column (toggle with [ / ]) shows 5 rows. The
// thumbnail grows with the card, so a short terminal gets fewer, bigger
// cards instead of cramped ones.

const (
	defaultGridCols = 2
	singleColRows   = 5
	maxGridRows     = 4
	minGridRows     = 1
	// A card shorter than this looks cramped, so two-column mode drops a
	// row instead.
	minCardH = 6
	// Covers are about 2:3 (w:h) and terminal cells about 1:2, so a
	// cover-shaped box is roughly 4/3 cells wide per cell of height.
	coverWPerH = 4.0 / 3.0
)

type grid struct {
	cols, rows int
}

func newGrid() *grid { return &grid{cols: defaultGridCols, rows: maxGridRows} }

// key handles [ and ] (one / two columns).
func (g *grid) key(k string) bool {
	switch k {
	case "[":
		g.cols = 1
	case "]":
		g.cols = 2
	default:
		return false
	}
	return true
}

// fit sets rows for the available height.
func (g *grid) fit(height int) {
	if g.cols == 1 {
		g.rows = singleColRows
		return
	}
	g.rows = imax(minGridRows, imin(maxGridRows, height/minCardH))
}

// perPage is how many cards one screen holds.
func (g *grid) perPage() int { return g.cols * g.rows }

// cardGeometry is the size of one card and of its thumbnail, in cells.
func (g *grid) cardGeometry(width, height int) (cardW, cardH, thumbW, thumbH int) {
	cardW = imax(width/g.cols, 10)
	cardH = imax(height/g.rows, 2)
	// Thumbnail fills the card height minus a 1-row gap, kept cover-shaped
	// and never wider than ~45% of the card so the title has room.
	thumbH = imax(cardH-1, 1)
	thumbW = int(float64(thumbH)*coverWPerH + 0.5)
	if lim := cardW * 45 / 100; thumbW > lim {
		thumbW = imax(lim, 2)
	}
	return
}

// card is one rendered cell of the grid.
type card struct {
	thumbKey string
	title    string
	sub      string
	selected bool
}

// render draws the grid of the list's current page and returns it with
// the page indicator separately, so the caller can center the grid and
// pin the indicator to the bottom. want is called for each card on screen
// (visible=true) and for the next page (prefetch). height is the space
// for the grid alone.
func (g *grid) render(l *list.Model, th *thumbs, width, height int, toCard func(list.Item) card, want func(list.Item, bool)) (body, footer string) {
	items := l.VisibleItems()
	if len(items) == 0 {
		return "No results", ""
	}
	g.fit(height)
	cardW, cardH, thumbW, thumbH := g.cardGeometry(width, height)
	if th.enabled {
		th.SetSpan(thumbW, thumbH)
	} else {
		// No images: one text line per card, rows fill the height.
		thumbW, thumbH, cardH = 0, 1, 1
		g.rows = imax(height, 1)
	}

	perPage := g.cols * g.rows
	cur := l.Index()
	page := cur / perPage
	start := page * perPage
	end := imin(start+perPage, len(items))

	for i := start; i < end; i++ {
		want(items[i], true)
	}
	for i := end; i < imin(end+perPage, len(items)); i++ {
		want(items[i], false)
	}

	var rowsOut []string
	for r := 0; r < g.rows; r++ {
		var cells []string
		for c := 0; c < g.cols; c++ {
			// Row-major: left to right, then down.
			i := start + r*g.cols + c
			if i >= end {
				cells = append(cells, lipgloss.NewStyle().Width(cardW).Height(cardH).Render(""))
				continue
			}
			cd := toCard(items[i])
			cd.selected = i == cur
			cells = append(cells, renderCard(cd, th, cardW, cardH, thumbW, thumbH))
		}
		rowsOut = append(rowsOut, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}

	pages := (len(items) + perPage - 1) / perPage
	return strings.Join(rowsOut, "\n"), fmt.Sprintf("%d/%d", page+1, pages)
}

func renderCard(cd card, th *thumbs, cardW, cardH, thumbW, thumbH int) string {
	textW := cardW - thumbW - 3
	title := rowTitle(cd.title, cd.selected, textW)
	sub := ""
	if cd.sub != "" {
		sub = " " + lipgloss.NewStyle().Faint(true).Render(truncate.StringWithTail(cd.sub, uint(imax(textW-2, 1)), "…"))
	}

	lines := make([]string, cardH)
	mid := imax(thumbH/2-1, 0)
	for r := 0; r < cardH; r++ {
		left := ""
		if thumbW > 0 {
			if r < thumbH {
				left = th.Cells(cd.thumbKey, r)
			} else {
				left = strings.Repeat(" ", thumbW)
			}
			left += " "
		}
		text := ""
		switch r {
		case mid:
			text = title
		case mid + 1:
			text = sub
		}
		lines[r] = left + text
	}
	return lipgloss.NewStyle().Width(cardW).MaxWidth(cardW).Render(strings.Join(lines, "\n"))
}

// mangaCard and chapterCard adapt list items for the grid.
func mangaCard(it list.Item) card {
	m := it.(*listItem).internal.(*source.Manga)
	return card{thumbKey: coverURL(m), title: m.Name}
}

func chapterCard(it list.Item) card {
	li := it.(*listItem)
	c := li.internal.(*source.Chapter)
	return card{thumbKey: c.URL, title: li.Title(), sub: c.Volume}
}

// nav moves the list cursor through the grid: h/j/k/l go left, down, up,
// right one card (up/down arrows too), ctrl+j/ctrl+k or left/right arrows
// flip a page, g/G jump to the ends. esc/enter (back/open) fall through.
// Returns true when the key was handled.
func (g *grid) nav(l *list.Model, k string) bool {
	n := len(l.VisibleItems())
	if n == 0 {
		return false
	}
	cur := l.Index()
	to := cur
	switch k {
	case "j", "down":
		to = cur + g.cols
		if to >= n {
			to = n - 1
		}
	case "k", "up":
		to = cur - g.cols
		if to < 0 {
			to = 0
		}
	case "tab":
		return false
	case "right", "ctrl+j", "pgdown":
		to = imin(cur+g.perPage(), n-1)
	case "left", "ctrl+k", "pgup":
		to = imax(cur-g.perPage(), 0)
	case "h":
		// Left one card; from the left column wrap to the end of the
		// previous row so h walks the whole list in reading order.
		to = imax(cur-1, 0)
	case "l":
		to = imin(cur+1, n-1)
	case "g", "home":
		to = 0
	case "G", "end":
		to = n - 1
	default:
		return false
	}
	l.Select(to)
	return true
}
