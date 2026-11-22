package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
	"github.com/spf13/viper"
)

// sourceChoices lists the home screen picker entries: every provider in
// the order set in source settings (ctrl+s), disabled ones left out.
func sourceChoices() []string {
	var names []string
	for _, p := range provider.Ordered() {
		names = append(names, p.Name)
	}
	return names
}

// defaultSourceIdx picks Mangapill, falling back to the first entry.
func defaultSourceIdx(choices []string) int {
	for i, n := range choices {
		if strings.EqualFold(n, "mangapill") {
			return i
		}
	}
	return 0
}

// pickSource makes choices[i] the active search source.
func (b *statefulBubble) pickSource(i int) error {
	if len(b.sourceChoices) == 0 {
		return nil
	}
	n := len(b.sourceChoices)
	i = ((i % n) + n) % n
	name := b.sourceChoices[i]

	p, ok := provider.Get(name)
	if !ok {
		return nil
	}
	src, err := p.CreateSource()
	if err != nil {
		return err
	}
	b.sourceIdx = i
	b.selectedSources = []source.Source{src}
	return nil
}

// sourcePicker renders the source row under the search bar.
func (b *statefulBubble) sourcePicker(w int) string {
	if b.sourceIdx >= len(b.sourceChoices) {
		return ""
	}
	n := b.sourceChoices[b.sourceIdx]
	if strings.EqualFold(n, "mangapill") {
		n += " (Recommended)"
	}
	bold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	lang := languages[languageIndex(viper.GetString(key2.MangadexLanguage))]
	box := func(width int, label string) string {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("5")).
			Width(imax(width-2, 4)).
			Align(lipgloss.Center).
			Render(label)
	}
	leftW := (w - 1) / 2
	rightW := w - 1 - leftW
	srcLabel := style.Faint("Source  ") + bold.Render(n)
	if bd := b.pingC.badge(b.sourceChoices[b.sourceIdx]); bd != "" {
		srcLabel += "  " + bd
	}
	src := box(leftW, srcLabel+style.Faint(" (ctrl+s)"))
	lng := box(rightW, style.Faint("Language  ")+bold.Render(lang.name)+style.Faint(" (ctrl+l)"))
	row := lipgloss.JoinHorizontal(lipgloss.Top, src, " ", lng)
	if b.dropdown == nil {
		return row
	}
	// Expanded list sits under the box it belongs to.
	if b.dropdown.kind == sourceDropdown {
		return lipgloss.JoinVertical(lipgloss.Left, row, b.viewDropdown(leftW))
	}
	return lipgloss.JoinVertical(lipgloss.Left, row,
		lipgloss.NewStyle().PaddingLeft(leftW+1).Render(b.viewDropdown(rightW)))
}
