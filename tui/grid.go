package tui

import (
	"fmt"
	"github.com/hongnoul/mangap/archive"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/source"
	"github.com/muesli/reflow/truncate"
)

// Grid layout for the manga and chapter lists: thumbnail cards in two
// columns by default (each 50% of the width), with as many rows as fit
// the height, 1 to 4. One column (toggle with [ / ]) shows 5 rows. The
// thumbnail grows with the card, so a short terminal gets fewer, bigger
// cards instead of cramped ones.

const (
	defaultGridCols = 1
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
	// fixedCardH, when > 0, pins the card height: a shorter area shows
	// fewer rows instead of squashing them.
	fixedCardH int
}

func newGrid() *grid { return &grid{cols: defaultGridCols, rows: maxGridRows} }

// key handles [ and ] (one / two columns).
func (g *grid) key(k string) bool {
	return false // single column only
}

// fit sets rows for the available height.
func (g *grid) fit(height int) {
	if g.fixedCardH > 0 {
		g.rows = imax(1, imin(singleColRows, height/g.fixedCardH))
		return
	}
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
	if g.fixedCardH > 0 {
		cardH = g.fixedCardH
	}
	// Thumbnail fills the whole card height so it lines up with the row
	// highlight, kept cover-shaped and never wider than ~45% of the card
	// so the title has room.
	thumbH = cardH
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
	selected bool // cursor is on it
	marked   bool // chosen in select mode
	lastRead bool // last viewed chapter, painted green
	dl       bool // chapter already downloaded, painted magenta
	// dlChapters > 0 marks a manga already in the downloads archive:
	// magenta title and a "downloaded · N ch" tag.
	dlChapters int
	dlSize     int64 // bytes on disk of the downloaded chapters
}

