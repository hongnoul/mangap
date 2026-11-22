// Package reader is the in-terminal manga reader: one goroutine owns the
// terminal and reconciles it with the view state, render lanes produce
// frames in the background. Ported from pdfpane's App.swift.
//
// The loop: wait for keys, a resize, or a finished render; apply every key
// in the batch to the state; then present() once. present is state-driven
// and idempotent, so a key batch that nets back to the shown frame writes
// zero bytes, and held-key autorepeat can never queue stale frames.
package reader

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/hongnoul/mangaz/reader/input"
	"github.com/hongnoul/mangaz/reader/kitty"
	"github.com/hongnoul/mangaz/reader/pages"
	"github.com/hongnoul/mangaz/reader/render"
	"github.com/hongnoul/mangaz/reader/term"
	"github.com/hongnoul/mangaz/reader/viewer"
	"github.com/muesli/cancelreader"
)

// ErrNoKittyGraphics is returned when the terminal does not advertise the
// kitty graphics protocol. mangaz has no fallback viewer by design.
var ErrNoKittyGraphics = errors.New("this terminal does not support the kitty graphics protocol. " +
	"Run mangaz in kitty, ghostty, wezterm, or gwae (or force it with MANGAZ_KITTY_GRAPHICS=1)")

// Options configures a reading session.
type Options struct {
	// Start is the 0-based page to open at.
	Start int
	// RTL overrides the default (right-to-left).
	RTL *bool
	// Lanes is the render worker count (0 = auto).
	Lanes int
	// In/Out default to the process stdin/stdout.
	In, Out *os.File
	// Trace receives timing lines (never the terminal).
	Trace io.Writer
	// Chapters, if set, lets j/k cross into the neighbor chapters.
	Chapters Chapters
}

// Result reports where the reader stopped.
type Result struct {
	Page int // 0-based
	// Chapter is the chapter position the reader stopped in, relative to
	// the one it opened (0 unless Options.Chapters crossed an edge).
	Chapter int
}

// Open reads the chapter at path (directory or .cbz/.zip).
func Open(path string, opt Options) (Result, error) {
	src, err := pages.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer src.Close()
	return Run(src, opt)
}

// Run reads src until the user quits.
func Run(src pages.Source, opt Options) (Result, error) {
	if !term.SupportsKittyGraphics() {
		return Result{}, ErrNoKittyGraphics
	}
	if opt.In == nil {
		opt.In = os.Stdin
	}
	if opt.Out == nil {
		opt.Out = os.Stdout
	}
	r := newSession(src, opt)
	return r.run()
}

type session struct {
	opt   Options
	src   pages.Source
	state *viewer.State
	cache *viewer.Cache
	kt    *kitty.Transport
	pf    *viewer.Prefetcher
	out   *os.File
	t0    time.Time

	shown  *viewer.Target
	recent []viewer.Grid
	// awaiting is the missed target already demanded (and, for a resize,
	// already stretched). Unrelated Ready events re-enter present; they
	// must not re-demand or rewrite the same stretch frame.
	awaiting string

	// Chapter crossing: cur is the chapter shown (relative position),
	// chapters the loaded ones (cur and its neighbors).
	cur          int
	chapters     map[int]*chapter
	loading      map[int]bool
	failed       map[int]error
	loaded       chan loadResult
	ready        chan struct{}
	done         chan struct{}
	pendingCross int
	pendingEntry int
	toast        string
	toastT       *time.Timer
	repaint      bool

	gotoDigits string
	gotoOrigin int
	inGoto     bool

	// Test seams: terminal acquisition and size. Real sessions use the
	// term package; tests drive the loop over pipes.
	acquire func() (restore func([]byte), err error)
	size    func() term.Size
	resizes func() (<-chan struct{}, func())
}

func newSession(src pages.Source, opt Options) *session {
	st := viewer.NewState(src.Len())
	if opt.RTL != nil {
		st.RTL = *opt.RTL
	}
	st.GoTo(opt.Start)
	st.LastDir = 1
	s := &session{
		opt: opt, src: src, state: st,
		cache: viewer.NewCache(), kt: kitty.New(),
		out: opt.Out, t0: time.Now(),
		chapters: map[int]*chapter{}, loading: map[int]bool{},
		failed: map[int]error{}, loaded: make(chan loadResult, 4),
		ready: make(chan struct{}, 1), done: make(chan struct{}),
	}
	s.pf = s.newPrefetcher(src)
	s.chapters[0] = &chapter{src: src, pf: s.pf}
	s.acquire = func() (func([]byte), error) {
		t, err := term.Acquire(opt.In, opt.Out)
		if err != nil {
			return nil, err
		}
		return t.Restore, nil
	}
	s.size = func() term.Size { return term.GetSize(opt.Out) }
	s.resizes = term.Resizes
	return s
}

func (s *session) trace(format string, args ...any) {
	if s.opt.Trace == nil {
		return
	}
	ms := float64(time.Since(s.t0).Microseconds()) / 1000
	fmt.Fprintf(s.opt.Trace, "TRACE %.1f %s\n", ms, fmt.Sprintf(format, args...))
}

