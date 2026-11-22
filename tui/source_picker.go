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
	srcLabel := style.Faint("Source  ") + bold.Render(n)
	if bd := b.pingC.badge(b.sourceChoices[b.sourceIdx]); bd != "" {
		srcLabel += "  " + bd
	}
	parts := []string{box(w, srcLabel+style.Faint(" (ctrl+s)"))}
	if b.dropdown != nil && b.dropdown.kind == sourceDropdown {
		parts = append(parts, b.viewDropdown(w))
	}
	parts = append(parts, box(w, style.Faint("Language  ")+bold.Render(lang.name)+style.Faint(" (ctrl+l)")))
	if b.dropdown != nil && b.dropdown.kind == languageDropdown {
		parts = append(parts, b.viewDropdown(w))
	}
	word := "show"
	if b.showKeys {
		word = "hide"
	}
	parts = append(parts, box(w, style.Faint("Keybinds  ")+bold.Render(word)+style.Faint(" (ctrl+/)")))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
