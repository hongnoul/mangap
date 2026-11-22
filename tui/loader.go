package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/codewandler/spinners"
)

// loaderMaxW caps the Knight Rider bar so it stays a scanner, not a ruler.
const loaderMaxW = 40

// loaderTickMsg advances one loader. id routes it to its owner, gen drops
// ticks from a previous start so restarting never doubles the speed.
type loaderTickMsg struct{ id, gen int }

var loaderIDs int

// loader drives a codewandler/spinners animation off our own tick, since
// that package's Model targets Bubble Tea v2.
type loader struct {
	spinner spinners.Spinner
	id      int
	tick    int
	gen     int
	running bool
}

func newLoader() loader { return newLoaderWith(spinners.KnightRider) }

func newLoaderWith(s spinners.Spinner) loader {
	loaderIDs++
	return loader{spinner: s, id: loaderIDs}
}

func (l *loader) fps() time.Duration {
	if l.spinner.FPS > 0 {
		return l.spinner.FPS
	}
	return time.Second / 16
}

func (l *loader) next() tea.Cmd {
	id, gen := l.id, l.gen
	return tea.Tick(l.fps(), func(time.Time) tea.Msg { return loaderTickMsg{id: id, gen: gen} })
}

// start begins (or restarts) the animation loop.
func (l *loader) start() tea.Cmd {
	l.gen++
	l.running = true
	return l.next()
}

func (l *loader) stop() {
	l.gen++
	l.running = false
}

// owns reports whether msg belongs to this loader.
func (l *loader) owns(msg loaderTickMsg) bool { return msg.id == l.id }

// update consumes our tick and schedules the next one.
func (l *loader) update(msg loaderTickMsg) tea.Cmd {
	if !l.running || msg.id != l.id || msg.gen != l.gen {
		return nil
	}
	l.tick++
	return l.next()
}

// view renders the bar into exactly w columns (capped at loaderMaxW).
func (l *loader) view(w int) string {
	if w > loaderMaxW {
		w = loaderMaxW
	}
	if w < 4 {
		w = 4
	}
	return l.spinner.Frames(l.tick, w)
}

// viewAt renders the bar with a per-row phase offset, so stacked bars
// shimmer out of step instead of in lockstep.
func (l *loader) viewAt(w, phase int) string {
	if w < 2 {
		w = 2
	}
	return l.spinner.Frames(l.tick+phase, w)
}
