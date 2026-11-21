// Package viewer is the reader's state machine (page, spread, direction,
// tone, rotation) and the image-id cache that keeps the terminal's image
// store under its budgets. Ported from pdfpane's Viewer.swift.
package viewer

import (
	"fmt"
	"math"

	"github.com/hongnoul/mangaz/reader/render"
)

// Grid is the placement cell span and raster pixel size for a viewport.
type Grid struct {
	Cols, Rows int
	PxW, PxH   int
}

// Viewport is the terminal pane: cell grid plus cell pixel size.
type Viewport struct {
	Cols, Rows   int
	CellW, CellH int
}

// Budgets (pdfpane): gwae drops transfers silently past 64 images / 128
// MiB per pane, and decoded RGBA past 32 MiB or 16384 px per axis.
const (
	MaxLiveImages = 48
	MaxLiveBytes  = 80 << 20
	maxRGBABytes  = 32 << 20
	maxAxis       = 16384
	minID         = 10
	maxID         = 249
)

// GridFor is the pane-covering layout: placement = whole pane (clamped to
// the 256-cell diacritics limit), raster = pane pixels at retina 2x,
// clamped under the terminal's decoded-size budget with a small margin.
func GridFor(vp Viewport) Grid {
	cols := clampInt(vp.Cols, 1, 256)
	rows := clampInt(vp.Rows, 1, 256)
	w := float64(cols*vp.CellW) * 2
	h := float64(rows*vp.CellH) * 2
	scale := 1.0
	if s := math.Sqrt(maxRGBABytes * 0.995 / 4 / (w * h)); s < scale {
		scale = s
	}
	if s := maxAxis * 0.995 / math.Max(w, h); s < scale {
		scale = s
	}
	return Grid{Cols: cols, Rows: rows,
		PxW: maxInt(int(w*scale), 1), PxH: maxInt(int(h*scale), 1)}
}

// Target is one frame: everything that changes the pixels.
type Target struct {
	Page     int // 0-based first page of the sheet
	Grid     Grid
	Rotation int
	Tone     render.Tone
	Spread   bool
	RTL      bool
}

// Key is the cache key shared by the id map, the prefetcher, and the
// transport: two tones/rotations/directions of a page are different
// pixels, exactly like two raster sizes.
func (t Target) Key() string {
	s := fmt.Sprintf("%d@%dx%dr%dt%d", t.Page, t.Grid.PxW, t.Grid.PxH, t.Rotation, t.Tone)
	if t.Spread {
		s += "s"
		if t.RTL {
			s += "r" // RTL swaps the pair's sides
		}
	}
	return s
}

// State is the reader's view state.
type State struct {
	Pages    int
	Page     int // 0-based
	Rotation int
	Tone     render.Tone
	Spread   bool
	RTL      bool
	// LastDir is the last flip's direction (+1/-1), seeding the
	// prefetcher's reading-order bias.
	LastDir int
}

// NewState starts at page 0 with the mangaz defaults: dark, RTL.
func NewState(pages int) *State {
	return &State{Pages: pages, Tone: render.Dark, RTL: true, LastDir: 1}
}

// Stride is pages per flip: two in spread view, like a physical book.
func (s *State) Stride() int {
	if s.Spread {
		return 2
	}
	return 1
}

// SheetStart normalizes a page to the first page of its sheet. Spread
// view puts the cover (page 0) alone, then pairs (1,2), (3,4), ... as in
// a printed volume.
func (s *State) SheetStart(p int) int {
	p = clampInt(p, 0, s.Pages-1)
	if s.Spread && p > 0 && p%2 == 0 {
		p--
	}
	return p
}

// SheetPages lists the page indices of the sheet starting at p, in
// DISPLAY order (left to right). RTL puts the later page on the left.
func (s *State) SheetPages(p int) []int {
	p = s.SheetStart(p)
	if !s.Spread || p == 0 || p+1 >= s.Pages {
		return []int{p}
	}
	if s.RTL {
		return []int{p + 1, p}
	}
	return []int{p, p + 1}
}

// Flip moves delta sheets in reading order, clamped.
func (s *State) Flip(delta int) {
	if delta == 0 {
		return
	}
	if delta > 0 {
		s.LastDir = 1
	} else {
		s.LastDir = -1
	}
	p := s.SheetStart(s.Page)
	for i := 0; i < abs(delta); i++ {
		if delta > 0 {
			p = s.nextSheet(p)
		} else {
			p = s.prevSheet(p)
		}
	}
	s.Page = p
}

func (s *State) nextSheet(p int) int {
	n := p + 1
	if s.Spread && p > 0 {
		n = p + 2
	}
	if n >= s.Pages {
		return p
	}
	return n
}

