package reader

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hongnoul/mangap/reader/term"
)

type memSource [][]byte

func (m memSource) Len() int                   { return len(m) }
func (m memSource) Name(i int) string          { return "p" }
func (m memSource) Read(i int) ([]byte, error) { return m[i], nil }
func (m memSource) Close() error               { return nil }

// page is a distinct gray level per page, so frames are distinguishable.
func page(level uint8) []byte {
	g := image.NewGray(image.Rect(0, 0, 60, 90))
	for i := range g.Pix {
		g.Pix[i] = level
	}
	var b bytes.Buffer
	_ = png.Encode(&b, g)
	return b.Bytes()
}

// harness runs a session over pipes and records everything it writes.
type harness struct {
	t        *testing.T
	keysW    *os.File
	mu       sync.Mutex
	out      bytes.Buffer
	done     chan struct{}
	res      Result
	err      error
	restored []byte
	size     term.Size
	resize   chan struct{}
}

func start(t *testing.T, n int, opt Options) *harness {
	t.Helper()
	src := memSource{}
	for i := 0; i < n; i++ {
		src = append(src, page(uint8(20+i*10)))
	}
	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()
	h := &harness{t: t, keysW: inW, done: make(chan struct{}),
		size:   term.Size{Cols: 40, Rows: 20, CellW: 8, CellH: 16},
		resize: make(chan struct{}, 1)}
	opt.In, opt.Out = inR, outW
	s := newSession(src, opt)
	s.acquire = func() (func([]byte), error) {
		return func(b []byte) {
			h.mu.Lock()
			h.restored = append([]byte(nil), b...)
			h.mu.Unlock()
		}, nil
	}
	s.size = func() term.Size { h.mu.Lock(); defer h.mu.Unlock(); return h.size }
	s.resizes = func() (<-chan struct{}, func()) { return h.resize, func() {} }
	go func() {
		buf := make([]byte, 1<<16)
		for {
			n, err := outR.Read(buf)
			h.mu.Lock()
			h.out.Write(buf[:n])
			h.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	go func() {
		h.res, h.err = s.run()
		_ = outW.Close()
		close(h.done)
	}()
	return h
}

func (h *harness) output() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.out.String()
}

// waitFrames waits until n synchronized frames have been written.
func (h *harness) waitFrames(n int) string {
	h.t.Helper()
	return h.waitFor(func(s string) bool { return strings.Count(s, "\x1b[?2026l") >= n })
}

func (h *harness) waitFor(ok func(string) bool) string {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := h.output(); ok(s) {
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.t.Fatalf("timed out; output has %d frames", strings.Count(h.output(), "\x1b[?2026l"))
	return ""
}

func (h *harness) press(keys string) { _, _ = io.WriteString(h.keysW, keys) }

func (h *harness) quit() {
	h.t.Helper()
	h.press("q")
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		h.t.Fatal("reader did not exit on q")
	}
}

func TestSessionFlipsTransmitsOncePerPageAndRestores(t *testing.T) {
	h := start(t, 6, Options{})
	h.waitFrames(1)
	h.press("j")
	h.waitFrames(2)
	h.press("k") // back to page 0: already transmitted, placeholder only
	s := h.waitFrames(3)
	h.quit()
	if h.err != nil {
		t.Fatal(h.err)
	}
	if got := strings.Count(s, "a=T,"); got != 2 {
		t.Fatalf("transfers = %d, want 2 (page 0, page 1; revisit is free)", got)
	}
	if h.res.Page != 0 {
		t.Fatalf("result page %d want 0", h.res.Page)
	}
	if !strings.Contains(string(h.restored), "a=d,d=I") {
		t.Fatal("restore did not free transmitted images")
	}
}

func TestSessionNetZeroBatchWritesNothing(t *testing.T) {
	h := start(t, 6, Options{})
	h.waitFrames(1)
	before := len(h.output())
	h.press("jk") // one batch that nets back to the shown page
	time.Sleep(150 * time.Millisecond)
	after := len(h.output())
	h.quit()
	if after != before {
		t.Fatalf("net-zero batch wrote %d bytes", after-before)
	}
}

func TestSessionRTLLeftIsNext(t *testing.T) {
	h := start(t, 6, Options{})
	h.waitFrames(1)
	h.press("h") // RTL default: left = next page
	h.waitFrames(2)
	h.quit()
	if h.res.Page != 1 {
		t.Fatalf("after h in RTL page = %d, want 1", h.res.Page)
	}
}

func TestSessionGotoLive(t *testing.T) {
	h := start(t, 30, Options{})
	h.waitFrames(1)
	h.press("f12")
	time.Sleep(300 * time.Millisecond)
	h.quit()
	if h.res.Page != 11 {
		t.Fatalf("f12 -> page %d, want 11 (1-based 12)", h.res.Page)
	}
}

func TestSessionResizeStretchesThenRendersCrisp(t *testing.T) {
	h := start(t, 3, Options{})
	h.waitFrames(1)
	h.mu.Lock()
	h.size = term.Size{Cols: 60, Rows: 30, CellW: 8, CellH: 16}
	h.mu.Unlock()
	h.resize <- struct{}{}
	s := h.waitFor(func(s string) bool { return strings.Count(s, "a=T,") >= 2 })
	h.quit()
	if !strings.Contains(s, "a=p,U=1,i=10,p=1,c=60,r=30") {
		t.Fatal("resize did not re-span the shown image to the new grid")
	}
	if n := strings.Count(s, "a=p,"); n != 1 {
		t.Fatalf("stretch written %d times, want once", n)
	}
}
