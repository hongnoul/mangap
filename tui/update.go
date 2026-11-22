package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hongnoul/mangap/downloader"
	"github.com/hongnoul/mangap/history"
	"github.com/hongnoul/mangap/installer"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/query"
	"github.com/hongnoul/mangap/reader"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/util"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"time"
)

func (b *statefulBubble) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	msg = b.vimHL(msg)
	if _, ok := msg.(thumbReadyMsg); ok {
		return b, nil // a cover landed: the repaint after Update shows it
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		liveToggle := b.liveMode && b.state == mangasState && (km.String() == "ctrl+j" || km.String() == "ctrl+k")
		if l := b.gridList(); l != nil && !liveToggle && l.FilterState() != list.Filtering {
			if b.grid.key(km.String()) || b.grid.nav(l, km.String()) {
				b.applyVisual()
				return b, nil
			}
		}
	}

	switch msg := msg.(type) {
	case error:
		b.raiseError(msg)
	case tea.WindowSizeMsg:
		b.resize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, b.keymap.forceQuit):
			return b, tea.Quit
		case key.Matches(msg, b.keymap.back):
			onListBack := func(l *list.Model) tea.Cmd {
				l.ResetSelected()
				l.ResetFilter()

				return tea.Batch(cmd, l.NewStatusMessage(""))
			}

			switch b.state {
			case searchState:
				if len(b.searchSuggestions) > 0 {
					b.clearSuggestions()
					return b, nil
				}
				b.inputC.SetValue("")
			case chaptersState:
				if b.chaptersC.FilterState() != list.Unfiltered {
					b.chaptersC, cmd = b.chaptersC.Update(msg)
					return b, cmd
				}

				if b.visualAnchor >= 0 || len(b.selectedChapters) > 0 {
					b.clearChapterSelection()
					return b, nil
				}
				cmd = onListBack(&b.chaptersC)
			case mangasState:
				if b.liveMode {
					b.liveMode = false
					b.inputC.Focus()
					b.previousState()
					return b, nil
				}
				if b.mangasC.FilterState() != list.Unfiltered {
					b.mangasC, cmd = b.mangasC.Update(msg)
					return b, cmd
				}

				cmd = onListBack(&b.mangasC)
			case historyState:
				if b.historyC.FilterState() != list.Unfiltered {
					b.historyC, cmd = b.historyC.Update(msg)
					return b, cmd
				}

				cmd = onListBack(&b.historyC)
			case sourcesState:
				if b.sourcesC.FilterState() != list.Unfiltered {
					b.sourcesC, cmd = b.sourcesC.Update(msg)
					return b, cmd
				}

				cmd = onListBack(&b.sourcesC)
			case scrapersInstallState:
				if b.scrapersInstallC.FilterState() != list.Unfiltered {
					b.scrapersInstallC, cmd = b.scrapersInstallC.Update(msg)
					return b, cmd
				}

				cmd = onListBack(&b.scrapersInstallC)
			}

			b.previousState()
			b.stopLoading()
			b.failedChapters = make([]*source.Chapter, 0)
			b.succededChapters = make([]*source.Chapter, 0)
			return b, cmd
		}
	}

	switch b.state {
	case loadingState:
		return b.updateLoading(msg)
	case historyState:
		return b.updateHistory(msg)
	case sourcesState:
		return b.updateSources(msg)
	case searchState:
		return b.updateSearch(msg)
	case mangasState:
		return b.updateMangas(msg)
	case chaptersState:
		return b.updateChapters(msg)
	case confirmState:
		return b.updateConfirm(msg)
	case readState:
		return b.updateRead(msg)
	case downloadState:
		return b.updateDownload(msg)
	case downloadDoneState:
		return b.updateDownloadDone(msg)
	case scrapersInstallState:
		return b.updateScrapersInstall(msg)
	case errorState:
		return b.updateError(msg)
	case sourceSettingsState:
		return b.updateSourceSettings(msg)
	}

	panic("unreachable")
}

