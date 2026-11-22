package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/codewandler/spinners"
)

// loaderMaxW caps the Knight Rider bar so it stays a scanner, not a ruler.
const loaderMaxW = 40

// loaderTickMsg advances the loading bar. gen drops ticks from a previous
// start so restarting never doubles the speed.
type loaderTickMsg struct{ gen int }

// loader drives a codewandler/spinners animation off our own tick, since
// that package's Model targets Bubble Tea v2.
type loader struct {
	spinner spinners.Spinner
	tick    int
	gen     int
	running bool
}

func newLoader() loader {
	return loader{spinner: spinners.KnightRider}
}

func (l *loader) fps() time.Duration {
	if l.spinner.FPS > 0 {
		return l.spinner.FPS
	}
	return time.Second / 16
}

func (l *loader) next() tea.Cmd {
	gen := l.gen
	return tea.Tick(l.fps(), func(time.Time) tea.Msg { return loaderTickMsg{gen: gen} })
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

// update consumes our tick and schedules the next one.
func (l *loader) update(msg loaderTickMsg) tea.Cmd {
	if !l.running || msg.gen != l.gen {
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
