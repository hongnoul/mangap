package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/constant"
	"github.com/hongnoul/mangap/filesystem"
	"github.com/hongnoul/mangap/where"
	"github.com/muesli/reflow/truncate"
)

// downloadedMaxRows caps how many manga the home Downloaded box lists.
const downloadedMaxRows = 5

// downloadedTTL is how long a scan of the downloads dir is reused, so
// rendering the home screen does not hit the disk every frame.
const downloadedTTL = 3 * time.Second

type downloadedManga struct {
	name     string
	chapters int
	modTime  time.Time
}

type downloadedCache struct {
	at     time.Time
	dir    string
	mangas []downloadedManga
}

var downloadedC downloadedCache

// isChapterEntry reports whether a dir entry looks like a downloaded chapter.
func isChapterEntry(fi os.FileInfo) bool {
	name := strings.ToLower(fi.Name())
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "cover.") || name == "series.json" {
		return false
	}
	if fi.IsDir() {
		return true // plain format: one dir of images per chapter
	}
	switch strings.TrimPrefix(filepath.Ext(name), ".") {
	case constant.FormatPDF, constant.FormatCBZ, constant.FormatZIP:
		return true
	}
	return false
}

// scanDownloaded lists manga dirs under the downloads path that hold at
// least one chapter, newest first.
func scanDownloaded() (string, []downloadedManga) {
	dir := where.Downloads()
	if time.Since(downloadedC.at) < downloadedTTL && downloadedC.dir == dir {
		return dir, downloadedC.mangas
	}
	fs := filesystem.Api()
	var out []downloadedManga
	entries, _ := fs.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		subs, err := fs.ReadDir(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		m := downloadedManga{name: e.Name(), modTime: e.ModTime()}
		for _, s := range subs {
			if !isChapterEntry(s) {
				continue
			}
			// A plain-format chapter dir has images, not more dirs, so a
			// dir of dirs (e.g. some unrelated project) is not a manga.
			if s.IsDir() && !hasImages(filepath.Join(dir, e.Name(), s.Name())) {
				continue
			}
			m.chapters++
			if s.ModTime().After(m.modTime) {
				m.modTime = s.ModTime()
			}
		}
		if m.chapters > 0 {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].modTime.After(out[j].modTime) })
	downloadedC = downloadedCache{at: time.Now(), dir: dir, mangas: out}
	return dir, out
}

func hasImages(dir string) bool {
	subs, err := filesystem.Api().ReadDir(dir)
	if err != nil {
		return false
	}
	for _, s := range subs {
		switch strings.ToLower(filepath.Ext(s.Name())) {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".avif":
			return true
		}
	}
	return false
}

// invalidateDownloaded forces the next render to rescan (after a download).
func invalidateDownloaded() { downloadedC.at = time.Time{} }

// downloadedBox renders the home screen list of downloaded manga, w wide.
func (b *statefulBubble) downloadedBox(w int) string {
	dir, mangas := scanDownloaded()
	inner := imax(w-4, 4)
	bold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	faint := lipgloss.NewStyle().Faint(true)

	count := fmt.Sprintf("%d manga", len(mangas))
	if len(mangas) == 1 {
		count = "1 manga"
	}
	title := faint.Render("Downloaded  ") + bold.Render(count)
	path := shortenHome(dir)
	gap := inner - lipgloss.Width(title) - lipgloss.Width(path) - 1
	if gap >= 1 {
		title += strings.Repeat(" ", gap+1) + faint.Render(path)
	}
	lines := []string{title}

	if len(mangas) == 0 {
		lines = append(lines, faint.Render("nothing yet, press d on a chapter to download"))
	}
	for i, m := range mangas {
		if i == downloadedMaxRows {
			lines = append(lines, faint.Render(fmt.Sprintf("+ %d more", len(mangas)-i)))
			break
		}
		ch := fmt.Sprintf("%d ch", m.chapters)
		nameW := imax(inner-lipgloss.Width(ch)-2, 1)
		name := truncate.StringWithTail(m.name, uint(nameW), "…")
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

// shortenHome replaces the home dir prefix with ~.
func shortenHome(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
