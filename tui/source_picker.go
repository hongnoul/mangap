package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
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
	label := style.Faint("Source  ") + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Render(n)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Width(imax(w-2, 4)).
		Align(lipgloss.Center).
		Render(label)
}
