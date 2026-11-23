package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// isArchiveKey matches ctrl+x, which opens the downloads archive.
func isArchiveKey(km tea.KeyMsg) bool { return km.String() == "ctrl+x" }

// inArchive reports whether the manga grid is showing the archive.
func (b *statefulBubble) inArchive() bool {
	return b.state == mangasState && b.arch.on
}

// openArchive shows the downloaded manga in the same grid as search
// results. Opening one lists its chapters, read straight from disk.
func (b *statefulBubble) openArchive() tea.Cmd {
	b.liveSeq++ // a late live search result must not replace the archive
	b.clearSuggestions()
	b.dropdown = nil
	b.showKeys = false

	b.arch.on = true
	b.arch.anchor = -1
	b.arch.editDir = false
	b.arch.dirErr = ""
	b.arch.dir.Blur()
	b.arch.filter.SetValue("")
	b.arch.typing = false
	b.arch.filter.Blur()
	b.loadArchiveItems()
	cmd := b.applyArchiveFilter()
	b.mangasC.ResetFilter()
	b.thumbs.Clear()
	b.preview.Clear()
	b.recent.Clear()
	if b.state == chaptersState {
		b.previousState() // back to whatever grid led here
	}
	if b.state != mangasState {
		b.newState(mangasState)
	}
	return cmd
}

// canOpenArchive limits ctrl+x to the browsing screens, never while a
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
