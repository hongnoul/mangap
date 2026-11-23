package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
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
)

func (b *statefulBubble) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// The downloads viewer: / types into its filter box, otherwise
	// hjkl and the grid keys browse.
	if km, ok := msg.(tea.KeyMsg); ok && b.del.on {
		return b, b.updateArchiveDelete(km)
	}
	if km, ok := msg.(tea.KeyMsg); ok && b.inArchive() && !isArchiveKey(km) {
		if c, handled := b.updateArchiveKeys(km); handled {
			return b, c
		}
	}
	msg = b.vimHL(msg)
	if m, ok := msg.(dlFinishedMsg); ok {
		b.dlDone++
		invalidateDownloaded()
		next := b.nextDownload()
		if m.err != nil && viper.GetBool(key2.DownloaderStopOnError) {
			b.chaptersToDownload.Clear()
			b.dlCurrent = nil
			b.dlDone, b.dlTotal = 0, 0
			b.raiseError(m.err)
			return b, nil
		}
		return b, next
	}
	if t, ok := msg.(spinner.TickMsg); ok && b.dlCurrent != nil {
		var c tea.Cmd
		b.spinnerC, c = b.spinnerC.Update(t)
		return b, c
	}
	if _, ok := msg.(thumbReadyMsg); ok {
		return b, nil // a cover landed: the repaint after Update shows it
	}
	if t, ok := msg.(loaderTickMsg); ok {
		if b.pingC.bar.owns(t) {
			return b, b.pingC.bar.update(t)
		}
		switch b.state {
		case loadingState, readState, downloadState:
			return b, b.loaderC.update(t)
		}
		b.loaderC.stop() // left the loading screens: stop repainting
		return b, nil
	}
	if m, ok := msg.(pingResultMsg); ok {
		b.pingC.update(m)
		return b, nil
	}
	// Live search messages are handled in any state: late results still
	// fill the caches and free their in-flight slot.
	switch m := msg.(type) {
	case liveTickMsg:
		return b, b.runLive(m)
	case liveResultMsg:
		return b, b.applyLive(m)
	case livePrefetchMsg:
		return b, b.runPrefetch(m)
	case liveWarmMsg:
		return b, b.handleWarm()
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		if l := b.gridList(); l != nil && l.FilterState() != list.Filtering {
			if b.grid.key(km.String()) || b.grid.nav(l, km.String()) {
				b.applyVisual()
				return b, nil
			}
		}
	}

	if chs, ok := msg.([]*source.Chapter); ok && b.wholePending {
		b.wholePending = false
		b.stopLoading()
		if len(chs) == 0 {
			return b, nil
		}
		b.visualAnchor = -1
		b.selectedChapters = make(map[*source.Chapter]struct{})
		for _, c := range chs {
			b.selectedChapters[c] = struct{}{}
		}
		b.confirmAll = true
		b.newState(confirmState)
		return b, nil
	}
	if _, ok := msg.(error); ok {
		b.wholePending = false
	}

	switch msg := msg.(type) {
	case error:
		b.raiseError(msg)
	case tea.WindowSizeMsg:
		b.resize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch {
		case msg.String() == "ctrl+d" && b.dropdown == nil && !b.inArchive() &&
			(b.state == searchState || (b.state == mangasState && b.mangasC.FilterState() != list.Filtering)):
			// ctrl+d on a search result: fetch its chapters, then confirm
			// downloading all of them.
			it := b.mangasC.SelectedItem()
			if it == nil {
				return b, nil
			}
			m, ok := it.(*listItem).internal.(*source.Manga)
			if !ok {
				return b, nil
			}
			b.selectedManga = m
			b.wholePending = true
			return b, tea.Batch(b.getChapters(m), b.waitForChapters(), b.startLoading(), b.spinnerC.Tick)
		case msg.String() == "ctrl+d" && b.state == chaptersState && b.chaptersC.FilterState() != list.Filtering && !b.inArchiveChapters():
			// ctrl+d on a comic's chapters downloads the whole comic.
			if len(b.chaptersC.Items()) == 0 {
				return b, nil
			}
			b.visualAnchor = -1
			b.selectedChapters = make(map[*source.Chapter]struct{})
			for _, it := range b.chaptersC.Items() {
				b.selectedChapters[it.(*listItem).internal.(*source.Chapter)] = struct{}{}
			}
			b.confirmAll = true
			b.newState(confirmState)
			return b, nil
		case isArchiveKey(msg) && b.canOpenArchive():
			return b, b.openArchive()
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
				if b.dropdown != nil {
					b.dropdown = nil
					return b, nil
				}
				if b.showKeys {
					b.showKeys = false
					return b, nil
				}
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
				if b.mangasC.FilterState() != list.Unfiltered {
					b.mangasC, cmd = b.mangasC.Update(msg)
					return b, cmd
				}

				if b.inArchive() {
					// Back to search: the live results were replaced, redo them.
					b.arch.on = false
					b.mangasC.SetItems(nil)
					b.thumbs.Clear()
					b.preview.Clear()
					b.recent.Clear()
					b.previousState()
					return b, b.scheduleLive()
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
			case confirmState:
				if b.confirmAll {
					b.confirmAll = false
					b.clearChapterSelection()
				}
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
		b.arch.on = false
		b.thumbs.Clear()
		b.preview.Clear()
		b.recent.Clear()
		b.newState(mangasState)
		b.stopLoading()
	case []*source.Chapter:
		if b.statesHistory.Peek() == historyState {
			b.newState(historyState)
			b.stopLoading()
			cmds = append(cmds, func() tea.Msg {
				return msg
			})
		} else {
			cmds = append(cmds, b.showChapters(msg))
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
		if b.dropdown != nil {
			return b.updateDropdown(msg)
		}
		switch {
		case key.Matches(msg, b.keymap.confirm) && len(b.mangasC.Items()) > 0:
			// Open the highlighted live result.
			m, ok := b.mangasC.SelectedItem().(*listItem).internal.(*source.Manga)
			if !ok {
				break
			}
			b.liveSeq++
			b.cancelAll()
			b.clearSuggestions()
			b.selectedManga = m
			go liveCatalog().Opened(m)
			go query.Remember(b.inputC.Value(), 1)
			go query.Remember(m.Name, 2)
			b.newState(loadingState)
			b.chaptersPending = true
			return b, tea.Batch(b.getChapters(m), b.waitForChapters(), b.startLoading(), b.spinnerC.Tick)
		case key.Matches(msg, b.keymap.confirm) && (b.inputC.Value() != "" || b.suggestionIdx >= 0):
			if b.suggestionIdx >= 0 && b.suggestionIdx < len(b.searchSuggestions) {
				b.inputC.SetValue(b.searchSuggestions[b.suggestionIdx])
				b.inputC.SetCursor(len(b.inputC.Value()))
			}
			b.clearSuggestions()
			b.liveSeq++ // drop in-flight live lookups
			b.startLoading()
			b.newState(loadingState)
			go query.Remember(b.inputC.Value(), 1)
			return b, tea.Batch(b.searchManga(b.inputC.Value()), b.waitForMangas(), b.spinnerC.Tick)
		case key.Matches(msg, b.keymap.sourcePicker):
			b.openDropdown(sourceDropdown)
			return b, nil
		case msg.String() == "ctrl+l":
			b.openDropdown(languageDropdown)
			return b, nil
		case isKeysToggle(msg):
			b.showKeys = !b.showKeys
			return b, nil
		case msg.String() == "ctrl+r":
			if err := b.toggleViewer(); err != nil {
				b.raiseError(err)
			}
			return b, nil
		case msg.String() == "ctrl+t":
			if err := b.toggleThumbnails(); err != nil {
				b.raiseError(err)
			}
			return b, nil
		case b.showKeys && msg.String() == "esc":
			b.showKeys = false
			return b, nil
		case len(b.sourceChoices) > 0 && (msg.String() == "ctrl+o" ||
			(b.inputC.Value() == "" && (msg.String() == "left" || msg.String() == "right"))):
			d := 1
			if msg.String() == "left" {
				d = -1
			}
			if err := b.pickSource(b.sourceIdx + d); err != nil {
				b.raiseError(err)
				return b, nil
			}
			return b, tea.Batch(b.scheduleLive(), b.warmSources())
		case len(b.mangasC.Items()) > 0 && lo.Contains([]string{"ctrl+k", "tab", "shift+tab"}, msg.String()):
			// Walk the live results while typing stays in the bar.
			n, i := len(b.mangasC.Items()), b.mangasC.Index()
			if s := msg.String(); s == "tab" {
				i = (i + 1) % n
			} else {
				i = (i - 1 + n) % n
			}
			b.mangasC.Select(i)
			b.live.picked = true
			return b, b.schedulePrefetch()
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case b.mangasC.FilterState() == list.Filtering:
			break
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
		return b, b.showChapters(msg)
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
			if b.inArchiveChapters() {
				b.askDeleteChapters() // d in the archive deletes from disk
				return b, nil
			}
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
			return b, tea.Batch(b.readChapter(chapter), b.waitForChapterRead(), b.startLoading(), b.spinnerC.Tick)
		case key.Matches(msg, b.keymap.confirm):
			if viper.GetBool(key2.TUIReadOnEnter) {
				if b.chaptersC.SelectedItem() == nil {
					break
				}

				chapter := b.chaptersC.SelectedItem().(*listItem).internal.(*source.Chapter)
				b.newState(readState)
				return b, tea.Batch(b.readChapter(chapter), b.waitForChapterRead(), b.startLoading(), b.spinnerC.Tick)
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
			b.confirmAll = false
			chapters := lo.Keys(b.selectedChapters)
			util.SortFunc(chapters, func(a, b *source.Chapter) bool {
				return a.Index < b.Index
			})
			// Downloads run in the background: back to where we were.
			b.previousState()
			if b.state == chaptersState {
				b.clearChapterSelection()
			} else {
				b.selectedChapters = make(map[*source.Chapter]struct{})
			}
			return b, b.enqueueDownloads(chapters)
		case key.Matches(msg, b.keymap.back):
			if b.confirmAll {
				b.confirmAll = false
				b.clearChapterSelection()
			}
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
		if !useTerminalViewer() {
			// External viewer: hand off a PDF and stay in the TUI.
			name := msg.chapter.Name
			if msg.chapter.Manga != nil {
				name = msg.chapter.Manga.Name + " - " + name
			}
			src := msg.src
			return b, func() tea.Msg {
				defer src.Close()
				_, err := reader.OpenExternal(src, name)
				return readFinishedMsg{err: err, chapter: msg.chapter}
			}
		}
		opt, nb := downloader.ReaderOptionsFor(msg.chapter)
		rc := reader.NewCommand(msg.src, opt)
		// Mute now, not in mutedExec.Run: tea.Exec's ReleaseTerminal leaves
		// the alt screen before Run starts, and a renderer flush in that
		// window (the loader keeps ticking) paints this Reading frame onto
		// the main screen, where it survives quitting the TUI.
		b.thumbs.out.setMuted(true)
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
		b.recent.Clear()
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

		invalidateDownloaded()
		if b.chaptersToDownload.Len() == 0 {
			// Done: no summary page, the Downloaded box shows the result.
			b.stopLoading()
			b.currentDownloadingChapter = nil
			b.failedChapters = make([]*source.Chapter, 0)
			b.succededChapters = make([]*source.Chapter, 0)
			b.previousState()
			if b.state == chaptersState {
				b.clearChapterSelection()
			} else {
				b.selectedChapters = make(map[*source.Chapter]struct{})
			}
			return b, nil
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
			return b, tea.Batch(b.startLoading(), b.downloadChapter(b.chaptersToDownload.Pop()), b.waitForChapterDownload(), b.progressC.SetPercent(0), b.spinnerC.Tick)
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
		case key.Matches(msg, b.keymap.confirm):
			b.stopLoading()
			b.previousState()
		}
	}

	return b, cmd
}

// maxSearchSuggestions caps the dropdown under the search bar.
const maxSearchSuggestions = 6

// refreshSuggestions clears suggestion state. The text dropdown was
// removed since the live thumbnail results already show the matches.
func (b *statefulBubble) refreshSuggestions() {
	b.suggestionIdx = -1
	b.searchSuggestions = nil
}

func (b *statefulBubble) clearSuggestions() {
	b.searchSuggestions = nil
	b.suggestionIdx = -1
}

// showChapters fills the chapter list and switches to it.
func (b *statefulBubble) showChapters(chapters []*source.Chapter) tea.Cmd {
	items := make([]list.Item, len(chapters))
	reverse := viper.GetBool(key2.TUIReverseChapters)
	for i, c := range chapters {
		if reverse {
			items[len(chapters)-i-1] = &listItem{internal: c}
		} else {
			items[i] = &listItem{internal: c}
		}
	}

	cmd := b.chaptersC.SetItems(items)
	b.clearChapterSelection()
	b.newState(chaptersState)
	b.stopLoading()
	return tea.Batch(cmd, b.selectBookmark())
}
