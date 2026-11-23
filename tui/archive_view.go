package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/archive"
	"github.com/hongnoul/mangap/color"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/style"
	"github.com/hongnoul/mangap/where"
	"github.com/muesli/reflow/truncate"
	"github.com/samber/lo"
)

// archiveView is the downloads viewer state: a filter box over the grid
// and an editable save directory box.
type archiveView struct {
	on      bool
	all     []list.Item // every downloaded manga, unfiltered
	filter  textinput.Model
	dir     textinput.Model
	editDir bool   // the save dir box has focus
	typing  bool   // the filter box has focus (/ to start, esc to stop)
	dirErr  string // last save dir error, shown in the box
	anchor  int    // v select mode start index, -1 when off
}

func newArchiveView() archiveView {
	f := textinput.New()
	f.Placeholder = "/ to filter downloads"
	f.Prompt = ""
	f.CharLimit = 60
	d := textinput.New()
	d.Prompt = ""
	d.CharLimit = 1024
	return archiveView{filter: f, dir: d, anchor: -1}
}

// isArchiveDirKey matches ctrl+e, which edits the save directory.
func isArchiveDirKey(km tea.KeyMsg) bool { return km.String() == "ctrl+e" }

// loadArchiveItems rescans the downloads dir into the unfiltered list.
func (b *statefulBubble) loadArchiveItems() {
	mangas := archive.New(where.Downloads()).Mangas()
	b.arch.all = make([]list.Item, len(mangas))
	for i, m := range mangas {
		b.arch.all[i] = &listItem{internal: m}
	}
}

// applyArchiveFilter shows the downloaded manga matching the filter text.
func (b *statefulBubble) applyArchiveFilter() tea.Cmd {
	q := strings.ToLower(strings.TrimSpace(b.arch.filter.Value()))
	nq := normTitle(q)
	items := lo.Filter(b.arch.all, func(it list.Item, _ int) bool {
		if q == "" {
			return true
		}
		name := it.(*listItem).internal.(*source.Manga).Name
		return strings.Contains(strings.ToLower(name), q) ||
			(nq != "" && strings.Contains(normTitle(name), nq))
	})
	cmd := b.mangasC.SetItems(items)
	b.mangasC.ResetSelected()
	return cmd
}

// expandPath resolves ~ and env vars like the config loader does.
func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if home, err := os.UserHomeDir(); err == nil {
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~"+string(os.PathSeparator)) {
			p = filepath.Join(home, p[2:])
		}
	}
	return os.ExpandEnv(p)
}

// setDownloadsDir persists a new save directory and reloads the archive.
func (b *statefulBubble) setDownloadsDir(p string) tea.Cmd {
	p = expandPath(p)
	if p == "" {
		b.arch.dirErr = "path is empty"
		return nil
	}
	abs, err := filepath.Abs(p)
	if err == nil {
		err = os.MkdirAll(abs, os.ModePerm)
	}
	if err == nil {
		err = savePref(key2.DownloaderPath, abs)
	}
	if err != nil {
		b.arch.dirErr = err.Error()
		return nil
	}
	b.arch.dirErr = ""
	b.arch.editDir = false
	b.arch.dir.Blur()
	invalidateDownloaded()
	b.thumbs.Clear()
	b.preview.Clear()
	b.loadArchiveItems()
	return b.applyArchiveFilter()
}

// updateArchiveKeys handles keys in the downloads viewer before the grid
// sees them. Printable keys type into the filter (or dir) box, like the
// search screen. Returns handled=false to let the normal path run.
func (b *statefulBubble) updateArchiveKeys(km tea.KeyMsg) (tea.Cmd, bool) {
	var cmd tea.Cmd
	if b.arch.editDir {
		switch km.String() {
		case "esc":
			b.arch.editDir = false
			b.arch.dirErr = ""
			b.arch.dir.Blur()
			return nil, true
		case "enter":
			return b.setDownloadsDir(b.arch.dir.Value()), true
		case "ctrl+c":
			return nil, false
		}
		b.arch.dir, cmd = b.arch.dir.Update(km)
		b.arch.dirErr = ""
		return cmd, true
	}
	if !b.arch.typing {
		// Browsing: hjkl and the grid keys move, / starts filtering.
		switch {
		case isArchiveDirKey(km):
			return b.startDirEdit(), true
		case km.String() == "v":
			if b.arch.anchor >= 0 {
				b.arch.anchor = -1 // keep the range marked
			} else {
				b.arch.anchor = b.mangasC.Index()
				b.applyArchiveVisual()
			}
			return nil, true
		case km.String() == "d":
			b.askDeleteMangas()
			return nil, true
		case km.String() == "esc" && b.clearArchiveSelection():
			return nil, true
		case km.String() == "/":
			b.arch.typing = true
			return b.arch.filter.Focus(), true
		case km.String() == "esc" && b.arch.filter.Value() != "":
			b.arch.filter.SetValue("")
			return b.applyArchiveFilter(), true
		}
		return nil, false
	}
	switch s := km.String(); {
	case isArchiveDirKey(km):
		b.stopTyping()
		return b.startDirEdit(), true
	case s == "esc":
		// Leave the filter: back to browsing the full list.
		b.stopTyping()
		b.arch.filter.SetValue("")
		return b.applyArchiveFilter(), true
	case s == "enter":
		// Keep the filter, go back to browsing the matches.
		b.stopTyping()
		return nil, true
	case s == "tab" || s == "ctrl+n" || s == "down":
		b.grid.nav(&b.mangasC, "j")
		return nil, true
	case s == "shift+tab" || s == "ctrl+p" || s == "up":
		b.grid.nav(&b.mangasC, "k")
		return nil, true
	case lo.Contains([]string{"ctrl+j", "ctrl+k", "pgup", "pgdown"}, s):
		b.grid.nav(&b.mangasC, s)
		return nil, true
	case s == "ctrl+c":
		return nil, false
	}
	prev := b.arch.filter.Value()
	b.arch.filter, cmd = b.arch.filter.Update(km)
	if b.arch.filter.Value() != prev {
		return tea.Batch(cmd, b.applyArchiveFilter()), true
	}
	return cmd, true
}

