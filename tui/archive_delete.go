package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/color"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
	"github.com/hongnoul/mangap/util"
	"github.com/hongnoul/mangap/where"
)

// archiveDelete is a pending "delete from disk" confirmation in the
// downloads viewer, drawn as a popup over the current grid.
type archiveDelete struct {
	on       bool
	mangas   []*source.Manga   // whole comics to remove
	chapters []*source.Chapter // single chapters to remove
	err      string
}

// archiveMarked is the downloaded manga chosen with v, else the focused one.
func (b *statefulBubble) archiveMarked() []*source.Manga {
	b.applyArchiveVisual()
	var out []*source.Manga
	for _, it := range b.mangasC.Items() {
		if li := it.(*listItem); li.marked {
			out = append(out, li.internal.(*source.Manga))
		}
	}
	if len(out) == 0 {
		if it := b.mangasC.SelectedItem(); it != nil {
			out = append(out, it.(*listItem).internal.(*source.Manga))
		}
	}
	return out
}

// applyArchiveVisual marks the manga between the v anchor and the cursor.
func (b *statefulBubble) applyArchiveVisual() {
	if !b.inArchive() || b.arch.anchor < 0 {
		return
	}
	lo_, hi := b.arch.anchor, b.mangasC.Index()
	if lo_ > hi {
		lo_, hi = hi, lo_
	}
	for i, it := range b.mangasC.Items() {
		it.(*listItem).marked = i >= lo_ && i <= hi
	}
}

// clearArchiveSelection leaves select mode and unmarks every manga.
func (b *statefulBubble) clearArchiveSelection() bool {
	had := b.arch.anchor >= 0
	b.arch.anchor = -1
	for _, it := range b.arch.all {
		li := it.(*listItem)
		had = had || li.marked
		li.marked = false
	}
	return had
}

// askDeleteMangas opens the confirmation for the marked/focused comics.
func (b *statefulBubble) askDeleteMangas() {
	ms := b.archiveMarked()
	if len(ms) == 0 {
		return
	}
	b.arch.anchor = -1
	b.del = archiveDelete{on: true, mangas: ms}
}

// askDeleteChapters opens the confirmation for the marked/focused chapters.
func (b *statefulBubble) askDeleteChapters() {
	var chs []*source.Chapter
	for _, it := range b.chaptersC.Items() {
		c := it.(*listItem).internal.(*source.Chapter)
		if _, ok := b.selectedChapters[c]; ok {
			chs = append(chs, c)
		}
	}
	if len(chs) == 0 {
		it := b.chaptersC.SelectedItem()
		if it == nil {
			return
		}
		chs = append(chs, it.(*listItem).internal.(*source.Chapter))
	}
	b.visualAnchor = -1
	b.del = archiveDelete{on: true, chapters: chs}
}

// insideDownloads guards RemoveAll: only paths under the save dir go.
func insideDownloads(p string) bool {
	root, err1 := filepath.Abs(where.Downloads())
	abs, err2 := filepath.Abs(p)
	if err1 != nil || err2 != nil || abs == root {
		return false
	}
	rel, err := filepath.Rel(root, abs)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func chapterPath(c *source.Chapter) string {
	if c.Manga == nil {
		return c.ID
	}
	if ls, ok := c.Manga.Source.(interface{ PathOf(*source.Chapter) string }); ok {
		return ls.PathOf(c)
	}
	return c.ID
}

// runArchiveDelete removes the confirmed files and refreshes the views.
func (b *statefulBubble) runArchiveDelete() tea.Cmd {
	d := b.del
	var errs []string
	remove := func(p string) bool {
		if !insideDownloads(p) {
			errs = append(errs, "refusing to delete "+p)
			return false
		}
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, err.Error())
			return false
		}
		return true
	}
	for _, m := range d.mangas {
		remove(m.ID)
	}
	gone := map[*source.Chapter]bool{}
	for _, c := range d.chapters {
		if remove(chapterPath(c)) {
			gone[c] = true
		}
	}
	if len(errs) > 0 {
		b.del.err = errs[0]
		return nil
	}
	b.del = archiveDelete{}

	invalidateDownloaded()
	b.thumbs.Clear()
	b.preview.Clear()
	b.recent.Clear()
	b.clearArchiveSelection()
	b.loadArchiveItems()
	cmds := []tea.Cmd{b.applyArchiveFilter()}

	if len(gone) > 0 && b.state == chaptersState {
		b.clearChapterSelection()
		var keep []list.Item
		for _, it := range b.chaptersC.Items() {
			if !gone[it.(*listItem).internal.(*source.Chapter)] {
				keep = append(keep, it)
			}
		}
		if len(keep) == 0 {
			b.previousState() // the comic folder is empty now
			if m := b.selectedManga; m != nil && insideDownloads(m.ID) {
				if left, _ := os.ReadDir(m.ID); !hasChapterLeft(left) {
					_ = os.RemoveAll(m.ID)
					b.loadArchiveItems()
					cmds = append(cmds, b.applyArchiveFilter())
				}
			}
		} else {
			idx := b.chaptersC.Index()
			cmds = append(cmds, b.chaptersC.SetItems(keep))
			b.chaptersC.Select(imin(idx, len(keep)-1))
		}
	}
	return tea.Batch(cmds...)
}

// hasChapterLeft: anything besides metadata/cover files counts.
func hasChapterLeft(es []os.DirEntry) bool {
	for _, e := range es {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			return true
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".cbz", ".zip", ".pdf", ".epub", ".cbr", ".rar":
			return true
		}
	}
	return false
}

// updateArchiveDelete handles keys while the confirmation is up.
func (b *statefulBubble) updateArchiveDelete(km tea.KeyMsg) tea.Cmd {
	switch km.String() {
	case "enter", "y", "Y":
		return b.runArchiveDelete()
	case "esc", "n", "N", "q":
		b.del = archiveDelete{}
	case "ctrl+c":
		return tea.Quit
	}
	return nil
}

func (b *statefulBubble) deleteQuestion() string {
	name := func(s string) string { return style.Fg(color.Red)(s) }
	switch d := b.del; {
	case len(d.mangas) == 1:
		return fmt.Sprintf("Delete %s from disk?", name(d.mangas[0].Name))
	case len(d.mangas) > 1:
		return fmt.Sprintf("Delete %s from disk?", name(util.Quantify(len(d.mangas), "comic", "comics")))
	case len(d.chapters) == 1:
		return fmt.Sprintf("Delete %s from disk?", name(d.chapters[0].Name))
	default:
		return fmt.Sprintf("Delete %s from disk?", name(util.Quantify(len(d.chapters), "chapter", "chapters")))
	}
}

// viewArchiveDelete draws the confirmation over base.
func (b *statefulBubble) viewArchiveDelete(base string) string {
	q := b.deleteQuestion()
	hint := style.Faint("enter/y delete · esc/n cancel")
	if b.del.err != "" {
		hint = lipgloss.NewStyle().Foreground(color.Red).Render(b.del.err)
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color.Red).
		Padding(1, 3).
		Align(lipgloss.Center).
		Width(imin(imax(lipgloss.Width(q), lipgloss.Width(hint))+8, imax(b.width-4, 20))).
		Render(q + "\n\n" + hint)
	return overlayCenter(base, box)
}
