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
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/viper"
	"math/rand"
	"strconv"
	"strings"
)

func (b *statefulBubble) View() string {
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
	lines := []string{
		style.Title("Loading"),
		"",
		b.loaderC.view(b.width),
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
	showPreview := b.thumbs.enabled && full >= maxListW+minPreviewW
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
	return b.renderLines(
		true,
		[]string{
			style.Title("Confirm"),
			"",
			fmt.Sprintf("%s Download %s?", icon.Get(icon.Question), util.Quantify(len(b.selectedChapters), "chapter", "chapters")),
		},
	)
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
	errorMsg := wrap.String(style.New().Italic(true).Foreground(color.Red).Render(b.lastError.Error()), b.width)
	return b.renderLines(
		true,
		append([]string{
			style.ErrorTitle("Error"),
			"",
			icon.Get(icon.Fail) + " Uggh, something went wrong. Maybe try again?",
			"",
		},
			strings.Split(wrap.String(style.Italic(b.errorPlot), b.width)+"\n\n"+errorMsg, "\n")...,
		),
	)
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
