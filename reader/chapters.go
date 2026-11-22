package reader

import (
	"fmt"
	"time"

	"github.com/hongnoul/mangaz/reader/pages"
	"github.com/hongnoul/mangaz/reader/viewer"
)

// Chapters lets the reader cross chapter edges without leaving: j on the
// last page opens the next chapter, k on the first page the previous one.
// Positions are relative to the chapter the reader opened with (0), so
// +1 is the next chapter in reading order and -1 the previous.
type Chapters interface {
	// Title names chapter pos, or reports false when there is none. It is
	// called on the terminal-owning goroutine and must not block.
	Title(pos int) (string, bool)
	// Load prepares chapter pos (download, open). It runs on a background
	// goroutine; the reader owns and closes the returned source.
	Load(pos int) (pages.Source, error)
	// Entered reports that the reader moved into chapter pos (history).
	Entered(pos int)
}

// chapter is one loaded chapter with its own render lanes.
type chapter struct {
	src pages.Source
	pf  *viewer.Prefetcher
	// owned sources are closed by the reader; the opening chapter's
	// source belongs to the caller.
	owned bool
}

type loadResult struct {
	pos int
	src pages.Source
	err error
}

// keepNear is how many sheets from an edge the neighbor chapter's first
// frames start rendering, so the crossing flip is a cached-frame flip.
const keepNear = 2

// ensureLoad starts fetching chapter pos in the background, once.
func (s *session) ensureLoad(pos int) {
	if s.opt.Chapters == nil {
		return
	}
	if _, ok := s.chapters[pos]; ok || s.loading[pos] {
		return
	}
	if _, ok := s.failed[pos]; ok {
		return
	}
	if _, ok := s.opt.Chapters.Title(pos); !ok {
		return
	}
	s.loading[pos] = true
	s.trace("chapter load start %d", pos)
	go func() {
		src, err := s.opt.Chapters.Load(pos)
		select {
		case s.loaded <- loadResult{pos: pos, src: src, err: err}:
		case <-s.done:
			if src != nil {
				_ = src.Close()
			}
		}
	}()
}

// onLoaded adopts a finished background load, crossing into it if the
// user is already waiting at the edge.
func (s *session) onLoaded(r loadResult) {
	delete(s.loading, r.pos)
	if r.err == nil && r.src != nil && r.src.Len() == 0 {
		r.err = pages.ErrNoPages
	}
	if r.err != nil {
		s.trace("chapter load %d failed: %v", r.pos, r.err)
		s.failed[r.pos] = r.err
		if r.src != nil {
			_ = r.src.Close()
		}
		if s.pendingCross != 0 && s.cur+s.pendingCross == r.pos {
			s.pendingCross = 0
			s.setToast(fmt.Sprintf("Could not load %s: %v", s.title(r.pos), r.err), 4*time.Second)
		}
		return
	}
	if abs(r.pos-s.cur) > 1 {
		_ = r.src.Close()
		return
	}
	s.trace("chapter load done %d pages=%d", r.pos, r.src.Len())
	s.chapters[r.pos] = &chapter{
		src: r.src, owned: true,
		pf: s.newPrefetcher(r.src),
	}
	if s.pendingCross != 0 && s.cur+s.pendingCross == r.pos {
		dir := s.pendingCross
		s.pendingCross = 0
		s.cross(dir)
	}
}

func (s *session) newPrefetcher(src pages.Source) *viewer.Prefetcher {
	pf := viewer.NewPrefetcherReady(src, s.opt.Lanes, s.ready)
	if s.opt.Trace != nil {
		pf.Trace = s.trace
	}
	return pf
}

func (s *session) title(pos int) string {
	if s.opt.Chapters != nil {
		if t, ok := s.opt.Chapters.Title(pos); ok && t != "" {
			return t
		}
	}
	return fmt.Sprintf("chapter %+d", pos)
}

// neighborState is the view state chapter c opens with when entered by
// a flip in direction dir: the first sheet going forward, the last going
// back, with the reader's layout carried over.
func (s *session) neighborState(pos int, c *chapter, dir int) *viewer.State {
	ns := viewer.NewState(c.src.Len())
	ns.Chapter = pos
	ns.RTL, ns.Spread, ns.Rotation = s.state.RTL, s.state.Spread, s.state.Rotation
	if dir < 0 {
		ns.GoTo(ns.Pages - 1)
	}
	ns.LastDir = dir
	return ns
}

