package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangaz/provider"
	"github.com/hongnoul/mangaz/source"
	"github.com/muesli/termenv"
)

type Options struct {
	Continue bool
	Install  bool
}

func Run(options *Options) error {
	// Monochrome UI: drop every color (ours and the bubbles defaults) and
	// keep bold/faint/reverse for emphasis.
	lipgloss.SetColorProfile(termenv.Ascii)

	bubble := newBubble()

	if options.Install {
		bubble.newState(scrapersInstallState)
	} else if options.Continue {
		_, err := bubble.loadHistory()
		if err != nil {
			return err
		}

		bubble.newState(historyState)
	} else if provider.IsAuto() {
		auto, err := provider.NewAuto()
		if err != nil {
			return err
		}
		bubble.selectedSources = []source.Source{auto}
		bubble.newState(searchState)
	} else {
		bubble.newState(sourcesState)
	}

	out := &syncOut{f: os.Stdout}
	*bubble.thumbs = *newThumbs(out)
	p := tea.NewProgram(bubble, tea.WithAltScreen(), tea.WithOutput(out))
	bubble.thumbs.send = p.Send
	defer bubble.thumbs.Clear()
	return p.Start()
}