func (b *statefulBubble) stopTyping() {
	b.arch.typing = false
	b.arch.filter.Blur()
}

func (b *statefulBubble) startDirEdit() tea.Cmd {
	b.arch.editDir = true
	b.arch.dir.SetValue(where.Downloads())
	b.arch.dir.CursorEnd()
	return b.arch.dir.Focus()
}

// archiveHeader renders one box: the filter row over the save directory
// row, split by a horizontal separator.
func (b *statefulBubble) archiveHeader(full int) string {
	boxW := imax(full-4, 24)
	inner := imax(boxW-4, 8)
	leftW, rightW := inner, inner
	faint := lipgloss.NewStyle().Faint(true)
	fit := func(s string, w int) string {
		s = lipgloss.NewStyle().MaxWidth(w).Render(s)
		return s + strings.Repeat(" ", imax(w-lipgloss.Width(s), 0))
	}

	// Filter half, count on the right.
	count := faint.Render(lo.Ternary(b.arch.filter.Value() == "",
		strconv.Itoa(len(b.arch.all))+" downloaded",
		strconv.Itoa(len(b.mangasC.Items()))+"/"+strconv.Itoa(len(b.arch.all))))
	b.arch.filter.Width = imax(leftW-lipgloss.Width(count)-3, 4)
	left := b.arch.filter.View()
	left += strings.Repeat(" ", imax(leftW-lipgloss.Width(left)-lipgloss.Width(count), 1)) + count

	// Save directory half.
	label := style.Faint("Save dir  ")
	hint := faint.Render(lo.Ternary(b.arch.editDir, "enter save · esc cancel", "ctrl+e change"))
	valW := imax(rightW-lipgloss.Width(label)-lipgloss.Width(hint)-2, 4)
	var val string
	if b.arch.editDir {
		b.arch.dir.Width = valW - 1
		val = b.arch.dir.View()
	} else {
		val = truncLeft(where.Downloads(), valW)
	}
	if b.arch.dirErr != "" {
		val = lipgloss.NewStyle().Foreground(color.Red).Render(truncate.StringWithTail(b.arch.dirErr, uint(valW), "…"))
		if b.arch.editDir {
			val = b.arch.dir.View()
			hint = lipgloss.NewStyle().Foreground(color.Red).Render(truncate.StringWithTail(b.arch.dirErr, uint(imax(rightW/3, 8)), "…"))
		}
	}
	right := label + val
	right += strings.Repeat(" ", imax(rightW-lipgloss.Width(right)-lipgloss.Width(hint), 1)) + hint

	border := lipgloss.Color("8")
	switch {
	case b.arch.typing:
		border = lipgloss.Color("5")
	case b.arch.editDir:
		border = color.Purple
	}
	bc := lipgloss.NewStyle().Foreground(border)
	line := func(l, m, r string) string { return bc.Render(l + strings.Repeat(m, inner+2) + r) }
	side := bc.Render("│")
	box := strings.Join([]string{
		line("╭", "─", "╮"),
		side + " " + fit(left, inner) + " " + side,
		line("├", "─", "┤"),
		side + " " + fit(right, inner) + " " + side,
		line("╰", "─", "╯"),
	}, "\n")
	return lipgloss.PlaceHorizontal(full, lipgloss.Center, box)
}

// viewArchive is the downloads viewer: header boxes over the grid.
func (b *statefulBubble) viewArchive() string {
	full := b.width + 4
	header := b.archiveHeader(full)
	b.applyArchiveVisual()
	H := b.height + 2 - lipgloss.Height(header)
	if len(b.mangasC.Items()) == 0 {
		msg := lo.Ternary(len(b.arch.all) == 0, "nothing downloaded in this folder yet", "no downloads match")
		return header + "\n\n" + lipgloss.PlaceHorizontal(full, lipgloss.Center, style.Faint(msg))
	}
	return header + "\n" + b.viewGridH(&b.mangasC, mangaCard, func(th *thumbs, it list.Item, prio int) {
		th.WantManga(it.(*listItem).internal.(*source.Manga), prio)
	}, H, true)
}

// truncLeft keeps the end of s (the useful part of a path) within w cells.
func truncLeft(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	return "…" + string(r[len(r)-(w-1):])
}
