package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/color"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/provider/mangadex"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
	"github.com/hongnoul/mangap/util"
	"github.com/spf13/viper"
)

// sourceRow is one line of the source settings screen.
type sourceRow struct {
	p       *provider.Provider
	enabled bool
}

// sourceSettings is the editable copy of the source preferences. Nothing
// is written until the user saves.
type sourceSettings struct {
	rows   []sourceRow
	cursor int
	auto   bool
	status string
	lang   int // index into languages
}

// language is a selectable MangaDex translation language.
type language struct{ code, name string }

// languages are common MangaDex translatedLanguage codes. Mangapill is
// English only, so this preference affects MangaDex.
var languages = []language{
	{"en", "English"},
	{"any", "Any language"},
	{"ja", "Japanese"},
	{"ko", "Korean"},
	{"zh", "Chinese (simplified)"},
	{"zh-hk", "Chinese (traditional)"},
	{"es", "Spanish"},
	{"es-la", "Spanish (Latin America)"},
	{"pt-br", "Portuguese (Brazil)"},
	{"pt", "Portuguese"},
	{"fr", "French"},
	{"de", "German"},
	{"it", "Italian"},
	{"ru", "Russian"},
	{"uk", "Ukrainian"},
	{"pl", "Polish"},
	{"tr", "Turkish"},
	{"ar", "Arabic"},
	{"id", "Indonesian"},
	{"vi", "Vietnamese"},
	{"th", "Thai"},
}

// languageIndex finds code in languages, appending unknown codes from the
// config so they are preserved.
func languageIndex(code string) int {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		code = "en"
	}
	for i, l := range languages {
		if l.code == code {
			return i
		}
	}
	languages = append(languages, language{code, code})
	return len(languages) - 1
}

func (ss *sourceSettings) cycleLang(d int) {
	n := len(languages)
	ss.lang = ((ss.lang+d)%n + n) % n
}

// langWarning explains when the language choice will not take effect.
func (ss *sourceSettings) langWarning() string {
	if languages[ss.lang].code == "en" {
		return ""
	}
	for _, r := range ss.rows {
		if !r.enabled {
			continue
		}
		if r.p.ID == mangadex.ID {
			return ""
		}
		if ss.auto {
			return r.p.Name + " is English only and ranks above MangaDex (press p to fix)"
		}
	}
	return "MangaDex is disabled, so this language is unused (press p to fix)"
}

// preferMangadex enables MangaDex and moves it to the top.
func (ss *sourceSettings) preferMangadex() {
	for i, r := range ss.rows {
		if r.p.ID == mangadex.ID {
			r.enabled = true
			ss.rows = append(ss.rows[:i], ss.rows[i+1:]...)
			ss.rows = append([]sourceRow{r}, ss.rows...)
			ss.cursor = 0
			return
		}
	}
}

// newSourceSettings loads rows in priority order: sources.order first,
// then remaining builtins, then customs (disabled until enabled here).
func newSourceSettings() *sourceSettings {
	enabled := map[string]bool{}
	for _, p := range provider.Ordered() {
		enabled[p.Name] = true
	}

	ss := &sourceSettings{
		auto: provider.IsAuto(),
		lang: languageIndex(viper.GetString(key2.MangadexLanguage)),
	}
	seen := map[string]bool{}
	add := func(p *provider.Provider) {
		if seen[p.Name] {
			return
		}
		seen[p.Name] = true
		ss.rows = append(ss.rows, sourceRow{p: p, enabled: enabled[p.Name]})
	}
	for _, p := range provider.Ordered() {
		add(p)
	}
	for _, n := range viper.GetStringSlice(key2.SourcesOrder) {
		if p, ok := provider.Get(n); ok {
			add(p)
		}
	}
	for _, p := range provider.Builtins() {
		add(p)
	}
	for _, p := range provider.Customs() {
		add(p)
	}
	return ss
}

func (ss *sourceSettings) move(d int) {
	j := ss.cursor + d
	if j < 0 || j >= len(ss.rows) {
		return
	}
	ss.rows[ss.cursor], ss.rows[j] = ss.rows[j], ss.rows[ss.cursor]
	ss.cursor = j
}

func (ss *sourceSettings) save() error {
	var order, disabled []string
	for _, r := range ss.rows {
		order = append(order, r.p.Name)
		if !r.enabled {
			disabled = append(disabled, r.p.Name)
		}
	}
	if disabled == nil {
		disabled = []string{}
	}
	mode := "auto"
	if !ss.auto {
		mode = "manual"
	}
	viper.Set(key2.MangadexLanguage, languages[ss.lang].code)
	return provider.SaveSourcePrefs(mode, order, disabled)
}

