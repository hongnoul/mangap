package tui

import (
	"github.com/hongnoul/mangap/reader/term"
	xterm "golang.org/x/term"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/source"
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
	*bubble.preview = *newThumbs(out)
	bubble.preview.next = previewBaseID
	p := tea.NewProgram(bubble, tea.WithAltScreen(), tea.WithOutput(out))
	bubble.thumbs.send = p.Send
	bubble.preview.send = p.Send
	defer bubble.preview.Clear()
	defer bubble.thumbs.Clear()

	// bubbletea only watches SIGWINCH when its output is an *os.File, and
	// syncOut is not, so forward resizes ourselves. Without this the TUI
	// keeps its startup width and wraps when the pane shrinks.
	resizes, stop := term.Resizes()
	defer stop()
	go func() {
		for range resizes {
			if w, h, err := xterm.GetSize(int(os.Stdout.Fd())); err == nil {
				p.Send(tea.WindowSizeMsg{Width: w, Height: h})
			}
		}
	}()
	go func() {
		if w, h, err := xterm.GetSize(int(os.Stdout.Fd())); err == nil {
			p.Send(tea.WindowSizeMsg{Width: w, Height: h})
		}
	}()
	return p.Start()
}
