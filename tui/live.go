package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
	"github.com/hongnoul/mangap/util"
	"github.com/muesli/reflow/truncate"
)

// Live search: while typing on the search screen, the top manga for the
// query is looked up (debounced) and its chapters shown under the bar.

const liveDebounce = 250 * time.Millisecond

type liveTickMsg struct {
	seq   int
	query string
}

type liveResultMsg struct {
	seq    int
	mangas []*source.Manga
}

// scheduleLive bumps the sequence (dropping stale lookups) and starts the
// debounce timer for the current input.
func (b *statefulBubble) scheduleLive() tea.Cmd {
	b.liveSeq++
	seq, q := b.liveSeq, b.inputC.Value()
	if q == "" {
		b.liveStatus = ""
		b.mangasC.SetItems(nil)
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
		var all []*source.Manga
		for _, s := range srcs {
			mangas, err := s.Search(m.query)
			if err != nil {
				log.Error(err)
				continue
			}
			all = append(all, mangas...)
		}
		return liveResultMsg{seq: m.seq, mangas: all}
	}
}

func (b *statefulBubble) applyLive(m liveResultMsg) tea.Cmd {
	if m.seq != b.liveSeq {
		return nil
	}
	b.liveStatus = util.Quantify(len(m.mangas), "result", "results")
	items := make([]list.Item, len(m.mangas))
	for i, mg := range m.mangas {
		items[i] = &listItem{internal: mg}
	}
	cmd := b.mangasC.SetItems(items)
	b.mangasC.ResetSelected()
	return cmd
}

// viewLive is the search screen: a full-width search bar on top and the
// live manga results below it. Before anything is
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
	if b.state == searchState && len(b.mangasC.Items()) > 0 {
		status += style.Faint("  ·  ctrl+j/k open results")
	} else if b.state == mangasState {
		status += style.Faint("  ·  ctrl+j/k or esc back to search")
	}
	header := box + "\n  " + truncate.StringWithTail(status, uint(imax(full-4, 1)), "…")

	H := b.height + 2 - lipgloss.Height(header)
	if len(b.mangasC.Items()) == 0 || H < 4 {
		return header
	}
	grid := b.viewGridH(&b.mangasC, mangaCard, func(th *thumbs, it list.Item, prio int) {
		th.WantManga(it.(*listItem).internal.(*source.Manga), prio)
	}, H)
	return header + "\n" + grid
}