func (b *statefulBubble) updateScrapersInstall(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if len(b.scrapersInstallC.Items()) == 0 {
		b.newState(loadingState)
		return b, tea.Batch(b.startLoading(), b.loadScrapers(), b.waitForScrapersLoaded())
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case b.scrapersInstallC.FilterState() == list.Filtering:
			break
		case key.Matches(msg, b.keymap.confirm):
			scraper := b.scrapersInstallC.SelectedItem().(*listItem).internal.(*installer.Scraper)
			b.newState(loadingState)
			return b, tea.Batch(b.startLoading(), b.installScraper(scraper), b.waitForScraperInstallation())
		}
	}

	b.scrapersInstallC, cmd = b.scrapersInstallC.Update(msg)
	return b, cmd
}

func (b *statefulBubble) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds = make([]tea.Cmd, 0)
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, b.keymap.back):
			b.previousState()
		}
	case []*installer.Scraper:
		b.newState(scrapersInstallState)
		return b, b.stopLoading()
	case *installer.Scraper:
		b.newState(scrapersInstallState)
		b.scrapersInstallC.NewStatusMessage(fmt.Sprintf("Installed %s", msg.Name))
		return b, b.stopLoading()
	case []*source.Manga:
		items := make([]list.Item, len(msg))
		for i, m := range msg {
			items[i] = &listItem{internal: m}
		}

		cmds = append(cmds, b.mangasC.SetItems(items))
		b.thumbs.Clear()
		b.preview.Clear()
		b.newState(mangasState)
		b.stopLoading()
	case []*source.Chapter:
		if b.statesHistory.Peek() == historyState {
			b.newState(historyState)
			b.stopLoading()
			cmds = append(cmds, func() tea.Msg {
				return msg
			})
		}
	case []source.Source:
		b.selectedSources = msg

		if b.statesHistory.Peek() == historyState {
			b.newState(historyState)
			b.stopLoading()
			cmds = append(cmds, func() tea.Msg {
				return msg
			})
		} else {
			b.stopLoading()
			b.newState(searchState)
		}
	}

	b.spinnerC, cmd = b.spinnerC.Update(msg)
	return b, tea.Batch(append(cmds, cmd)...)
}

func (b *statefulBubble) updateHistory(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case []source.Source:
		b.selectedSources = msg
		selected := b.historyC.SelectedItem().(*listItem).internal.(*history.SavedChapter)

		manga := &source.Manga{
			Name:   selected.MangaName,
			URL:    selected.MangaURL,
			Index:  0,
			ID:     selected.MangaID,
			Source: b.selectedSources[0],
		}

		b.selectedManga = manga
		b.newState(loadingState)
		return b, tea.Batch(
			b.startLoading(),
			b.getChapters(manga),
			b.waitForChapters(),
		)
	case []*source.Chapter:
		items := make([]list.Item, len(msg))
		selected := b.historyC.SelectedItem().(*listItem).internal.(*history.SavedChapter)

		for i, c := range msg {
			items[i] = &listItem{internal: c}
		}

		cmd = b.chaptersC.SetItems(items)
		b.clearChapterSelection()
		b.newState(chaptersState)
		b.stopLoading()
		b.markBookmark(selected)
		selectCmd := b.selectChapterBy(selected.Matches)
		return b, tea.Batch(cmd, selectCmd)
	case tea.KeyMsg:
		switch {
		case b.historyC.FilterState() == list.Filtering:
			break
		case key.Matches(msg, b.keymap.remove):
			if b.historyC.SelectedItem() != nil {
				chapter := b.historyC.SelectedItem().(*listItem).internal.(*history.SavedChapter)
				_ = history.Remove(chapter)
				cmd, err := b.loadHistory()
				if err != nil {
					return nil, nil
				}

				return b, cmd
			}
		case key.Matches(msg, b.keymap.confirm):
			if b.historyC.SelectedItem() != nil {
				selected := b.historyC.SelectedItem().(*listItem).internal.(*history.SavedChapter)
				p, ok := provider.GetByID(selected.SourceID)

				if !ok {
					err := fmt.Errorf("provider %s not found", selected.SourceID)
					b.raiseError(err)
					return b, nil
				}

				b.newState(loadingState)
				return b, tea.Batch(b.startLoading(), b.loadSources([]*provider.Provider{p}), b.waitForSourcesLoaded())
			}
		}
	}

	b.historyC, cmd = b.historyC.Update(msg)
	return b, cmd
}

