package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// vimHL makes h go back (esc) and l go forward (enter) on list screens,
// next to j/k for up/down. Text input (search, list filtering) keeps the
// letters.
func (b *statefulBubble) vimHL(msg tea.Msg) tea.Msg {
	km, ok := msg.(tea.KeyMsg)
	if !ok || km.Type != tea.KeyRunes || len(km.Runes) != 1 {
		return msg
	}
	r := km.Runes[0]
	if r != 'h' && r != 'l' {
		return msg
	}

	var l *list.Model
	switch b.state {
	case historyState:
		l = &b.historyC
	case sourcesState:
		l = &b.sourcesC
	case mangasState, chaptersState:
		// Grid screens: h/l move left/right in the grid (see grid.nav);
		// esc/enter are back/open.
		return msg
	case scrapersInstallState:
		l = &b.scrapersInstallC
	case sourceSettingsState, confirmState, errorState:
	default:
		return msg
	}
	if l != nil && l.FilterState() == list.Filtering {
		return msg
	}

	if r == 'h' {
		// The first screen has nowhere to go back to.
		if b.statesHistory.Len() == 0 {
			return nil
		}
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyEnter}
}
