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
func (b *statefulBubble) sourcePicker() string {
	var parts []string
	for i, n := range b.sourceChoices {
		if strings.EqualFold(n, "mangapill") {
			n += " (Recommended)"
		}
		box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
		if i == b.sourceIdx {
			box = box.Bold(true).BorderForeground(lipgloss.Color("5")).Foreground(lipgloss.Color("5"))
		} else {
			box = box.Faint(true).BorderForeground(lipgloss.Color("8"))
		}
		parts = append(parts, box.Render(n), " ")
	}
	hint := lipgloss.NewStyle().PaddingTop(1).Render(style.Faint(" (ctrl+o / ← →)"))
	parts = append(parts, hint)
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
