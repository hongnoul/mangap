package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/color"
	"github.com/hongnoul/mangap/icon"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
	"github.com/hongnoul/mangap/util"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/viper"
	"math/rand"
	"strconv"
	"strings"
)

func (b *statefulBubble) View() string {
	if b.del.on {
		on := b.del
		b.del.on = false
		base := b.View()
		b.del = on
		return b.viewArchiveDelete(base)
	}
	switch b.state {
	case scrapersInstallState:
		return b.viewScrapersInstallState()
	case loadingState:
		return b.viewLoading()
	case historyState:
		return b.viewHistory()
	case sourcesState:
		return b.viewSources()
	case searchState:
		return b.viewLive()
	case mangasState:
		if b.arch.on {
			return b.viewArchive()
		}
		return b.viewMangas()
	case chaptersState:
		return b.viewChapters()
	case confirmState:
		return b.viewConfirm()
	case readState:
		return b.viewRead()
	case downloadState:
		return b.viewDownload()
	case downloadDoneState:
		return b.viewDownloadDone()
	case errorState:
		return b.viewError()
	}

	panic("unknown state")
}

func (b *statefulBubble) viewLoading() string {
	// Searching from the hero screen keeps the hero layout (boxes, art)
	// with the loader in place of results.
	if b.statesHistory.Len() > 0 && b.statesHistory.Peek() == searchState {
		return b.viewLive()
	}
	lines := []string{
		style.Title("Loading"),
		"",
		style.Truncate(b.width)(style.Faint(b.progressStatus)),
	}
	// Art fills the space between the status and the help line.
	if artH := b.height - len(lines) - 1; artH >= len(homeArt) {
		lines = append(lines, strings.Split(b.homeBody(b.width, artH), "\n")...)
	}
	return b.renderLines(true, lines)
}

func (b *statefulBubble) viewHistory() string {
	return listExtraPaddingStyle.Render(b.historyC.View())
}

func (b *statefulBubble) viewSources() string {
	return listExtraPaddingStyle.Render(b.sourcesC.View())
}

func (b *statefulBubble) viewSearch() string {
	lines := []string{
		style.Title("Search Manga"),
		"",
		b.inputC.View(),
	}

	return b.renderLines(
		true,
		lines,
	)
}

func (b *statefulBubble) viewMangas() string {
	return b.viewGrid(&b.mangasC, mangaCard, func(th *thumbs, it list.Item, prio int) {
		th.WantManga(it.(*listItem).internal.(*source.Manga), prio)
	})
}

func (b *statefulBubble) viewChapters() string {
	return b.viewGrid(&b.chaptersC, chapterCard, func(th *thumbs, it list.Item, prio int) {
		th.WantChapter(it.(*listItem).internal.(*source.Chapter), prio)
	})
}

// List column width cap; past it the right side becomes a preview pane.
const (
	maxListW    = 64
	minPreviewW = 30
)

// viewGrid draws a thumbnail grid, or the list's own view while filtering
// (the filter input lives there).
func (b *statefulBubble) viewGrid(l *list.Model, toCard func(list.Item) card, want func(*thumbs, list.Item, int)) string {
	return b.viewGridH(l, toCard, want, b.height+2, false)
}

// viewGridH draws the grid in H rows. anchorTop puts it flush at the top
// (under the search bar) instead of centering it.
func (b *statefulBubble) viewGridH(l *list.Model, toCard func(list.Item) card, want func(*thumbs, list.Item, int), H int, anchorTop bool) string {
	if l.FilterState() == list.Filtering {
		return listExtraPaddingStyle.Render(l.View())
	}
	// The grid gets the height minus the indicator line and the bottom
	// margin, and is centered in H so it sits where it would with no
	// pagination UI.
	full := b.width + 4
	W := full
	showPreview := b.thumbs.active() && full >= maxListW+minPreviewW
	if showPreview {
		W = maxListW
	}
	const sideMargin = 2
	gridH := H - 4 // 1 top + indicator + 1 below it + 1 slack
	body, footer := b.grid.render(l, b.thumbs, W-2*sideMargin, gridH, toCard, func(it list.Item, p int) { want(b.thumbs, it, p) })
	if msg := strings.TrimSpace(l.Title); msg != "" && l.ShowTitle() {
		body = msg + "\n" + body
	}

	bodyH := lipgloss.Height(body)
	top := imax((H-bodyH)/2, 0)
	if anchorTop {
		top = 0
	}
	if top+bodyH > H-2 {
		top = imax(H-2-bodyH, 0)
	}
	lines := make([]string, 0, H)
	for i := 0; i < top; i++ {
		lines = append(lines, "")
	}
	pad := strings.Repeat(" ", sideMargin)
	for _, ln := range strings.Split(body, "\n") {
		lines = append(lines, pad+ln)
	}
	for len(lines) < H-2 {
		lines = append(lines, "")
	}
	lines = lines[:H-2]
	lines = append(lines, lipgloss.PlaceHorizontal(W, lipgloss.Center, lipgloss.NewStyle().Faint(true).Render(footer)), "")
	left := strings.Join(lines, "\n")
	if !showPreview {
		return left
	}
	left = lipgloss.NewStyle().Width(W).MaxWidth(W).Render(left)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, b.viewPreviewPane(l, toCard, want, full-W, H))
}

