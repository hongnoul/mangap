package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/source"
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
		b.mangasC.SetItems(nil)
		return nil
	}
	return tea.Tick(liveDebounce, func(time.Time) tea.Msg { return liveTickMsg{seq: seq, query: q} })
}

func (b *statefulBubble) runLive(m liveTickMsg) tea.Cmd {
	if m.seq != b.liveSeq || len(b.selectedSources) == 0 {
		return nil
	}
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

	header := box
	pickerW := lipgloss.Width(b.searchBox(boxW))
	var drops []string
	if b.dropdown != nil {
		drops = append(drops, b.viewDropdown(pickerW))
	}
	if b.showKeys {
		// Keybinds always stack below an open source/language picker.
		drops = append(drops, b.viewKeysBox(pickerW))
	}
	if len(drops) > 0 {
		// Expansions float over whatever is under the search bar
		// (art, results) so nothing below them moves.
		drop := lipgloss.PlaceHorizontal(full, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Left, drops...))
		return overlayLines(b.viewLiveBase(box, full, pickerW), drop, lipgloss.Height(box))
	}
	return b.viewLiveBase(header, full, pickerW)
}

// overlayLines replaces lines of base starting at row with the lines of top.
func overlayLines(base, top string, row int) string {
	bl := strings.Split(base, "\n")
	for i, l := range strings.Split(top, "\n") {
		if row+i < len(bl) {
			bl[row+i] = l
		} else {
			bl = append(bl, l)
		}
	}
	return strings.Join(bl, "\n")
}

// viewLiveBase renders the search screen under header, ignoring dropdowns.
func (b *statefulBubble) viewLiveBase(header string, full, pickerW int) string {
	homeView := b.inputC.Value() == "" && b.state == searchState
	footer := ""
	if len(b.sourceChoices) > 0 && homeView {
		footer = lipgloss.PlaceHorizontal(full, lipgloss.Center, b.sourcePicker(pickerW))
	}

	H := b.height + 2 - lipgloss.Height(header)
	if homeView {
		// Empty bar: art and keys fill the middle, the status box sits at
		// the very bottom.
		if footer == "" {
			return header + "\n" + b.homeBody(full, H)
		}
		// Downloaded box stacks above the status box, unless it would
		// push the art off a screen where the art otherwise fits.
		if dl := lipgloss.PlaceHorizontal(full, lipgloss.Center, b.downloadedBox(pickerW)); dl != "" {
			artH := len(homeArt)
			withDl := H - lipgloss.Height(footer) - lipgloss.Height(dl) - 1
			withoutDl := H - lipgloss.Height(footer) - 1
			if withDl >= artH || withoutDl < artH {
				footer = dl + "\n" + footer
			}
		}
		bodyH := H - lipgloss.Height(footer) - 1
		return header + "\n" + b.homeBody(full, imax(bodyH, 0)) + "\n" + footer
	}
	if len(b.mangasC.Items()) == 0 || H < 4 {
		return header
	}
	// Card height comes from the resting layout (collapsed bar, five
	// rows), so an open dropdown drops rows rather than shrinking them.
	restGridH := b.height + 2 - 3 - 2
	b.grid.fixedCardH = imax(restGridH/singleColRows, 2)
	defer func() { b.grid.fixedCardH = 0 }()
	grid := b.viewGridH(&b.mangasC, mangaCard, func(th *thumbs, it list.Item, prio int) {
		th.WantManga(it.(*listItem).internal.(*source.Manga), prio)
	}, H, true)
	return header + "\n" + grid
}