func (s *State) prevSheet(p int) int {
	if p == 0 {
		return 0
	}
	if s.Spread {
		return s.SheetStart(p - 1)
	}
	return p - 1
}

// Physical maps a physical left/right press to a reading-order flip:
// in RTL, left is forward (the next page is on the left).
func (s *State) Physical(left bool) {
	if left == s.RTL {
		s.Flip(1)
	} else {
		s.Flip(-1)
	}
}

// GoTo jumps to an absolute 0-based page.
func (s *State) GoTo(p int) {
	p = s.SheetStart(p)
	if p > s.Page {
		s.LastDir = 1
	} else if p < s.Page {
		s.LastDir = -1
	}
	s.Page = p
}

// ToggleSpread switches single/spread, keeping the current page visible.
func (s *State) ToggleSpread() {
	s.Spread = !s.Spread
	s.Page = s.SheetStart(s.Page)
}

// Rotate turns the view by delta quarter turns clockwise.
func (s *State) Rotate(delta int) { s.Rotation = ((s.Rotation+delta)%4 + 4) % 4 }

// Target is the frame for the current state at grid g.
func (s *State) Target(g Grid) Target {
	return s.TargetAt(s.Page, g)
}

// TargetAt is the frame for the sheet at page p.
func (s *State) TargetAt(p int, g Grid) Target {
	return Target{Page: s.SheetStart(p), Grid: g, Rotation: s.Rotation,
		Tone: s.Tone, Spread: s.Spread, RTL: s.RTL}
}

// WarmSet is the prefetch set around t: 4 sheets ahead along the last
// flip's direction and 1 behind (pdfpane: sustained scroll outruns a
// symmetric set, and pages just passed are usually still cached).
func (s *State) WarmSet(t Target) []Target {
	var out []Target
	seen := map[int]bool{t.Page: true}
	add := func(p int) {
		if !seen[p] {
			seen[p] = true
			n := t
			n.Page = p
			out = append(out, n)
		}
	}
	fwd, back := s.nextSheet, s.prevSheet
	if s.LastDir < 0 {
		fwd, back = back, fwd
	}
	p := t.Page
	for i := 0; i < 4; i++ {
		p = fwd(p)
		add(p)
	}
	add(back(t.Page))
	return out
}

// Cache maps frame keys to terminal image ids with LRU eviction under
// MaxLiveImages / MaxLiveBytes. Ids stay in 10..249 (SGR 38;5 indexed
// colors, and gwae rejects 0).
type Cache struct {
	idFor   map[string]int
	lru     []string
	bytesOf map[int]int
	// Displayed is protected from eviction: it is on screen, and the
	// resize stretch uses it as its source.
	Displayed string
}

// NewCache returns an empty cache.
func NewCache() *Cache {
	return &Cache{idFor: map[string]int{}, bytesOf: map[int]int{}}
}

// Lookup returns the id for key without allocating.
func (c *Cache) Lookup(key string) (int, bool) {
	id, ok := c.idFor[key]
	return id, ok
}

// ID returns the id for key (allocating one if needed) and ids the caller
// must delete from the terminal before transmitting anything new.
func (c *Cache) ID(key string) (int, []int) {
	c.touch(key)
	if id, ok := c.idFor[key]; ok {
		return id, c.evict(0)
	}
	evicted := c.evict(1)
	used := map[int]bool{}
	for _, id := range c.idFor {
		used[id] = true
	}
	id := minID
	for used[id] && id < maxID {
		id++
	}
	c.idFor[key] = id
	return id, evicted
}

// Transmitted records id's retained footprint (base64 PNG + overhead) and
// returns ids to delete to stay under budget.
func (c *Cache) Transmitted(id, pngBytes int) []int {
	c.bytesOf[id] = pngBytes/3*4 + 1024
	return c.evict(0)
}

func (c *Cache) touch(key string) {
	for i, k := range c.lru {
		if k == key {
			c.lru = append(c.lru[:i], c.lru[i+1:]...)
			break
		}
	}
	c.lru = append(c.lru, key)
}

func (c *Cache) total() int {
	t := 0
	for _, b := range c.bytesOf {
		t += b
	}
	return t
}

func (c *Cache) evict(incoming int) []int {
	var out []int
	for len(c.idFor)+incoming > MaxLiveImages || c.total() > MaxLiveBytes {
		victim := -1
		for i, k := range c.lru {
			// Never the newest (being shown now) or the displayed frame.
			if k != c.Displayed && i != len(c.lru)-1 {
				victim = i
				break
			}
		}
		if victim < 0 {
			break
		}
		k := c.lru[victim]
		c.lru = append(c.lru[:victim], c.lru[victim+1:]...)
		if id, ok := c.idFor[k]; ok {
			delete(c.idFor, k)
			delete(c.bytesOf, id)
			out = append(out, id)
		}
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
