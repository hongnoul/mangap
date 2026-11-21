package viewer

import (
	"bytes"
	"image"
	"image/png"
	"testing"
	"time"
)

func TestFlipSingleAndClamp(t *testing.T) {
	s := NewState(5)
	s.Flip(1)
	s.Flip(1)
	if s.Page != 2 {
		t.Fatalf("page %d want 2", s.Page)
	}
	s.Flip(10)
	if s.Page != 4 {
		t.Fatalf("clamped page %d want 4", s.Page)
	}
	s.Flip(-10)
	if s.Page != 0 {
		t.Fatalf("clamped page %d want 0", s.Page)
	}
}

func TestSpreadCoverAloneThenPairs(t *testing.T) {
	s := NewState(6) // pages 0..5: [0] [1 2] [3 4] [5]
	s.ToggleSpread()
	var sheets [][]int
	for i := 0; i < 5; i++ {
		sheets = append(sheets, s.SheetPages(s.Page))
		s.Flip(1)
	}
	// RTL default: later page on the left.
	want := [][]int{{0}, {2, 1}, {4, 3}, {5}, {5}}
	for i := range want {
		if len(sheets[i]) != len(want[i]) || sheets[i][0] != want[i][0] {
			t.Fatalf("sheets %v want %v", sheets, want)
		}
	}
	s.Flip(-1)
	if s.Page != 3 {
		t.Fatalf("back from last sheet: page %d want 3", s.Page)
	}
	s.RTL = false
	if p := s.SheetPages(3); p[0] != 3 || p[1] != 4 {
		t.Fatalf("LTR pair %v want [3 4]", p)
	}
}

func TestToggleSpreadKeepsPage(t *testing.T) {
	s := NewState(10)
	s.GoTo(4)
	s.ToggleSpread()
	if pp := s.SheetPages(s.Page); pp[0] != 4 && pp[1] != 4 {
		t.Fatalf("page 4 not visible after spread on: %v", pp)
	}
}

func TestPhysicalKeysFollowDirection(t *testing.T) {
	s := NewState(10)
	s.Physical(true) // RTL: left = next
	if s.Page != 1 {
		t.Fatalf("RTL left -> page %d want 1", s.Page)
	}
	s.RTL = false
	s.Physical(true) // LTR: left = prev
	if s.Page != 0 {
		t.Fatalf("LTR left -> page %d want 0", s.Page)
	}
}

func TestWarmSetBiased(t *testing.T) {
	s := NewState(20)
	s.GoTo(10)
	s.Flip(1)
	ws := s.WarmSet(s.Target(Grid{}))
	got := []int{}
	for _, w := range ws {
		got = append(got, w.Page)
	}
	want := []int{12, 13, 14, 15, 10}
	if len(got) != len(want) {
		t.Fatalf("warm %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("warm %v want %v", got, want)
		}
	}
}

func TestGridClampsBudget(t *testing.T) {
	g := GridFor(Viewport{Cols: 300, Rows: 100, CellW: 20, CellH: 40})
	if g.Cols != 256 || g.Rows != 100 {
		t.Fatalf("cells %dx%d", g.Cols, g.Rows)
	}
	if g.PxW*g.PxH*4 > maxRGBABytes || g.PxW > maxAxis || g.PxH > maxAxis {
		t.Fatalf("raster %dx%d over budget", g.PxW, g.PxH)
	}
}

func TestCacheEvictsLRUButNotDisplayed(t *testing.T) {
	c := NewCache()
	first, _ := c.ID("k0")
	c.Displayed = "k0"
	var evicted []int
	for i := 1; i <= MaxLiveImages; i++ {
		_, ev := c.ID("k" + string(rune('A'+i)))
		evicted = append(evicted, ev...)
	}
	if len(evicted) != 1 {
		t.Fatalf("evicted %v, want exactly one", evicted)
	}
	if evicted[0] == first {
		t.Fatal("evicted the displayed frame")
	}
	if _, ok := c.Lookup("k0"); !ok {
		t.Fatal("displayed key lost")
	}
}

func TestCacheByteBudget(t *testing.T) {
	c := NewCache()
	id1, _ := c.ID("a")
	c.Transmitted(id1, MaxLiveBytes/2)
	id2, _ := c.ID("b")
	ev := c.Transmitted(id2, MaxLiveBytes/2)
	if len(ev) != 1 || ev[0] != id1 {
		t.Fatalf("evicted %v, want [%d]", ev, id1)
	}
}

type memSource [][]byte

func (m memSource) Len() int                   { return len(m) }
func (m memSource) Name(i int) string          { return "p" }
func (m memSource) Read(i int) ([]byte, error) { return m[i], nil }
func (m memSource) Close() error               { return nil }

func TestPrefetcherRendersDemandAndSignals(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewGray(image.Rect(0, 0, 30, 40)))
	src := memSource{buf.Bytes(), buf.Bytes(), buf.Bytes()}
	p := NewPrefetcher(src, 3)
	defer p.Close()
	s := NewState(3)
	tgt := s.Target(Grid{Cols: 10, Rows: 10, PxW: 60, PxH: 80})
	p.Demand([]Target{tgt}, s.WarmSet(tgt))
	select {
	case <-p.Ready:
	case <-time.After(5 * time.Second):
		t.Fatal("no ready signal")
	}
	deadline := time.Now().Add(5 * time.Second)
	for p.Take(tgt) == nil {
		if time.Now().After(deadline) {
			t.Fatal("demanded frame never landed")
		}
		<-p.Ready
	}
	// Want a different set: the old result is dropped.
	other := s.TargetAt(2, tgt.Grid)
	p.ClearAwait()
	p.Want([]Target{other})
	if p.Take(tgt) != nil {
		t.Fatal("unwanted result kept")
	}
}
