package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/archive"
	"github.com/hongnoul/mangap/where"
	"github.com/muesli/reflow/truncate"
)

// downloadedMaxRows caps how many manga the home Downloaded box lists.
const downloadedMaxRows = 5

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

	title := faint.Render("Downloaded  ") + bold.Render(fmt.Sprintf("%d manga", len(entries)))
	hint := "ctrl+d open"
	gap := imax(inner-lipgloss.Width(title)-lipgloss.Width(hint), 1)
	lines := []string{title + strings.Repeat(" ", gap) + faint.Render(hint)}

	if len(entries) == 0 {
		lines = append(lines, faint.Render("nothing yet, press d on a chapter to download"))
	}
	for i, e := range entries {
		if i == downloadedMaxRows {
			lines = append(lines, faint.Render(fmt.Sprintf("+ %d more", len(entries)-i)))
			break
		}
		ch := fmt.Sprintf("%d ch", len(e.Chapters))
		nameW := imax(inner-lipgloss.Width(ch)-2, 1)
		name := truncate.StringWithTail(e.Name, uint(nameW), "…")
		pad := imax(inner-lipgloss.Width(name)-lipgloss.Width(ch), 1)
		lines = append(lines, name+strings.Repeat(" ", pad)+faint.Render(ch))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(0, 1).
		Width(imax(w-2, 4)).
		Render(strings.Join(lines, "\n"))
}