func (b *statefulBubble) updateSources(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case b.sourcesC.FilterState() == list.Filtering:
			break
		case key.Matches(msg, b.keymap.sourceSettings):
			b.openSourceSettings()
			return b, nil
		case key.Matches(msg, b.keymap.confirm):
			if b.sourcesC.SelectedItem() == nil {
				break
			}

			item := b.sourcesC.SelectedItem().(*listItem)
			defer b.newState(loadingState)
			return b, tea.Batch(b.startLoading(), b.loadSources([]*provider.Provider{item.internal.(*provider.Provider)}), b.waitForSourcesLoaded())
		}
	}

	b.sourcesC, cmd = b.sourcesC.Update(msg)
	return b, cmd
}

func (b *statefulBubble) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, b.keymap.confirm) && (b.inputC.Value() != "" || b.suggestionIdx >= 0):
			if b.suggestionIdx >= 0 && b.suggestionIdx < len(b.searchSuggestions) {
				b.inputC.SetValue(b.searchSuggestions[b.suggestionIdx])
				b.inputC.SetCursor(len(b.inputC.Value()))
			}
			b.clearSuggestions()
			b.liveSeq++ // drop in-flight live lookups
			b.liveMode = false
			b.startLoading()
			b.newState(loadingState)
			go query.Remember(b.inputC.Value(), 1)
			return b, tea.Batch(b.searchManga(b.inputC.Value()), b.waitForMangas(), b.spinnerC.Tick)
		case key.Matches(msg, b.keymap.sourceSettings):
			b.openSourceSettings()
			return b, nil
		case (msg.String() == "ctrl+j" || msg.String() == "ctrl+k") && len(b.mangasC.Items()) > 0:
			// Dive into the live manga results; esc comes back here.
			b.clearSuggestions()
			b.inputC.Blur()
			b.liveMode = true
			b.newState(mangasState)
			return b, nil
		case msg.String() == "down" || msg.String() == "ctrl+n":
			if n := len(b.searchSuggestions); n > 0 {
				b.suggestionIdx = (b.suggestionIdx+2)%(n+1) - 1
			}
			return b, nil
		case msg.String() == "up" || msg.String() == "ctrl+p":
			if n := len(b.searchSuggestions); n > 0 {
				b.suggestionIdx = (b.suggestionIdx+n+1)%(n+1) - 1
			}
			return b, nil
		case key.Matches(msg, b.keymap.acceptSearchSuggestion) && len(b.searchSuggestions) > 0:
			i := imax(b.suggestionIdx, 0)
			b.inputC.SetValue(b.searchSuggestions[i])
			b.inputC.SetCursor(len(b.inputC.Value()))
			b.refreshSuggestions()
			return b, nil
		}
	}

	switch msg := msg.(type) {
	case liveTickMsg:
		return b, b.runLive(msg)
	case liveResultMsg:
		return b, b.applyLive(msg)
	}

	prev := b.inputC.Value()
	b.inputC, cmd = b.inputC.Update(msg)

	if b.inputC.Value() != prev || (b.inputC.Value() == "" && len(b.searchSuggestions) > 0) {
		b.refreshSuggestions()
	}
	if b.inputC.Value() != prev {
		cmd = tea.Batch(cmd, b.scheduleLive())
	}

	return b, cmd
}

func (b *statefulBubble) updateMangas(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m := msg.(type) {
	case liveResultMsg:
		return b, b.applyLive(m)
	case liveTickMsg:
		return b, b.runLive(m)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case b.mangasC.FilterState() == list.Filtering:
			break
		case b.liveMode && (msg.String() == "ctrl+j" || msg.String() == "ctrl+k"):
			// Toggle back up to the search bar.
			b.liveMode = false
			b.inputC.Focus()
			b.previousState()
			return b, nil
		case key.Matches(msg, b.keymap.confirm):
			if b.mangasC.SelectedItem() == nil {
				break
			}

			m, _ := b.mangasC.SelectedItem().(*listItem).internal.(*source.Manga)
			b.selectedManga = m
			go query.Remember(m.Name, 2)
			return b, tea.Batch(b.getChapters(m), b.waitForChapters(), b.startLoading())
		}
	case []*source.Chapter:
		items := make([]list.Item, len(msg))

		if viper.GetBool(key2.TUIReverseChapters) {
			for i, c := range msg {
				items[len(msg)-i-1] = &listItem{internal: c}
			}
		} else {
			for i, c := range msg {
				items[i] = &listItem{internal: c}
			}
		}

		cmd = b.chaptersC.SetItems(items)
		b.clearChapterSelection()
		b.newState(chaptersState)
		b.stopLoading()
		cmd = tea.Batch(cmd, b.selectBookmark())

		return b, cmd
	}

	b.mangasC, cmd = b.mangasC.Update(msg)
	return b, cmd
}

