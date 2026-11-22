package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/history"
	"github.com/hongnoul/mangap/installer"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/util"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/spf13/viper"
	"strings"
	"time"
)

type statefulBubble struct {
	state         state
	statesHistory util.Stack[state]
	loading       bool

	keymap *statefulKeymap

	// components
	spinnerC         spinner.Model
	loaderC          loader
	pingC            pinger
	inputC           textinput.Model
	scrapersInstallC list.Model
	historyC         list.Model
	sourcesC         list.Model
	mangasC          list.Model
	chaptersC        list.Model
	progressC        progress.Model
	helpC            help.Model

	selectedProviders map[*provider.Provider]struct{}
	selectedSources   []source.Source
	selectedManga     *source.Manga
	selectedChapters  map[*source.Chapter]struct{} // mathematical set
	// visualAnchor is the chapter index where select mode (v) started, -1 when off.
	visualAnchor int

	scrapersLoadedChannel   chan []*installer.Scraper
	scraperInstalledChannel chan *installer.Scraper
	sourcesLoadedChannel    chan []source.Source
	foundMangasChannel      chan []*source.Manga
	foundChaptersChannel    chan []*source.Chapter
	chapterReadChannel      chan readyToReadMsg
	chapterDownloadChannel  chan struct{}
	errorChannel            chan error

	progressStatus string

	chaptersToDownload util.Stack[*source.Chapter]

	currentDownloadingChapter *source.Chapter
	lastError                 error

	width, height int
	errorPlot     string

	failedChapters   []*source.Chapter
	succededChapters []*source.Chapter

	// search bar dropdown: matching past queries, and the highlighted
	// row (-1 means none, the typed text is used as is)
	searchSuggestions []string
	suggestionIdx     int

	sourceSettings *sourceSettings

	// home screen source picker (see source_picker.go)
	sourceChoices []string
	sourceIdx     int
	dropdown      *dropdown // open source/language picker, nil when closed
	showKeys      bool      // home keybind cheatsheet expanded (ctrl+/)

	// live search (see live.go)
	liveSeq int

	thumbs  *thumbs
	preview *thumbs // large image of the selected item, right pane
	grid    *grid
}

func (b *statefulBubble) raiseError(err error) {
	b.lastError = err
	b.errorPlot = randomPlot()
	b.newState(errorState)
}

func (b *statefulBubble) setState(s state) {
	b.state = s
	b.keymap.setState(s)
}

func (b *statefulBubble) newState(s state) {
	// do not push state if it is the same as the current state
	if b.state == s {
		return
	}

	// Transitioning to these states is not allowed (it makes no sense)
	if !lo.Contains([]state{
		loadingState,
		readState,
		downloadDoneState,
		downloadState,
		confirmState,
	}, b.state) {
		b.statesHistory.Push(b.state)
	}

	b.setState(s)
}

func (b *statefulBubble) previousState() {
	if b.statesHistory.Len() > 0 {
		s := b.statesHistory.Pop()
		if s != 0 {
			b.setState(s)
		}
	}
}

func (b *statefulBubble) resize(width, height int) {
	x, y := paddingStyle.GetFrameSize()
	xx, yy := listExtraPaddingStyle.GetFrameSize()

	styledWidth := width - x
	styledHeight := height - y

	listWidth := width - xx
	listHeight := height - yy

	b.scrapersInstallC.SetSize(listWidth, listHeight)
	b.scrapersInstallC.Help.Width = listWidth

	b.historyC.SetSize(listWidth, listHeight)
	b.historyC.Help.Width = listWidth

	b.sourcesC.SetSize(listWidth, listHeight)
	b.sourcesC.Help.Width = listWidth

	b.mangasC.SetSize(listWidth, listHeight)
	b.mangasC.Help.Width = listWidth

	b.chaptersC.SetSize(listWidth, listHeight)
	b.chaptersC.Help.Width = listWidth

	b.progressC.Width = listWidth

	b.width = styledWidth
	b.height = styledHeight
	b.helpC.Width = listWidth
}

func (b *statefulBubble) startLoading() tea.Cmd {
	b.loading = true
	return tea.Batch(b.loaderC.start(), b.mangasC.StartSpinner(), b.chaptersC.StartSpinner())
}

func (b *statefulBubble) stopLoading() tea.Cmd {
	b.loading = false
	b.loaderC.stop()
	b.mangasC.StopSpinner()
	b.chaptersC.StopSpinner()
	return nil
}