// viewPreviewPane shows the selected item large: the manga cover or the
// chapter's first page, fitted to the pane, with its title underneath.
func (b *statefulBubble) viewPreviewPane(l *list.Model, toCard func(list.Item) card, want func(*thumbs, list.Item, int), w, h int) string {
	it := l.SelectedItem()
	if it == nil {
		return ""
	}
	cd := toCard(it)
	imgW, imgH := imax(w-2, 4), imax(h-4, 2)
	b.preview.SetSpan(imgW, imgH)
	// The pane rasters are large: only the selected one is full-res.
	b.preview.Retain(map[string]bool{cd.thumbKey: true})
	want(b.preview, it, prioVisible)

	lines := make([]string, 0, h)
	lines = append(lines, "")
	for r := 0; r < imgH; r++ {
		lines = append(lines, b.preview.Cells(cd.thumbKey, r))
	}
	lines = append(lines, "", lipgloss.PlaceHorizontal(imgW, lipgloss.Center,
		truncate.StringWithTail(cd.title, uint(imgW), "…")))
	return strings.Join(lines, "\n")
}

// gridList is the list shown as a grid in the current state, if any.
func (b *statefulBubble) gridList() *list.Model {
	switch b.state {
	case mangasState:
		return &b.mangasC
	case chaptersState:
		return &b.chaptersC
	}
	return nil
}

func (b *statefulBubble) viewConfirm() string {
	q := b.confirmQuestion()
	hint := style.Faint("enter confirm · esc cancel")
	maxW := imax(b.width-4, 20)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(1, 3).
		Align(lipgloss.Center).
		Width(imin(imax(lipgloss.Width(q), lipgloss.Width(hint))+8, maxW)).
		Render(q + "\n\n" + hint)
	return b.popup(box)
}

// popup draws box centered over the screen that led to the current state.
func (b *statefulBubble) popup(box string) string {
	var base string
	if b.statesHistory.Len() > 0 {
		if prev := b.statesHistory.Peek(); prev != b.state && prev != 0 {
			cur := b.state
			b.state = prev
			base = b.View()
			b.state = cur
		}
	}
	if base == "" {
		return lipgloss.Place(b.width+4, b.height+2, lipgloss.Center, lipgloss.Center, box)
	}
	return overlayCenter(base, box)
}

// skipCells drops the first n printable cells of s, keeping escape codes
// so the remainder keeps its colors.
func skipCells(s string, n int) string {
	var out strings.Builder
	w, inEsc := 0, false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
			out.WriteRune(r)
		case inEsc:
			out.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
		case w >= n:
			out.WriteRune(r)
		default:
			w += runewidth.RuneWidth(r)
			if w > n {
				out.WriteRune(' ') // half of a wide rune
			}
		}
	}
	return out.String()
}

// overlayCenter draws top centered over base, keeping base visible around it.
func overlayCenter(base, top string) string {
	bl := strings.Split(base, "\n")
	tl := strings.Split(top, "\n")
	bw := 0
	for _, l := range bl {
		bw = imax(bw, lipgloss.Width(l))
	}
	tw := lipgloss.Width(top)
	x := imax((bw-tw)/2, 0)
	y := imax((len(bl)-len(tl))/2, 0)
	for i, l := range tl {
		r := y + i
		for r >= len(bl) {
			bl = append(bl, "")
		}
		line := bl[r]
		if w := lipgloss.Width(line); w < x+tw {
			line += strings.Repeat(" ", x+tw-w)
		}
		bl[r] = truncate.String(line, uint(x)) + "\x1b[0m" + l + "\x1b[0m" + skipCells(line, x+tw)
	}
	return strings.Join(bl, "\n")
}

func (b *statefulBubble) confirmQuestion() string {
	n := util.Quantify(len(b.selectedChapters), "chapter", "chapters")
	if b.confirmAll && b.selectedManga != nil {
		return fmt.Sprintf("Download all of %s (%s)?", style.Fg(color.Purple)(b.selectedManga.Name), n)
	}
	return fmt.Sprintf("Download %s?", n)
}