func (b *statefulBubble) updateChapters(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case b.chaptersC.FilterState() == list.Filtering:
			break
		case key.Matches(msg, b.keymap.visual):
			if b.visualAnchor >= 0 {
				// Leave select mode, keeping the range selected.
				b.visualAnchor = -1
			} else {
				b.visualAnchor = b.chaptersC.Index()
				b.applyVisual()
			}
			return b, nil
		case key.Matches(msg, b.keymap.download):
			if len(b.selectedChapters) == 0 {
				it := b.chaptersC.SelectedItem()
				if it == nil {
					break
				}
				b.selectedChapters[it.(*listItem).internal.(*source.Chapter)] = struct{}{}
			}
			b.visualAnchor = -1
			b.newState(confirmState)
			return b, nil
		case key.Matches(msg, b.keymap.read):
			if b.chaptersC.SelectedItem() == nil {
				break
			}

			chapter := b.chaptersC.SelectedItem().(*listItem).internal.(*source.Chapter)
			b.newState(readState)
			return b, tea.Batch(b.readChapter(chapter), b.waitForChapterRead(), b.startLoading())
		case key.Matches(msg, b.keymap.confirm):
			if viper.GetBool(key2.TUIReadOnEnter) {
				if b.chaptersC.SelectedItem() == nil {
					break
				}

				chapter := b.chaptersC.SelectedItem().(*listItem).internal.(*source.Chapter)
				b.newState(readState)
				return b, tea.Batch(b.readChapter(chapter), b.waitForChapterRead(), b.startLoading())
			}
		}
	}

	b.chaptersC, cmd = b.chaptersC.Update(msg)
	b.applyVisual()
	return b, cmd
}

// applyVisual marks every chapter between the select-mode anchor and the
// cursor, yazi style. No-op outside select mode.
func (b *statefulBubble) applyVisual() {
	if b.state != chaptersState || b.visualAnchor < 0 {
		return
	}
	lo_, hi := b.visualAnchor, b.chaptersC.Index()
	if lo_ > hi {
		lo_, hi = hi, lo_
	}
	b.selectedChapters = make(map[*source.Chapter]struct{})
	for i, it := range b.chaptersC.Items() {
		li := it.(*listItem)
		li.marked = i >= lo_ && i <= hi
		if li.marked {
			b.selectedChapters[li.internal.(*source.Chapter)] = struct{}{}
		}
	}
}

// clearChapterSelection leaves select mode and unmarks all chapters.
func (b *statefulBubble) clearChapterSelection() {
	b.visualAnchor = -1
	b.selectedChapters = make(map[*source.Chapter]struct{})
	for _, it := range b.chaptersC.Items() {
		it.(*listItem).marked = false
	}
}

func (b *statefulBubble) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, b.keymap.quit):
			return b, tea.Quit
		case key.Matches(msg, b.keymap.confirm):
			chapters := lo.Keys(b.selectedChapters)
			slices.SortFunc(chapters, func(a, b *source.Chapter) bool {
				return a.Index > b.Index
			})

			for _, chapter := range chapters {
				b.chaptersToDownload.Push(chapter)
			}
			b.newState(downloadState)
			return b, tea.Batch(b.startLoading(), b.downloadChapter(b.chaptersToDownload.Pop()), b.waitForChapterDownload(), b.progressC.SetPercent(0))
		case key.Matches(msg, b.keymap.back):
			b.previousState()
		}
	}

	return b, cmd
}

