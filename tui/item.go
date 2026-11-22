package tui

import (
	"fmt"
	"github.com/hongnoul/mangap/history"
	"github.com/hongnoul/mangap/icon"
	"github.com/hongnoul/mangap/installer"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
	"github.com/spf13/viper"
	"strings"
)

type listItem struct {
	internal interface{}
	marked   bool
	// bookmark is a "p.13/38" hint on the chapter the user left off in.
	bookmark string
}

func (t *listItem) toggleMark() {
	t.marked = !t.marked
}

func (t *listItem) getMark() string {
	switch t.internal.(type) {
	case *source.Chapter:
		return "" // shown as a yellow row instead
	case *provider.Provider:
		return icon.Get(icon.Search)
	default:
		return ""
	}
}

func (t *listItem) Title() (title string) {
	switch e := t.internal.(type) {
	case *source.Chapter:
		var sb = strings.Builder{}

		sb.WriteString(t.FilterValue())
		if e.Volume != "" {
			sb.WriteString(" ")
			sb.WriteString(style.Faint(e.Volume))
		}

		if !source.IsLocal(e) && e.IsDownloaded() {
			sb.WriteString(" ")
			sb.WriteString(icon.Get(icon.Downloaded))
		}

		if t.bookmark != "" {
			sb.WriteString(" ")
			sb.WriteString("(last read: " + t.bookmark + ")")
		}

		title = sb.String()
	default:
		title = t.FilterValue()
	}

	if mark := t.getMark(); title != "" && t.marked && mark != "" {
		//title = fmt.Sprintf("%s %s", title, icon.Get(icon.Mark))
		title = fmt.Sprintf("%s %s", title, t.getMark())
	}

	return
}

func (t *listItem) Description() (description string) {
	switch e := t.internal.(type) {
	case *source.Chapter:
		description = urlOrEmpty(e.URL)
	case *source.Manga:
		description = urlOrEmpty(e.URL)
	case *installer.Scraper:
		description = urlOrEmpty(e.GithubURL())
	case *history.SavedChapter:
		description = fmt.Sprintf("%s : %d / %d", e.Name, e.Index, e.MangaChaptersTotal)
	case *provider.Provider:
		sb := strings.Builder{}
		if e.IsCustom {
			sb.WriteString("Custom")
		} else {
			sb.WriteString("Builtin")
		}

		if e.UsesHeadless {
			sb.WriteString(", uses headless chrome")
		}

		description = sb.String()
	}

	return
}

func (t *listItem) FilterValue() string {
	switch e := t.internal.(type) {
	case *source.Chapter:
		return e.Name
	case *source.Manga:
		return e.Name
	case *history.SavedChapter:
		return e.MangaName
	case *provider.Provider:
		return e.Name
	case *installer.Scraper:
		return e.Name
	default:
		return ""
	}
}

// urlOrEmpty hides URLs unless tui.show_urls is set.
func urlOrEmpty(u string) string {
	if viper.GetBool(key.TUIShowURLs) {
		return u
	}
	return ""
}