func newBubble() *statefulBubble {
	keymap := newStatefulKeymap()
	bubble := statefulBubble{
		statesHistory: util.Stack[state]{},
		keymap:        keymap,
		suggestionIdx: -1,

		scrapersLoadedChannel:   make(chan []*installer.Scraper),
		scraperInstalledChannel: make(chan *installer.Scraper),
		sourcesLoadedChannel:    make(chan []source.Source),
		foundMangasChannel:      make(chan []*source.Manga),
		foundChaptersChannel:    make(chan []*source.Chapter),
		chapterReadChannel:      make(chan readyToReadMsg),
		chapterDownloadChannel:  make(chan struct{}),
		errorChannel:            make(chan error),

		selectedProviders:  make(map[*provider.Provider]struct{}),
		selectedChapters:   make(map[*source.Chapter]struct{}),
		visualAnchor:       -1,
		chaptersToDownload: util.Stack[*source.Chapter]{},

		failedChapters:   make([]*source.Chapter, 0),
		succededChapters: make([]*source.Chapter, 0),
	}

	type listOptions struct {
		TitleStyle mo.Option[lipgloss.Style]
	}

	makeList := func(title string, description bool, options *listOptions) list.Model {
		delegate := list.NewDefaultDelegate()
		delegate.SetSpacing(viper.GetInt(key2.TUIItemSpacing))
		delegate.ShowDescription = description
		highlight := lipgloss.NewStyle().
			Reverse(true).
			Padding(0, 1)
		delegate.Styles.SelectedTitle = highlight.Copy().Bold(true)
		delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Copy().Padding(0, 1)
		delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Copy().Padding(0, 1)
		delegate.Styles.SelectedDesc = highlight.Copy()
		delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.Copy().Padding(0, 1)
		delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.Copy().Padding(0, 1)

		listC := list.New([]list.Item{}, delegate, 0, 0)
		listC.KeyMap = bubble.keymap.forList()
		listC.AdditionalShortHelpKeys = bubble.keymap.ShortHelp
		listC.AdditionalFullHelpKeys = func() []key.Binding {
			return bubble.keymap.FullHelp()[0]
		}
		listC.Title = title
		listC.Styles.NoItems = paddingStyle
		if titleStyle, ok := options.TitleStyle.Get(); ok {
			listC.Styles.Title = titleStyle
		}

		listC.SetShowPagination(false)
		listC.SetShowHelp(false)
		listC.SetShowStatusBar(false)
		listC.SetShowTitle(false)

		//listC.StatusMessageLifetime = time.Second * 5
		listC.StatusMessageLifetime = time.Hour * 999 // forever

		return listC
	}

	bubble.helpC = help.New()

	bubble.spinnerC = spinner.New()
	bubble.spinnerC.Spinner = spinner.Dot
	bubble.spinnerC.Style = lipgloss.NewStyle()
	bubble.loaderC = newLoader()
	bubble.pingC = newPinger()

	bubble.inputC = textinput.New()
	bubble.inputC.Placeholder = "Search"
	bubble.inputC.CharLimit = 60
	bubble.inputC.Prompt = ""

	bubble.progressC = progress.New(progress.WithDefaultGradient())

	showURLs := viper.GetBool(key2.TUIShowURLs)
	bubble.scrapersInstallC = makeList("Install Scrapers", showURLs, &listOptions{
		TitleStyle: mo.Some(
			lipgloss.NewStyle().Reverse(true).Bold(true).Padding(0, 1),
		),
	})
	bubble.scrapersInstallC.SetStatusBarItemName("scraper", "scrapers")

	bubble.historyC = makeList("History", true, &listOptions{})
	bubble.sourcesC.SetStatusBarItemName("chapter", "chapters")

	bubble.sourcesC = makeList("Select Source", true, &listOptions{
		TitleStyle: mo.Some(
			lipgloss.NewStyle().Reverse(true).Bold(true).Padding(0, 1),
		),
	})
	bubble.sourcesC.SetStatusBarItemName("source", "sources")

	bubble.mangasC = makeList("Mangas", showURLs, &listOptions{
		TitleStyle: mo.Some(
			lipgloss.NewStyle().Reverse(true).Bold(true).Padding(0, 1),
		),
	})
	bubble.mangasC.SetStatusBarItemName("manga", "mangas")
	// No "Mangas via X" title or "N mangas" count: just the results.
	bubble.mangasC.SetShowTitle(false)
	bubble.mangasC.SetShowStatusBar(false)
	bubble.thumbs = newThumbs(nil)
	bubble.thumbs.fill = true
	bubble.preview = newThumbs(nil)
	bubble.grid = newGrid()

	bubble.chaptersC = makeList("Chapters", showURLs, &listOptions{
		TitleStyle: mo.Some(
			lipgloss.NewStyle().Reverse(true).Bold(true).Padding(0, 1),
		),
	})
	bubble.chaptersC.SetStatusBarItemName("chapter", "chapters")

	if w, h, err := util.TerminalSize(); err == nil {
		bubble.resize(w, h)
	}

	bubble.inputC.Focus()

	return &bubble
}

func (b *statefulBubble) loadProviders() tea.Cmd {
	providers := provider.Builtins()
	customProviders := provider.Customs()

	var items []list.Item
	for _, p := range providers {
		items = append(items, &listItem{
			internal: p,
		})
	}
	util.SortFunc(items, func(a, b list.Item) bool {
		// temporary workaround for placing mangadex second because it is not stable for now
		// but, you know, there is nothing more permanent than a temporary solution
		return strings.Compare(a.FilterValue(), b.FilterValue()) > 0
	})

	var customItems []list.Item
	for _, p := range customProviders {
		customItems = append(customItems, &listItem{
			internal: p,
		})
	}
	util.SortFunc(customItems, func(a, b list.Item) bool {
		return strings.Compare(a.FilterValue(), b.FilterValue()) < 0
	})

	// built-in providers should come first
	return b.sourcesC.SetItems(append(items, customItems...))
}

func (b *statefulBubble) loadHistory() (tea.Cmd, error) {
	saved, err := history.Get()
	if err != nil {
		return nil, err
	}

	chapters := lo.Values(saved)
	util.SortFunc(chapters, func(a, b *history.SavedChapter) bool {
		if a.MangaName == b.MangaName {
			return a.Name < b.Name
		}
		return a.MangaName < b.MangaName
	})

	var items []list.Item
	for _, c := range chapters {
		items = append(items, &listItem{
			internal: c,
		})
	}

	return tea.Batch(b.historyC.SetItems(items), b.loadProviders()), nil
}
