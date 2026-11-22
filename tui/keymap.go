package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/hongnoul/mangap/color"
	"github.com/hongnoul/mangap/style"
)

type statefulKeymap struct {
	state state

	quit, forceQuit,
	acceptSearchSuggestion,
	sourcePicker,
	remove,
	redownloadFailed,
	confirm,
	visual, download,
	top, bottom,
	read,
	back,
	up, down, left, right,
	showHelp,
	vimHL key.Binding
}

func (k *statefulKeymap) setState(newState state) {
	k.state = newState
}

func newStatefulKeymap() *statefulKeymap {
	k := key.NewBinding
	keys := key.WithKeys
	help := key.WithHelp

	return &statefulKeymap{
		quit: k(
			keys("q"),
			help("q", "quit"),
		),
		forceQuit: k(
			keys("ctrl+c"),
			help("ctrl+c", "quit"),
		),
		remove: k(
			keys("d"),
			help("d", "remove"),
		),
		visual: k(
			keys("v"),
			help("v", "select mode"),
		),
		download: k(
			keys("d"),
			help("d", "download"),
		),
		top: k(
			keys("g"),
			help("g", "top"),
		),
		bottom: k(
			keys("G"),
			help("G", "bottom"),
		),
		confirm: k(
			keys("enter"),
			help("enter", "confirm"),
		),
		read: k(
			keys("r"),
			help(style.Fg(color.Orange)("r"), style.Fg(color.Orange)("read")),
		),
		sourcePicker: k(
			keys("ctrl+s"),
			help("ctrl+s", "pick source"),
		),
		acceptSearchSuggestion: k(
			keys("tab"),
			help("tab", "accept search suggestion"),
		),
		redownloadFailed: k(
			keys("r"),
			help("r", "redownload failed"),
		),
		back: k(
			keys("esc"),
			help("esc", "back"),
		),
		up: k(
			keys("up", "k"),
			help("↑", "up"),
		),
		down: k(
			keys("down", "j"),
			help("↓", "down"),
		),
		left: k(
			keys("left", "ctrl+k"),
			help("←", "prev page"),
		),
		right: k(
			keys("right", "ctrl+j"),
			help("→", "next page"),
		),
		vimHL: k(
			keys(),
			help("h/l", "back/open"),
		),
		showHelp: k(
			keys("?"),
			help("?", "help"),
		),
	}
}

// help returns short and full help for the state
func (k *statefulKeymap) help() ([]key.Binding, []key.Binding) {
	h := func(bindings ...key.Binding) []key.Binding {
		return bindings
	}

	to2 := func(a []key.Binding) ([]key.Binding, []key.Binding) {
		return a, a
	}

	switch k.state {
	case scrapersInstallState:
		install := withDescription(k.confirm, "install")
		return to2(h(install))
	case loadingState:
		return to2(h(k.forceQuit, k.back))
	case historyState:
		return to2(h(k.confirm, k.remove, k.back))
	case sourcesState:
		search := withDescription(k.confirm, "search")
		return to2(h(search))
	case searchState:
		return to2(h(k.confirm, k.acceptSearchSuggestion, k.sourcePicker, k.forceQuit))
	case mangasState:
		return to2(h(k.confirm, k.back, k.vimHL))
	case chaptersState:
		return h(k.read, k.visual, k.download, k.back), h(k.read, k.visual, k.download, k.top, k.bottom, k.back)
	case confirmState:
		return to2(h(k.confirm, k.back, k.quit))
	case readState:
		return to2(h(k.back, k.forceQuit))
	case downloadState:
		return to2(h(k.back, k.forceQuit))
	case downloadDoneState:
		return to2(h(k.back, k.quit, k.redownloadFailed))
	case errorState:
		return to2(h(k.back, k.quit))
	default:
		return to2(h())
	}
}

func (k *statefulKeymap) ShortHelp() []key.Binding {
	short, _ := k.help()
	return short
}

func (k *statefulKeymap) FullHelp() [][]key.Binding {
	_, full := k.help()
	return [][]key.Binding{full}
}

func (k *statefulKeymap) forList() list.KeyMap {
	return list.KeyMap{
		CursorUp:             k.up,
		CursorDown:           k.down,
		NextPage:             k.right,
		PrevPage:             k.left,
		GoToStart:            k.top,
		GoToEnd:              k.bottom,
		Filter:               key.NewBinding(key.WithDisabled()),
		ClearFilter:          k.back,
		CancelWhileFiltering: k.back,
		AcceptWhileFiltering: k.confirm,
		ShowFullHelp:         k.showHelp,
		CloseFullHelp:        k.showHelp,
		Quit:                 k.quit,
		ForceQuit:            k.forceQuit,
	}
}

func withDescription(k key.Binding, description string) key.Binding {
	return key.NewBinding(
		key.WithKeys(k.Keys()...),
		key.WithHelp(k.Help().Key, description),
	)
}
