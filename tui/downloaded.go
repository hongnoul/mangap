package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/archive"
	"github.com/hongnoul/mangap/where"
	"github.com/muesli/reflow/truncate"
)

// downloadedMaxRows caps how many manga the home Downloaded box lists.
const downloadedMaxRows = 4

// downloadedTTL is how long a scan of the downloads dir is reused, so
// rendering the home screen does not hit the disk every frame.
const downloadedTTL = 3 * time.Second

type downloadedCache struct {
	at      time.Time
	dir     string
	entries []archive.Entry
}

var downloadedC downloadedCache

// scanDownloaded lists the downloaded manga, newest first (cached).
func scanDownloaded() []archive.Entry {
	dir := where.Downloads()
	if time.Since(downloadedC.at) < downloadedTTL && downloadedC.dir == dir {
		return downloadedC.entries
	}
	entries := archive.Scan(dir)
	downloadedC = downloadedCache{at: time.Now(), dir: dir, entries: entries}
	return entries
}

// invalidateDownloaded forces the next render to rescan (after a download).
func invalidateDownloaded() { downloadedC.at = time.Time{} }

// downloadedBox renders the home screen list of downloaded manga, w wide.
func (b *statefulBubble) downloadedBox(w int) string {
	entries := scanDownloaded()
	inner := imax(w-4, 4)
	bold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	faint := lipgloss.NewStyle().Faint(true)

	// Fixed height so the cover thumbnail is always four rows tall.
	rows := downloadedMaxRows
	// Cover of the newest download sits left of the list, fit to the
	// box height (a ~2:3 cover spans ~4/3 cols per row, letterboxed).
	key, cols := "", 0
	if len(entries) > 0 && b.recent != nil && b.recent.active() {
		if k := recentThumbKey(entries[0]); k != "" && inner-imax(rows*4/3, 2)-2 >= 16 {
			key, cols = k, imax(rows*4/3, 2)
			b.recent.SetSpan(cols, rows)
			b.recent.Retain(map[string]bool{k: true})
			b.recent.want(k, prioVisible, nil)
		}
	}
	textW := inner
	if key != "" {
		textW = inner - cols - 2
	}

	lines := make([]string, 0, rows)
	if len(entries) == 0 {
		lines = append(lines, faint.Render(truncate.StringWithTail("press d on a selection of comics or chapters to download", uint(textW), "…")))
	}
	for _, e := range entries {
		if len(lines) >= rows {
			break
		}
		ch := fmt.Sprintf("%d ch", len(e.Chapters))
		name := truncate.StringWithTail(e.Name, uint(imax(textW-lipgloss.Width(ch)-2, 1)), "…")
		pad := imax(textW-lipgloss.Width(name)-lipgloss.Width(ch), 1)
		lines = append(lines, name+strings.Repeat(" ", pad)+faint.Render(ch))
	}
	// Fixed height, padded when short.
	for len(lines) < rows {
		lines = append(lines, "")
	}
	if key != "" {
		for i := range lines {
			lines[i] = b.recent.Cells(key, i) + "  " + lines[i]
		}
	}

	border := lipgloss.Color("5")
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(border).
		Padding(0, 1).
		Width(imax(w-2, 4)).
		Render(strings.Join(lines, "\n"))

	// Top border carries the label: ╭─ Downloaded ──── ctrl+x ─╮
	bw := lipgloss.Width(body)
	bc := lipgloss.NewStyle().Foreground(border)
	label := " " + bold.Render("Downloaded") + " "
	hint := " " + faint.Render("ctrl+x") + " "
	fill := imax(bw-2-1-1-lipgloss.Width(label)-lipgloss.Width(hint), 1)
	top := bc.Render("╭─") + label + bc.Render(strings.Repeat("─", fill)) + hint + bc.Render("─╮")
	return top + "\n" + body
}

// downloadQueueLines lists the chapter being fetched (spinner and n/total
// on the right) followed by the queued chapter names, at most max rows.
func (b *statefulBubble) downloadQueueLines(w, max int) []string {
	cur := b.dlCurrent
	if cur == nil {
		return nil
	}
	faint := lipgloss.NewStyle().Faint(true)
	row := func(name, right string) string {
		name = truncate.StringWithTail(name, uint(imax(w-lipgloss.Width(right)-2, 1)), "…")
		pad := imax(w-lipgloss.Width(name)-lipgloss.Width(right), 1)
		return name + strings.Repeat(" ", pad) + right
	}
	label := cur.Name
	if m := cur.Manga; m != nil {
		label = m.Name + " · " + cur.Name
	}
	right := faint.Render(fmt.Sprintf("%d/%d ", b.dlDone+1, b.dlTotal)) + b.spinnerC.View()
	lines := []string{row(label, right)}
	for _, c := range b.chaptersToDownload.Items() {
		if len(lines) >= max {
			break
		}
		lines = append(lines, faint.Render(truncate.StringWithTail(c.Name, uint(imax(w, 1)), "…")))
	}
	return lines
}

// recentThumbKey is the local image key for an entry: its saved cover, or
// the first page of its newest chapter.
func recentThumbKey(e archive.Entry) string {
	if e.Cover != "" {
		return archive.FileURL(e.Cover)
	}
	if e.Latest != "" {
		return archive.FileURL(e.Latest)
	}
	if n := len(e.Chapters); n > 0 {
		return archive.FileURL(e.Chapters[n-1].Path)
	}
	return ""
}

// dlAccent marks downloaded manga in search results, matching the
// Downloaded box border.
var dlAccent = lipgloss.Color("247")

// normTitle folds a title for matching results against the archive.
func normTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// downloadedChapters is how many chapters of the titled manga are in the
// downloads archive, 0 when none.
func downloadedChapters(name string) int {
	n := normTitle(name)
	if n == "" {
		return 0
	}
	for _, e := range scanDownloaded() {
		if normTitle(e.Name) == n {
			return len(e.Chapters)
		}
	}
	return 0
}

// downloadedEntry is the archive entry of the titled manga, if any.
func downloadedEntry(name string) (archive.Entry, bool) {
	n := normTitle(name)
	if n == "" {
		return archive.Entry{}, false
	}
	for _, e := range scanDownloaded() {
		if normTitle(e.Name) == n {
			return e, true
		}
	}
	return archive.Entry{}, false
}
