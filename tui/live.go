package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
	"github.com/muesli/reflow/truncate"
	"github.com/spf13/viper"
)

// Live search: while typing on the search screen, the top manga for the
// query is looked up (debounced) and its chapters shown under the bar.

const liveDebounce = 250 * time.Millisecond

type liveTickMsg struct {
	seq   int
	query string
}

type liveResultMsg struct {
	seq      int
	manga    *source.Manga
	chapters []*source.Chapter
	err      error
}

// scheduleLive bumps the sequence (dropping stale lookups) and starts the
// debounce timer for the current input.
func (b *statefulBubble) scheduleLive() tea.Cmd {
	b.liveSeq++
	seq, q := b.liveSeq, b.inputC.Value()
	if q == "" {
		b.liveManga = nil
		b.liveStatus = ""
		b.chaptersC.SetItems(nil)
		return nil
	}
	return tea.Tick(liveDebounce, func(time.Time) tea.Msg { return liveTickMsg{seq: seq, query: q} })
}

func (b *statefulBubble) runLive(m liveTickMsg) tea.Cmd {
	if m.seq != b.liveSeq || len(b.selectedSources) == 0 {
		return nil
	}
	b.liveStatus = "Searching…"
	srcs := b.selectedSources
	return func() tea.Msg {
		for _, s := range srcs {
			mangas, err := s.Search(m.query)
			if err != nil {
				log.Error(err)
				continue
			}
			if len(mangas) == 0 {
				continue
			}
			manga := mangas[0]
			chapters, err := manga.Source.ChaptersOf(manga)
			return liveResultMsg{seq: m.seq, manga: manga, chapters: chapters, err: err}
		}
		return liveResultMsg{seq: m.seq}
	}
}

func (b *statefulBubble) applyLive(m liveResultMsg) tea.Cmd {
	if m.seq != b.liveSeq {
		return nil
	}
	if m.err != nil {
		b.liveStatus = "Error: " + m.err.Error()
		return nil
	}
	if m.manga == nil {
		b.liveManga = nil
		b.liveStatus = "No results"
		return b.chaptersC.SetItems(nil)
	}
	b.liveManga = m.manga
	b.liveStatus = m.manga.Name
	b.selectedManga = m.manga
	items := make([]list.Item, len(m.chapters))
	for i, c := range m.chapters {
		if viper.GetBool(key2.TUIReverseChapters) {
			items[len(m.chapters)-i-1] = &listItem{internal: c}
		} else {
			items[i] = &listItem{internal: c}
		}
	}
	cmd := b.chaptersC.SetItems(items)
	b.chaptersC.ResetSelected()
	b.clearChapterSelection()
	return tea.Batch(cmd, b.selectBookmark())
}

// viewLive is the search screen: a full-width search bar on top and the
// live chapter results of the top match below it. Before anything is
// typed the home screen (art + keys) shows instead.
func (b *statefulBubble) viewLive() string {
	if b.inputC.Value() == "" && b.state == searchState {
		return b.viewHome()
	}
	full := b.width + 4
	boxW := imax(full-4, 12)
	b.inputC.Width = imax(boxW-6-lipgloss.Width(b.inputC.Prompt), 4)
	box := b.searchBox(boxW)
	for lipgloss.Width(box) > full-2 && boxW > 12 {
		boxW--
		b.inputC.Width = imax(boxW-6-lipgloss.Width(b.inputC.Prompt), 4)
		box = b.searchBox(boxW)
	}
	box = lipgloss.PlaceHorizontal(full, lipgloss.Center, box)

	status := b.liveStatus
	if b.state == searchState && len(b.chaptersC.Items()) > 0 {
		status += style.Faint("  ·  ctrl+j/k open chapters")
	} else if b.state == chaptersState {
		status += style.Faint("  ·  ctrl+j/k or esc back to search")
	}
	header := box + "\n  " + truncate.StringWithTail(status, uint(imax(full-4, 1)), "…")

	H := b.height + 2 - lipgloss.Height(header)
	if len(b.chaptersC.Items()) == 0 || H < 4 {
		return header
	}
	grid := b.viewGridH(&b.chaptersC, chapterCard, func(th *thumbs, it list.Item, prio int) {
		th.WantChapter(it.(*listItem).internal.(*source.Chapter), prio)
	}, H)
	return header + "\n" + grid
}
