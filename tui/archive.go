package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hongnoul/mangap/archive"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/where"
)

// isArchiveKey matches ctrl+d, which opens the downloads archive.
func isArchiveKey(km tea.KeyMsg) bool { return km.String() == "ctrl+d" }

// inArchive reports whether the manga grid is showing the archive.
func (b *statefulBubble) inArchive() bool {
	if b.state != mangasState || len(b.mangasC.Items()) == 0 {
		return false
	}
	m, ok := b.mangasC.Items()[0].(*listItem).internal.(*source.Manga)
	return ok && m.Source != nil && m.Source.ID() == "downloads"
}

// openArchive shows the downloaded manga in the same grid as search
// results. Opening one lists its chapters, read straight from disk.
func (b *statefulBubble) openArchive() tea.Cmd {
	b.liveSeq++ // a late live search result must not replace the archive
	b.clearSuggestions()
	b.dropdown = nil
	b.showKeys = false

	mangas := archive.New(where.Downloads()).Mangas()
	items := make([]list.Item, len(mangas))
	for i, m := range mangas {
		items[i] = &listItem{internal: m}
	}
	cmd := b.mangasC.SetItems(items)
	b.mangasC.ResetSelected()
	b.mangasC.ResetFilter()
	b.thumbs.Clear()
	b.preview.Clear()
	if b.state == chaptersState {
		b.previousState() // back to whatever grid led here
	}
	if b.state != mangasState {
		b.newState(mangasState)
	}
	return cmd
}

// canOpenArchive limits ctrl+d to the browsing screens, never while a
// list filter is being typed.
func (b *statefulBubble) canOpenArchive() bool {
	switch b.state {
	case searchState:
		return true
	case mangasState:
		return b.mangasC.FilterState() != list.Filtering
	case chaptersState:
		return b.chaptersC.FilterState() != list.Filtering
	}
	return false
}

// inArchiveChapters reports whether the chapter grid lists archive files.
func (b *statefulBubble) inArchiveChapters() bool {
	return b.state == chaptersState && b.selectedManga != nil &&
		b.selectedManga.Source != nil && b.selectedManga.Source.ID() == "downloads"
}
