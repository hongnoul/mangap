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
	n := b.sourceChoices[b.sourceIdx] // "(Recommended)" only shows in the ctrl+s picker
	bold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	lang := languages[languageIndex(viper.GetString(key2.MangadexLanguage))]
	srcLabel := style.Faint("Source    ") + bold.Render(n)
	if bd := b.pingC.badge(b.sourceChoices[b.sourceIdx]); bd != "" {
		srcLabel += "  " + bd
	}
	inner := imax(w-4, 4)
	row := func(label, hint string) string {
		gap := imax(inner-lipgloss.Width(label)-lipgloss.Width(hint), 1)
		return label + strings.Repeat(" ", gap) + style.Faint(hint)
	}
	langRow := row(style.Faint("Language  ")+bold.Render(lang.name), "ctrl+l")
	if b.languageLocked() {
		langRow = row(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Language  English"), "ctrl+l")
	}
	lines := []string{
		row(srcLabel, "ctrl+s"),
		langRow,
	}
	word := "show"
	if b.showKeys {
		word = "hide"
	}
	lines = append(lines, row(style.Faint("Keybinds  ")+bold.Render(word), "ctrl+g"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(0, 1).
		Width(imax(w-2, 4)).
		Render(strings.Join(lines, "\n"))
}