func (b *statefulBubble) updateRead(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case readyToReadMsg:
		downloader.SaveReadHistory(msg.chapter)
		opt, nb := downloader.ReaderOptionsFor(msg.chapter)
		rc := reader.NewCommand(msg.src, opt)
		return b, tea.Exec(
			&mutedExec{ExecCommand: rc, out: b.thumbs.out},
			func(err error) tea.Msg {
				last := msg.chapter
				if nb != nil {
					last = nb.Current
				}
				if err == nil {
					downloader.SaveReadProgress(last, rc.Result)
				}
				return readFinishedMsg{err: err, chapter: last}
			},
		)
	case readFinishedMsg:
		// The reader owned the terminal (and its own alt screen), so the
		// terminal no longer holds our thumbnails. Forget them so the next
		// frames re-transmit from the disk cache.
		b.thumbs.Clear()
		b.preview.Clear()
		b.stopLoading()
		b.previousState()
		if msg.err != nil {
			b.raiseError(msg.err)
		}
		// Crossing chapters in the reader moves the list cursor along.
		if last := msg.chapter; last != nil && b.state == chaptersState {
			if saved, ok := history.Lookup(b.selectedManga); ok {
				b.markBookmark(saved)
			}
			return b, b.selectChapterBy(func(c *source.Chapter) bool { return c == last })
		}
		return b, nil
	}

	b.spinnerC, cmd = b.spinnerC.Update(msg)
	return b, cmd
}

func (b *statefulBubble) updateDownload(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case struct{}:
		inc := 1 / float64(len(b.selectedChapters))

		if b.chaptersToDownload.Len() == 0 {
			// a little hack to make the progress render to the end
			go func() {
				time.Sleep(time.Millisecond * 400)
				b.newState(downloadDoneState)
			}()

			return b, b.progressC.IncrPercent(inc)
		}

		return b, tea.Batch(b.progressC.IncrPercent(inc), b.downloadChapter(b.chaptersToDownload.Pop()), b.waitForChapterDownload())
	case progress.FrameMsg:
		model, cmd := b.progressC.Update(msg)
		b.progressC = model.(progress.Model)

		return b, cmd
	}

	b.spinnerC, cmd = b.spinnerC.Update(msg)
	return b, cmd
}

func (b *statefulBubble) updateDownloadDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, b.keymap.quit):
			return b, tea.Quit
		case key.Matches(msg, b.keymap.redownloadFailed):
			if len(b.failedChapters) == 0 {
				break
			}

			b.chaptersToDownload = util.Stack[*source.Chapter]{}
			for _, chapter := range b.failedChapters {
				b.chaptersToDownload.Push(chapter)
			}
			b.failedChapters = make([]*source.Chapter, 0)
			b.succededChapters = make([]*source.Chapter, 0)
			b.newState(downloadState)
			return b, tea.Batch(b.startLoading(), b.downloadChapter(b.chaptersToDownload.Pop()), b.waitForChapterDownload(), b.progressC.SetPercent(0))
		}
	}

	return b, cmd
}

func (b *statefulBubble) updateError(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, b.keymap.quit):
			return b, tea.Quit
		}
	}

	return b, cmd
}

// maxSearchSuggestions caps the dropdown under the search bar.
const maxSearchSuggestions = 6

// refreshSuggestions recomputes the dropdown for the current input.
// The exact typed text is left out since it's already in the bar.
func (b *statefulBubble) refreshSuggestions() {
	b.suggestionIdx = -1
	b.searchSuggestions = nil

	value := b.inputC.Value()
	if strings.TrimSpace(value) == "" {
		return
	}

	typed := strings.ToLower(strings.TrimSpace(value))
	for _, s := range query.SuggestMany(value) {
		if s == typed {
			continue
		}
		b.searchSuggestions = append(b.searchSuggestions, s)
		if len(b.searchSuggestions) == maxSearchSuggestions {
			break
		}
	}

	// Default the highlight to the top suggestion.
	if len(b.searchSuggestions) > 0 {
		b.suggestionIdx = 0
	}
}

func (b *statefulBubble) clearSuggestions() {
	b.searchSuggestions = nil
	b.suggestionIdx = -1
}