func (s *session) run() (res Result, err error) {
	restore, err := s.acquire()
	if err != nil {
		return Result{}, err
	}
	defer func() {
		close(s.done)
		for _, c := range s.chapters {
			s.closeChapter(c)
		}
		if s.toastT != nil {
			s.toastT.Stop()
		}
	}()
	// Restore on every path, including panics: a reader that dies in raw
	// mode on the alternate screen leaves the user's shell unusable.
	defer func() {
		if p := recover(); p != nil {
			restore(s.kt.DeleteAll(nil))
			panic(p)
		}
		restore(s.kt.DeleteAll(nil))
	}()

	resize, stopResize := s.resizes()
	defer stopResize()

	cr, err := cancelreader.NewReader(s.opt.In)
	if err != nil {
		return Result{}, err
	}
	keys := make(chan []byte, 64)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, err := cr.Read(buf)
			if n > 0 {
				b := make([]byte, n)
				copy(b, buf[:n])
				keys <- b
			}
			if err != nil {
				close(keys)
				return
			}
		}
	}()
	// Stop the stdin goroutine before handing the terminal back: a
	// leftover reader would steal the caller's next keystrokes (bubbletea
	// resumes reading stdin right after us).
	defer func() {
		cr.Cancel()
		<-readDone
		_ = cr.Close()
	}()

	var parser input.Parser
	if s.opt.Chapters != nil {
		s.setToast(s.label(0), 2*time.Second)
	}
	s.present()
	for {
		select {
		case b, ok := <-keys:
			if !ok {
				return s.result(), nil
			}
			// Coalesce: fold every byte already queued into this pass.
			ks := parser.Feed(b)
		drain:
			for {
				select {
				case more, ok := <-keys:
					if !ok {
						break drain
					}
					ks = append(ks, parser.Feed(more)...)
				default:
					break drain
				}
			}
			if s.apply(ks) {
				return s.result(), nil
			}
		case <-resize:
		case <-s.ready:
		case r := <-s.loaded:
			s.onLoaded(r)
		case <-s.toastC():
			s.toastT = nil
			s.clearToast()
		}
		s.present()
	}
}

func (s *session) result() Result {
	return Result{Page: s.state.Page, Chapter: s.cur}
}

// apply folds a key batch into the state. Reports quit.
func (s *session) apply(ks []input.Key) bool {
	for _, k := range ks {
		st := s.state
		if s.inGoto {
			switch k.Action {
			case input.Digit:
				if len(s.gotoDigits) < 5 {
					s.gotoDigits += strconv.Itoa(k.Digit)
				}
				s.gotoApply()
				continue
			case input.Backspace:
				if s.gotoDigits != "" {
					s.gotoDigits = s.gotoDigits[:len(s.gotoDigits)-1]
				}
				s.gotoApply()
				continue
			case input.Enter:
				s.inGoto = false
				continue
			default:
				// Any other key ends goto and still runs its binding.
				s.inGoto = false
			}
		}
		switch k.Action {
		case input.Quit:
			return true
		case input.Next:
			s.flip(1)
		case input.Prev:
			s.flip(-1)
		case input.Left:
			s.flip(s.state.PhysicalDelta(true))
		case input.Right:
			s.flip(s.state.PhysicalDelta(false))
		case input.NextChapter:
			s.jumpChapter(1)
		case input.PrevChapter:
			s.jumpChapter(-1)
		case input.First:
			st.GoTo(0)
		case input.Last:
			st.GoTo(st.Pages - 1)
		case input.RotateCW:
			st.Rotate(1)
		case input.RotateCCW:
			st.Rotate(-1)
		case input.Spread:
			st.ToggleSpread()
		case input.Direction:
			st.RTL = !st.RTL
		case input.Goto:
			s.inGoto, s.gotoDigits, s.gotoOrigin = true, "", st.Page
		}
	}
	return false
}

// gotoApply navigates live as digits are typed (pdfpane's f): f 3 shows
// page 3, then 4 shows 34. Pages are 1-based for humans.
func (s *session) gotoApply() {
	if s.gotoDigits == "" {
		s.state.GoTo(s.gotoOrigin)
		return
	}
	n, err := strconv.Atoi(s.gotoDigits)
	if err != nil {
		return
	}
	s.state.GoTo(n - 1)
}

func (s *session) grid() viewer.Grid {
	sz := s.size()
	return viewer.GridFor(viewer.Viewport{Cols: sz.Cols, Rows: sz.Rows, CellW: sz.CellW, CellH: sz.CellH})
}

func (s *session) noteGrid(g viewer.Grid) {
	out := []viewer.Grid{g}
	for _, r := range s.recent {
		if r != g && len(out) < 4 {
			out = append(out, r)
		}
	}
	s.recent = out
}

// warm is the neighbor set plus the current page at recently seen grids
// (a width cycle back to a seen width is a pure placeholder repaint).
func (s *session) warm(t viewer.Target) []viewer.Target {
	ws := s.state.WarmSet(t)
	for _, g := range s.recent {
		if g != t.Grid {
			n := t
			n.Grid = g
			ws = append(ws, n)
		}
	}
	return ws
}