// render draws the grid of the list's current page and returns it with
// the page indicator separately, so the caller can center the grid and
// pin the indicator to the bottom. want is called for each card on screen
// (visible=true) and for the next page (prefetch). height is the space
// for the grid alone.
func (g *grid) render(l *list.Model, th *thumbs, width, height int, toCard func(list.Item) card, want func(list.Item, int)) (body, footer string) {
	items := l.VisibleItems()
	if len(items) == 0 {
		return "No results", ""
	}
	g.fit(height)
	cardW, cardH, thumbW, thumbH := g.cardGeometry(width, height)
	if th.active() {
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

	// Full res for this page, the next two and the previous one; low-res
	// for everything else, swept outward from the cursor.
	lo, hi := imax(start-perPage, 0), imin(end+2*perPage, len(items))
	if th.active() {
		keep := make(map[string]bool, hi-lo)
		for i := lo; i < hi; i++ {
			keep[toCard(items[i]).thumbKey] = true
		}
		th.Retain(keep)
	}
	for i := start; i < end; i++ {
		want(items[i], prioVisible)
	}
	for i := end; i < hi; i++ {
		want(items[i], prioNear)
	}
	for i := lo; i < start; i++ {
		want(items[i], prioNear)
	}
	for d := 1; start-d >= 0 || end+d-1 < len(items); d++ {
		if i := end + d - 1; i < len(items) {
			want(items[i], prioSweep)
		}
		if i := start - d; i >= 0 {
			want(items[i], prioSweep)
		}
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
	title := rowTitle(cd.title, false, textW)
	if cd.dlChapters > 0 {
		title = lipgloss.NewStyle().Foreground(dlAccent).Render(title)
	}
	if cd.selected {
		title = lipgloss.NewStyle().Bold(true).Render(title)
	}
	sub := ""
	if cd.dlChapters > 0 && cd.sub == "" {
		sub = " " + lipgloss.NewStyle().Foreground(dlAccent).Faint(true).Render(truncate.StringWithTail(dlLabel(cd), uint(imax(textW-2, 1)), "…"))
	} else if cd.sub != "" {
		sub = " " + lipgloss.NewStyle().Faint(true).Render(truncate.StringWithTail(cd.sub, uint(imax(textW-2, 1)), "…"))
	}

	lefts := make([]string, cardH)
	texts := make([]string, cardH)
	mid := imax(thumbH/2-1, 0)
	for r := 0; r < cardH; r++ {
		if thumbW > 0 {
			if r < thumbH {
				lefts[r] = th.Cells(cd.thumbKey, r)
			} else {
				lefts[r] = strings.Repeat(" ", thumbW)
			}
			lefts[r] += " "
		}
		switch r {
		case mid:
			texts[r] = title
		case mid + 1:
			texts[r] = sub
		}
	}
	// The highlight paints only the text column: the cover stays as is.
	leftW := 0
	if thumbW > 0 {
		leftW = thumbW + 1
	}
	textCol := lipgloss.NewStyle().Width(cardW - leftW).MaxWidth(cardW - leftW).Render(strings.Join(texts, "\n"))
	textCol = highlightRows(textCol, cd.selected, cd.marked, cd.lastRead, cd.dl, cardW-leftW)
	tl := strings.Split(textCol, "\n")
	lines := make([]string, cardH)
	for r := range lines {
		t := ""
		if r < len(tl) {
			t = tl[r]
		}
		lines[r] = lefts[r] + "\x1b[0m" + t
	}
	return strings.Join(lines, "\n")
}

// Row backgrounds: focus is gray, select mode is yellow, both is orange.
const (
	bgFocus           = "\x1b[48;5;238m"
	bgMarked          = "\x1b[48;5;220m\x1b[38;5;16m"
	bgMarkedFocus     = "\x1b[48;5;214m\x1b[38;5;16m\x1b[1m"
	bgLastRead        = "\x1b[48;5;28m\x1b[38;5;16m"
	bgLastReadFocus   = "\x1b[48;5;34m\x1b[38;5;16m\x1b[1m"
	bgDownloaded      = "\x1b[48;5;244m\x1b[38;5;16m"
	bgDownloadedFocus = "\x1b[48;5;250m\x1b[38;5;16m\x1b[1m"
)

var sgrRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// highlightRows paints the whole card (thumbnail row included) with a
// background, re-applying it after any SGR reset inside the content.
func highlightRows(s string, focused, marked, lastRead, dl bool, w int) string {
	bg := ""
	switch {
	case lastRead && !marked && focused:
		bg, marked = bgLastReadFocus, true
	case lastRead && !marked:
		bg, marked = bgLastRead, true
	case dl && !marked && focused:
		bg, marked = bgDownloadedFocus, true
	case dl && !marked:
		bg, marked = bgDownloaded, true
	case focused && marked:
		bg = bgMarkedFocus
	case marked:
		bg = bgMarked
	case focused:
		bg = bgFocus
	default:
		return s
	}
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if pad := w - lipgloss.Width(ln); pad > 0 {
			ln += strings.Repeat(" ", pad)
		}
		// Any SGR can reset the background (\x1b[0m, \x1b[m, or an empty
		// param like \x1b[2;m), so re-apply ours after every one.
		ln = sgrRe.ReplaceAllString(ln, "${0}"+bg)
		// Thumbnail cells set their own fg and end with 39: restore ours.
		if marked {
			ln = strings.ReplaceAll(ln, "\x1b[39m", "\x1b[38;5;16m")
		}
		lines[i] = bg + ln + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}

// mangaCard and chapterCard adapt list items for the grid.
func mangaCard(it list.Item) card {
	m := it.(*listItem).internal.(*source.Manga)
	cd := card{thumbKey: coverURL(m), title: m.Name, marked: it.(*listItem).marked}
	if e, ok := downloadedEntry(m.Name); ok {
		cd.dlChapters = len(e.Chapters)
		if m.Source != nil && m.Source.ID() == "downloads" {
			cd.dlSize = e.Size // size only in the archive viewer
		}
	}
	return cd
}

func chapterCard(it list.Item) card {
	li := it.(*listItem)
	c := li.internal.(*source.Chapter)
	return card{thumbKey: c.URL, title: li.Title(), sub: c.Volume, marked: li.marked, lastRead: li.bookmark != "", dl: !source.IsLocal(c) && c.IsDownloaded()}
}

// nav moves the list cursor through the grid: h/j/k/l go left, down, up,
// right one card (up/down arrows too), ctrl+k or left/right arrows
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
	case "right", "pgdown":
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

// dlLabel is the downloaded subtitle, with disk size in the archive viewer.
func dlLabel(cd card) string {
	s := fmt.Sprintf("downloaded · %d ch", cd.dlChapters)
	if cd.dlSize > 0 {
		s += " · " + archive.HumanSize(cd.dlSize)
	}
	return s
}
