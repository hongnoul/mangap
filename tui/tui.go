package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hongnoul/mangaz/provider"
	"github.com/hongnoul/mangaz/source"
)

type Options struct {
	Continue bool
	Install  bool
}

func Run(options *Options) error {

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

	return tea.NewProgram(bubble, tea.WithAltScreen()).Start()
}