func (s *session) hit(t viewer.Target) bool {
	id, ok := s.cache.Lookup(t.Key())
	return ok && s.kt.Transmitted(id)
}

// present makes the pane show the current state as fast as truth allows:
// transmitted id -> placeholder repaint; ready render -> transmit+paint;
// miss -> demand it urgently, and meanwhile show the nearest ready page
// (held-key scroll) or stretch the shown frame to a new grid (resize).
func (s *session) present() {
	g := s.grid()
	t := s.state.Target(g)
	s.warmNeighbors(g)
	if s.shown != nil && s.shown.Key() == t.Key() {
		s.pf.ClearAwait()
		s.pf.Want(s.warm(t))
		if s.repaint {
			s.show(t, nil)
		}
		return
	}
	if s.hit(t) {
		s.pf.ClearAwait()
		s.show(t, nil)
		s.pf.Want(s.warm(t))
		return
	}
	if r := s.pf.Take(t); r != nil {
		s.pf.ClearAwait()
		s.show(t, r)
		s.pf.Want(s.warm(t))
		return
	}
	if s.awaiting == t.Key() {
		return
	}
	s.trace("miss %s", t.Key())

	// Scroll bridge: a held key moves the target faster than renders land.
	// Show the nearest ready page between the shown one and the target so
	// the pane keeps moving instead of freezing until release.
	if sh := s.shown; sh != nil && sh.Chapter == t.Chapter && sh.Grid == t.Grid && sh.Rotation == t.Rotation &&
		sh.Spread == t.Spread && sh.RTL == t.RTL && sh.Page != t.Page {
		dir := 1
		if t.Page < sh.Page {
			dir = -1
		}
		p := t.Page
		for steps := 0; steps < 8; steps++ {
			p = s.state.SheetStart(p - dir*s.state.Stride())
			if dir*(p-sh.Page) <= 0 {
				break
			}
			c := s.state.TargetAt(p, g)
			if r := s.pf.Take(c); r != nil || s.hit(c) {
				s.pf.Demand([]viewer.Target{t}, s.warm(t))
				s.trace("scroll-bridge %s lag=%d", c.Key(), abs(t.Page-p))
				s.show(c, r)
				return
			}
		}
	}

	s.pf.Demand([]viewer.Target{t}, s.warm(t))
	s.awaiting = t.Key()

	// Resize stretch: same content at a new grid. Re-span the on-screen
	// image to the new cells (zero image bytes); the crisp frame replaces
	// it when it lands.
	if sh := s.shown; sh != nil && sh.Chapter == t.Chapter && sh.Grid != t.Grid && sh.Page == t.Page &&
		sh.Rotation == t.Rotation && sh.Spread == t.Spread && sh.RTL == t.RTL {
		if id, ok := s.cache.Lookup(sh.Key()); ok && s.kt.Transmitted(id) {
			out := []byte("\x1b[?2026h")
			out = s.kt.RePlace(out, id, t.Grid.Cols, t.Grid.Rows)
			out = append(out, "\x1b[H"...)
			out = append(out, s.kt.PlaceholderFrame(id, t.Grid.Cols, t.Grid.Rows)...)
			out = append(out, "\x1b[?2026l"...)
			s.write(out)
		}
	}
}

// show paints t: free evicted ids, transmit if new, heal the placement
// span, then one synchronized full-pane placeholder overwrite, all in one
// write. No clear: every cell is rewritten, so no blank flash.
func (s *session) show(t viewer.Target, r *render.Result) {
	id, evict := s.cache.ID(t.Key())
	var out []byte
	for _, old := range evict {
		out = s.kt.Delete(out, old)
	}
	if !s.kt.Transmitted(id) {
		if r == nil {
			if len(out) > 0 {
				s.write(out)
			}
			return
		}
		out, _ = s.kt.Transmit(out, r.PNG, r.W, r.H, id, t.Grid.Cols, t.Grid.Rows)
		for _, old := range s.cache.Transmitted(id, len(r.PNG)) {
			if old != id {
				out = s.kt.Delete(out, old)
			}
		}
	}
	out = s.kt.EnsurePlacement(out, id, t.Grid.Cols, t.Grid.Rows)
	out = append(out, "\x1b[?2026h\x1b[H"...)
	out = append(out, s.kt.PlaceholderFrame(id, t.Grid.Cols, t.Grid.Rows)...)
	out = s.toastBytes(out)
	out = append(out, "\x1b[?2026l"...)
	s.repaint = false
	t0 := time.Now()
	s.write(out)
	s.trace("show %s %dB %.1fms", t.Key(), len(out), float64(time.Since(t0).Microseconds())/1000)
	tt := t
	s.shown = &tt
	s.awaiting = ""
	s.cache.Displayed = t.Key()
	s.noteGrid(t.Grid)
}

func (s *session) write(b []byte) {
	_, _ = s.out.Write(b)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