func (b *statefulBubble) openSourceSettings() {
	b.sourceSettings = newSourceSettings()
	b.newState(sourceSettingsState)
}

func (b *statefulBubble) updateSourceSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	ss := b.sourceSettings
	km, ok := msg.(tea.KeyMsg)
	if !ok || ss == nil {
		return b, nil
	}

	switch km.String() {
	case "up", "k":
		if ss.cursor > 0 {
			ss.cursor--
		}
	case "down", "j":
		if ss.cursor < len(ss.rows)-1 {
			ss.cursor++
		}
	case "K", "shift+up":
		ss.move(-1)
	case "J", "shift+down":
		ss.move(1)
	case " ", "x":
		ss.rows[ss.cursor].enabled = !ss.rows[ss.cursor].enabled
	case "m", "tab":
		ss.auto = !ss.auto
	case "l", "right":
		ss.cycleLang(1)
	case "h", "left":
		ss.cycleLang(-1)
	case "p":
		ss.preferMangadex()
	case "enter":
		n := 0
		for _, r := range ss.rows {
			if r.enabled {
				n++
			}
		}
		if ss.auto && n == 0 {
			ss.status = "Enable at least one source for auto mode"
			return b, nil
		}
		if err := ss.save(); err != nil {
			b.raiseError(err)
			return b, nil
		}
		b.sourceSettings = nil
		if ss.auto {
			auto, err := provider.NewAuto()
			if err != nil {
				b.raiseError(err)
				return b, nil
			}
			b.selectedSources = []source.Source{auto}
			b.sourceChoices = sourceChoices()
			b.sourceIdx = len(b.sourceChoices) - 1
			b.statesHistory = util.Stack[state]{}
			b.setState(searchState)
			b.inputC.Focus()
			return b, nil
		}
		// Manual: pick sources from the list, like mangal did.
		b.statesHistory = util.Stack[state]{}
		b.setState(sourcesState)
		return b, b.loadProviders()
	default:
		if key.Matches(km, b.keymap.back) {
			b.sourceSettings = nil
			b.previousState()
		}
	}
	return b, nil
}

func (b *statefulBubble) viewSourceSettings() string {
	ss := b.sourceSettings
	if ss == nil {
		return ""
	}

	mode := style.Fg(color.Green)("auto") + style.Faint(": first source with results, top to bottom")
	if !ss.auto {
		mode = style.Fg(color.Orange)("manual") + style.Faint(": pick sources on start")
	}

	lines := []string{
		style.Title("Sources"),
		"",
		"Mode      " + mode,
		"Language  " + style.Fg(color.Green)(languages[ss.lang].name) +
			style.Faint(fmt.Sprintf(" (%s): MangaDex only, Mangapill is English", languages[ss.lang].code)),
	}
	if w := ss.langWarning(); w != "" {
		lines = append(lines, style.Fg(color.Orange)("          "+w))
	}
	lines = append(lines, "")

	rank := 0
	for i, r := range ss.rows {
		box := "[ ]"
		if r.enabled {
			box = style.Fg(color.Green)("[x]")
		}
		num := "  "
		if r.enabled {
			rank++
			num = fmt.Sprintf("%d.", rank)
		}
		kind := "builtin"
		if r.p.IsCustom {
			kind = "custom"
		}
		line := fmt.Sprintf("%s %s %s %s", num, box, r.p.Name, style.Faint(kind))
		if i == ss.cursor {
			line = lipgloss.NewStyle().
				Reverse(true).
				Bold(true).
				Render(fmt.Sprintf("%s [%s] %s %s", num, map[bool]string{true: "x", false: " "}[r.enabled], r.p.Name, kind))
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	if ss.status != "" {
		lines = append(lines, style.Fg(color.Red)(ss.status), "")
	}
	lines = append(lines,
		style.Faint(strings.Join([]string{
			"j/k move", "J/K reorder", "space toggle", "m mode", "h/l language", "p prefer MangaDex", "enter save", "esc cancel",
		}, " • ")),
	)

	return b.renderLines(false, lines)
}

// autoChain is "Mangapill, then Mangadex" for status lines.
func autoChain(srcs []source.Source) string {
	var names []string
	for _, s := range srcs {
		if a, ok := s.(*provider.Auto); ok {
			for _, x := range a.Sources {
				names = append(names, x.Name())
			}
		} else {
			names = append(names, s.Name())
		}
	}
	return strings.Join(names, ", then ")
}