// flip moves one sheet in reading order, crossing into the neighbor
// chapter at an edge.
func (s *session) flip(dir int) {
	if s.pendingCross != 0 {
		if dir != s.pendingCross {
			// Changed their mind while the chapter loads: stay here.
			s.pendingCross = 0
			s.clearToast()
			s.state.Flip(dir)
		}
		return
	}
	if !s.state.AtEnd(dir) {
		s.state.Flip(dir)
		return
	}
	s.cross(dir)
}

// cross enters chapter cur+dir: instantly when it is loaded, otherwise
// after it loads (with a toast meanwhile).
func (s *session) cross(dir int) {
	pos := s.cur + dir
	c, ok := s.chapters[pos]
	if !ok {
		if s.opt.Chapters == nil {
			return
		}
		if _, ok := s.opt.Chapters.Title(pos); !ok {
			if dir > 0 {
				s.setToast("Last chapter", 2*time.Second)
			} else {
				s.setToast("First chapter", 2*time.Second)
			}
			return
		}
		if err, ok := s.failed[pos]; ok {
			// Retry on the next press after showing why it failed.
			delete(s.failed, pos)
			s.setToast(fmt.Sprintf("Could not load %s: %v (press again to retry)", s.title(pos), err), 4*time.Second)
			return
		}
		s.ensureLoad(pos)
		s.pendingCross = dir
		s.setToast("Loading "+s.title(pos)+"...", 0)
		return
	}

	if old := s.chapters[s.cur]; old != nil {
		old.pf.ClearAwait()
	}
	s.state = s.neighborState(pos, c, dir)
	s.pf = c.pf
	s.cur = pos
	s.awaiting = ""
	for p, ch := range s.chapters {
		if abs(p-pos) > 1 {
			s.closeChapter(ch)
			delete(s.chapters, p)
		}
	}
	s.trace("cross -> chapter %d", pos)
	s.setToast(s.title(pos), 1500*time.Millisecond)
	if s.opt.Chapters != nil {
		s.opt.Chapters.Entered(pos)
	}
}

func (s *session) closeChapter(c *chapter) {
	c.pf.Close()
	if c.owned {
		_ = c.src.Close()
	}
}

// warmNeighbors keeps the next chapter loading from the start, the
// previous one once the reader heads back to the first pages, and
// renders the neighbor's entry frames when the reader nears an edge.
func (s *session) warmNeighbors(g viewer.Grid) {
	if s.opt.Chapters == nil {
		return
	}
	s.ensureLoad(s.cur + 1)
	if s.state.LastDir < 0 && s.state.SheetsLeft(-1) <= keepNear {
		s.ensureLoad(s.cur - 1)
	}
	for _, dir := range []int{1, -1} {
		pos := s.cur + dir
		c, ok := s.chapters[pos]
		if !ok {
			continue
		}
		if s.state.SheetsLeft(dir) > keepNear {
			c.pf.Want(nil)
			continue
		}
		ns := s.neighborState(pos, c, dir)
		t := ns.Target(g)
		want := []viewer.Target{t}
		if ws := ns.WarmSet(t); len(ws) > 0 {
			want = append(want, ws[0])
		}
		c.pf.Want(want)
	}
}

// setToast shows msg on the top row over the page. d == 0 keeps it until
// replaced or cleared.
func (s *session) setToast(msg string, d time.Duration) {
	s.toast = msg
	if s.toastT != nil {
		s.toastT.Stop()
		s.toastT = nil
	}
	if d > 0 {
		s.toastT = time.NewTimer(d)
	}
	if s.shown != nil {
		s.write(s.toastBytes(nil))
	}
}

// clearToast removes the toast by repainting the shown frame.
func (s *session) clearToast() {
	if s.toast == "" {
		return
	}
	s.toast = ""
	if s.toastT != nil {
		s.toastT.Stop()
		s.toastT = nil
	}
	s.repaint = true
}

func (s *session) toastC() <-chan time.Time {
	if s.toastT == nil {
		return nil
	}
	return s.toastT.C
}

func (s *session) toastBytes(out []byte) []byte {
	if s.toast == "" {
		return out
	}
	msg := []rune(" " + s.toast + " ")
	if cols := s.size().Cols; cols > 0 && len(msg) > cols {
		msg = msg[:cols]
	}
	out = append(out, "\x1b[1;1H\x1b[0;7m"...)
	out = append(out, string(msg)...)
	out = append(out, "\x1b[0m"...)
	return out
}
