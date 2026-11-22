package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/style"
	"github.com/spf13/viper"
)

type dropdownKind int

const (
	sourceDropdown dropdownKind = iota + 1
	languageDropdown
)

// dropdown is an expanded picker under the source or language box.
type dropdown struct {
	kind   dropdownKind
	cursor int
}

// dropdownMaxRows caps how many options show at once.
const dropdownMaxRows = 10

func (b *statefulBubble) openDropdown(kind dropdownKind) {
	d := &dropdown{kind: kind}
	if kind == sourceDropdown {
		d.cursor = b.sourceIdx
	} else {
		d.cursor = languageIndex(viper.GetString(key2.MangadexLanguage))
	}
	b.dropdown = d
}

func (b *statefulBubble) dropdownOptions() []string {
	if b.dropdown == nil {
		return nil
	}
	if b.dropdown.kind == sourceDropdown {
		var out []string
		for _, n := range b.sourceChoices {
			if strings.EqualFold(n, "mangapill") {
				n += " (Recommended)"
			}
			out = append(out, n)
		}
		return out
	}
	var out []string
	for _, l := range languages {
		out = append(out, l.name+style.Faint(" "+l.code))
	}
	return out
}

// updateDropdown handles keys while a dropdown is open.
func (b *statefulBubble) updateDropdown(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	d := b.dropdown
	n := len(b.dropdownOptions())
	switch km.String() {
	case "up", "k", "ctrl+p", "shift+tab":
		if n > 0 {
			d.cursor = (d.cursor - 1 + n) % n
		}
	case "down", "j", "ctrl+n", "tab":
		if n > 0 {
			d.cursor = (d.cursor + 1) % n
		}
	case "g", "home":
		d.cursor = 0
	case "G", "end":
		d.cursor = imax(n-1, 0)
	case "ctrl+s":
		if d.kind == sourceDropdown {
			// Second press: full source settings (order, enable, ...).
			b.dropdown = nil
			b.openSourceSettings()
			return b, nil
		}
		b.openDropdown(sourceDropdown)
	case "ctrl+l":
		if d.kind == languageDropdown {
			b.dropdown = nil
			return b, nil
		}
		b.openDropdown(languageDropdown)
	case "enter", "l", "right":
		b.dropdown = nil
		if n == 0 {
			return b, nil
		}
		if d.kind == sourceDropdown {
			if err := b.pickSource(d.cursor); err != nil {
				b.raiseError(err)
				return b, nil
			}
		} else if err := provider.SaveMangadexLanguage(languages[d.cursor].code); err != nil {
			b.raiseError(err)
			return b, nil
		}
		return b, b.scheduleLive()
	case "esc", "q", "h", "left":
		b.dropdown = nil
	case "ctrl+c":
		return b, tea.Quit
	}
	return b, nil
}

// viewDropdown renders the open option list, w wide.
func (b *statefulBubble) viewDropdown(w int) string {
	d := b.dropdown
	opts := b.dropdownOptions()
	start := 0
	if d.cursor >= dropdownMaxRows {
		start = d.cursor - dropdownMaxRows + 1
	}
	end := imin(len(opts), start+dropdownMaxRows)
	inner := imax(w-4, 4)
	var lines []string
	for i := start; i < end; i++ {
		line := "  " + opts[i]
		if i == d.cursor {
			line = lipgloss.NewStyle().Reverse(true).Bold(true).Width(inner).
				Render("> " + stripFaint(opts[i]))
		}
		lines = append(lines, line)
	}
	hint := "↑/↓ move • enter pick • esc close"
	if d.kind == sourceDropdown {
		hint += " • ctrl+s settings"
	}
	lines = append(lines, style.Faint(hint))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(0, 1).
		Width(imax(w-2, 4)).
		Render(strings.Join(lines, "\n"))
}

// stripFaint drops ANSI styling so the highlighted row renders uniformly.
func stripFaint(s string) string {
	var out strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			out.WriteRune(r)
		}
	}
	return out.String()
}
