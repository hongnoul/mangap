package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
)

// autoChoice is the picker entry for the fallback chain.
const autoChoice = "Auto"

// sourceChoices lists the home screen picker entries: every provider in
// priority order, then Auto.
func sourceChoices() []string {
	var names []string
	for _, p := range provider.Builtins() {
		names = append(names, p.Name)
	}
	for _, p := range provider.Customs() {
		names = append(names, p.Name)
	}
	return append(names, autoChoice)
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

	var src source.Source
	if name == autoChoice {
		auto, err := provider.NewAuto()
		if err != nil {
			return err
		}
		src = auto
	} else {
		p, ok := provider.Get(name)
		if !ok {
			return nil
		}
		s, err := p.CreateSource()
		if err != nil {
			return err
		}
		src = s
	}
	b.sourceIdx = i
	b.selectedSources = []source.Source{src}
	return nil
}

// sourcePicker renders the source row under the search bar.
func (b *statefulBubble) sourcePicker() string {
	parts := []string{style.Faint("source:")}
	for i, n := range b.sourceChoices {
		if i == b.sourceIdx {
			parts = append(parts, lipgloss.NewStyle().Reverse(true).Bold(true).Render(" "+n+" "))
		} else {
			parts = append(parts, style.Faint(" "+n+" "))
		}
	}
	parts = append(parts, style.Faint("  (ctrl+o / ← →)"))
	return strings.Join(parts, " ")
}