func (b *statefulBubble) downloadingChapterMetainfo() string {
	metainfo := strings.Builder{}

	// Even though when this function is called chapter isn't supposed to be nil,
	// it can be one for a brief moment.
	// I assume that it's because View() is called before Update()
	if b.currentDownloadingChapter != nil {
		metainfo.WriteString("From ")
		metainfo.WriteString(style.Fg(color.Orange)(b.currentDownloadingChapter.Source().Name()))
		metainfo.WriteString(" as ")
	}

	metainfo.WriteString(style.Fg(color.Purple)(viper.GetString(key.FormatsUse)))
	return metainfo.String()
}

func (b *statefulBubble) viewRead() string {
	return b.viewLive()
}

func (b *statefulBubble) viewReadPlain() string {
	var chapterName string

	chapter := b.currentDownloadingChapter
	if chapter != nil {
		chapterName = chapter.Name
	}

	return b.renderLines(
		true,
		[]string{
			style.Title("Reading"),
			"",
			style.Truncate(b.width)(fmt.Sprintf(icon.Get(icon.Progress)+" Downloading %s", style.Fg(color.Purple)(chapterName))),
			"",
			b.loaderC.view(b.width),
			style.Truncate(b.width)(style.Faint(b.progressStatus)),
			"",
			style.Truncate(b.width)(b.downloadingChapterMetainfo()),
		},
	)
}

func (b *statefulBubble) viewDownload() string {
	return b.viewLive()
}

func (b *statefulBubble) viewDownloadPlain() string {
	var chapterName string

	chapter := b.currentDownloadingChapter
	if chapter != nil {
		chapterName = chapter.Name
	}

	return b.renderLines(
		true,
		[]string{
			style.Title("Downloading"),
			"",
			style.Truncate(b.width)(fmt.Sprintf(icon.Get(icon.Progress)+" Downloading %s", style.Fg(color.Purple)(chapterName))),
			"",
			b.progressC.View(),
			"",
			b.loaderC.view(b.width),
			style.Truncate(b.width)(style.Faint(b.progressStatus)),
			"",
			style.Truncate(b.width)(b.downloadingChapterMetainfo()),
		},
	)
}

func (b *statefulBubble) viewDownloadDone() string {
	failed := len(b.failedChapters)
	succeded := len(b.succededChapters)

	var msg string

	{
		temp := strings.Split(util.Quantify(succeded, "chapter", "chapters"), " ")
		temp[0] = style.Fg(color.Green)(temp[0])
		s := strings.Join(temp, " ") + " downloaded"
		f := fmt.Sprintf("%s failed", style.Fg(color.Red)(strconv.Itoa(failed)))

		msg = fmt.Sprintf("%s, %s", s, f)
	}

	lines := []string{
		style.Title("Finish"),
		"",
		msg,
	}

	if succeded > 0 && viper.GetBool(key.TUIShowDownloadedPath) {
		path, err := b.selectedManga.Path(false)
		if err == nil {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("Downloaded to %s", style.Faint(path)))
		}
	}

	return b.renderLines(
		true,
		lines,
	)
}

func (b *statefulBubble) viewError() string {
	msg := ""
	if b.lastError != nil {
		msg = b.lastError.Error()
	}
	w := imin(imax(lipgloss.Width(msg), 30)+6, imax(b.width-4, 20))
	body := style.ErrorTitle("Error") + "\n\n" +
		wrap.String(style.New().Foreground(color.Red).Render(msg), w-6) + "\n\n" +
		style.Faint("enter / esc close")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color.Red).
		Padding(1, 2).
		Align(lipgloss.Center).
		Width(w).
		Render(body)
	return b.popup(box)
}

func (b *statefulBubble) viewScrapersInstallState() string {
	return listExtraPaddingStyle.Render(b.scrapersInstallC.View())
}

var (
	listExtraPaddingStyle = lipgloss.NewStyle().Padding(1, 2, 1, 0)
	paddingStyle          = lipgloss.NewStyle().Padding(1, 2)
)

func (b *statefulBubble) renderLines(addHelp bool, lines []string) string {
	h := len(lines)
	l := strings.Join(lines, "\n")
	if addHelp {
		l += strings.Repeat("\n", b.height-h) + b.helpC.View(b.keymap)
	}

	return paddingStyle.Render(l)
}

func randomPlot() string {
	plots := []string{
		"The universe is a dangerous place. There are many things that can go wrong. This is one of them:",
		"Fighting an endless army of errors and bugs mangap died a hero. Their last words were:",
		"I used to download stuff without any errors, then I took an arrow to the knee. By arrow I mean this:",
	}

	return plots[rand.Intn(len(plots))]
}
