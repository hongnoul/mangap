package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/provider"
	"github.com/spf13/viper"
)

func (b *statefulBubble) Init() tea.Cmd {
	if b.state == historyState || b.state == scrapersInstallState {
		return tea.Batch(textinput.Blink, b.loadProviders())
	}

	// Explicit sources (-S flag or downloader.default_sources) win.
	if names := viper.GetStringSlice(key.DownloaderDefaultSources); len(names) != 0 {
		var providers []*provider.Provider

		for _, name := range names {
			p, ok := provider.Get(name)
			if !ok {
				b.raiseError(fmt.Errorf("provider %s not found", name))
				return nil
			}

			providers = append(providers, p)
		}

		b.setState(loadingState)
		return tea.Batch(b.startLoading(), b.loadSources(providers), b.waitForSourcesLoaded())
	}

	// Auto mode: straight to search, sources tried in priority order.
	if b.state == searchState {
		return textinput.Blink
	}

	return tea.Batch(textinput.Blink, b.loadProviders())
}
