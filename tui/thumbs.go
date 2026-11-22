package tui

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hongnoul/mangap/constant"
	"github.com/hongnoul/mangap/network"
	"github.com/hongnoul/mangap/provider/generic"
	"github.com/hongnoul/mangap/reader/kitty"
	"github.com/hongnoul/mangap/reader/render"
	"github.com/hongnoul/mangap/reader/term"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/where"
)

// Thumbnail image ids start well above the reader's 10..249 range and are
// addressed with 24-bit SGR, so the two never collide.
const thumbBaseID = 1 << 16

// previewBaseID keeps the preview pane's ids apart from the list's.
const previewBaseID = 1 << 22

// Raster sizes. The preview is tiny (a few KB of PNG) so it reaches the
// terminal almost as soon as the bytes arrive; the terminal stretches it
// to the cell box. The full raster then replaces it under the same id.
const (
	lowPx    = 48 // long edge of the low-res raster
	cacheDir = "thumbs"
)

// syncOut serializes writes to the terminal: bubbletea's frames and the
// background thumbnail transfers share stdout, and an interleaved escape
// sequence would corrupt both.
type syncOut struct {
	mu sync.Mutex
	f  *os.File
	// muted drops writes while another program (the reader) owns the
	// terminal. Without it bubbletea's ticker repaints the TUI onto the
	// main screen, which is left behind as garbage after quitting.
	muted bool
}

func (s *syncOut) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.muted {
		return len(p), nil
	}
	return s.f.Write(p)
}

func (s *syncOut) setMuted(m bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.muted = m
	s.mu.Unlock()
}

// unmuteAlt switches to the alt screen and unmutes atomically, so no
// write can slip onto the main screen in between.
func (s *syncOut) unmuteAlt() {
	if s == nil {
		return
	}
	s.mu.Lock()
	_, _ = s.f.Write([]byte("\x1b[?1049h\x1b[2J\x1b[H"))
	s.muted = false
	s.mu.Unlock()
}

func (s *syncOut) Read(p []byte) (int, error) { return s.f.Read(p) }
func (s *syncOut) Fd() uintptr                { return s.f.Fd() }

// thumbReadyMsg asks for a repaint once a thumbnail reached the terminal.
type thumbReadyMsg struct{}

// thumbState is one thumbnail's progress.
type thumbState struct {
	id    int // kitty image id, 0 until something was transmitted
	level int // 0 nothing shown, 1 low-res, 2 full-res
	fail  bool
	// low is the low-res PNG, kept (a few KB) so a full raster can be
	// swapped back to it without decoding the source again.
	low        []byte
	lowW, lowH int
	// Scheduling: busy while a job is queued or running, queued is the
	// best priority queued so far, running once a worker took it. seq
	// bumps when a job ends so stale duplicates in slower lanes drop out.
	busy    bool
	running bool
	queued  int
	seq     int
	cols    int // span the image was transmitted for
	rows    int
}

// Priorities, lowest first. sweep fills in low-res for the whole list.
const (
	prioSweep = iota + 1
	prioNear
	prioVisible
)

// thumbs fetches manga covers and chapter first pages and shows them as
// kitty images: a low-res preview first, then the full raster under the
// same id. Sources are cached on disk, and visible rows always jump the
// queue ahead of prefetches.
type thumbs struct {
	enabled bool
	// fill center-crops images to the cell box instead of letterboxing
	// (list rows); the preview pane keeps the whole page.
	fill bool
	out  *syncOut
	send func(tea.Msg)

	mu    sync.Mutex
	state map[string]*thumbState
	next  int

	// One worker pool per priority: on-screen first, then the pages
	// around it, then the low-res sweep of everything else.
	lanes [prioVisible + 1]chan struct{}
	// hot keys get the full raster; everything else stays low-res.
	hot map[string]bool
	// Cell span every thumbnail is drawn at; set by the layout.
	cols, rows int
}

func newThumbs(out *syncOut) *thumbs {
	return &thumbs{
		enabled: out != nil && term.SupportsKittyGraphics(),
		out:     out,
		state:   map[string]*thumbState{},
		next:    thumbBaseID,
		lanes: [prioVisible + 1]chan struct{}{
			prioSweep:   make(chan struct{}, 12),
			prioNear:    make(chan struct{}, 6),
			prioVisible: make(chan struct{}, 16),
		},
		hot:  map[string]bool{},
		cols: 6,
		rows: 4,
	}
}

// SetSpan changes the thumbnail cell box. Transmitted images are
// re-spanned in the terminal (zero bytes), and full rasters re-render in
// the background at the new pixel size.
func (t *thumbs) SetSpan(cols, rows int) {
	t.mu.Lock()
	if cols == t.cols && rows == t.rows {
		t.mu.Unlock()
		return
	}
	t.cols, t.rows = cols, rows
	// Re-send every low-res raster at the new span (cheap, cached bytes).
	// Hot ones are promoted to full again by the next frame's want.
	var buf []byte
	for _, st := range t.state {
		if st.level == 0 || st.low == nil {
			continue
		}
		st.level = 1
		buf = t.appendLow(buf, st)
	}
	t.mu.Unlock()
	t.write(buf)
}

func coverURL(m *source.Manga) string {
	u, _ := m.GetCover()
	return u
}

// WantManga requests a manga's cover. visible rows are fetched first.
func (t *thumbs) WantManga(m *source.Manga, prio int) {
	u, ref := coverURL(m), m.URL
	if u == "" {
		return
	}
	t.want(u, prio, func() (string, string, error) { return u, ref, nil })
}

// WantChapter requests a chapter's first page. Scraper sources only:
// MangaDex would download the whole chapter to learn its first page.
func (t *thumbs) WantChapter(c *source.Chapter, prio int) {
	if c == nil || c.Manga == nil || c.URL == "" {
		return
	}
	sc, ok := c.Source().(*generic.Scraper)
	if !ok {
		return
	}
	t.want(c.URL, prio, func() (string, string, error) {
		u, err := sc.FirstPageURL(c)
		return u, c.URL, err
	})
}

// want schedules whatever key still needs: the low-res raster if nothing
// is shown, the full one if key is hot. Safe to call on every frame.
func (t *thumbs) want(key string, prio int, locate func() (url, referer string, err error)) {
	if !t.enabled || key == "" {
		return
	}
	t.mu.Lock()
	st := t.state[key]
	if st == nil {
		st = &thumbState{}
		t.state[key] = st
	}
	need := !st.fail && (st.level == 0 || (st.level == 1 && t.hot[key]))
	if !need || (st.busy && (st.running || prio <= st.queued)) {
		t.mu.Unlock()
		return
	}
	// Either idle, or queued in a slower lane: add a job here too and let
	// whichever starts first do the work.
	st.busy, st.queued = true, prio
	seq := st.seq
	lane := t.lanes[prio]
	t.mu.Unlock()

	go func() {
		lane <- struct{}{}
		defer func() { <-lane }()
		t.mu.Lock()
		// Dropped by Clear (list replaced), superseded, or already taken.
		if t.state[key] != st || st.seq != seq || st.running || !st.busy {
			t.mu.Unlock()
			return
		}
		st.running = true
		t.mu.Unlock()

		t.work(key, st, locate)

		t.mu.Lock()
		st.busy, st.running, st.queued = false, false, 0
		st.seq++
		t.mu.Unlock()
		if t.send != nil {
			t.send(thumbReadyMsg{})
		}
	}()
}

func cachePath(key string) string {
	h := sha1.Sum([]byte(key))
	return filepath.Join(where.Cache(), cacheDir, hex.EncodeToString(h[:]))
}

// fetchSource returns the image bytes for key, from the disk cache or the
// network (with one retry, so a blip doesn't blank the card for good).
func fetchSource(key string, locate func() (string, string, error)) ([]byte, error) {
	path := cachePath(key)
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}
	var data []byte
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		var u, ref string
		if u, ref, err = locate(); err == nil {
			data, err = get(u, ref)
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o644)
	return data, nil
}

func (t *thumbs) work(key string, st *thumbState, locate func() (string, string, error)) {
	data, err := fetchSource(key, locate)
	var img image.Image
	if err == nil {
		if img, err = render.Decode(data); err != nil {
			_ = os.Remove(cachePath(key))
		}
	}
	if err != nil {
		t.mu.Lock()
		st.fail = true
		t.mu.Unlock()
		return
	}

	t.mu.Lock()
	needLow := st.level == 0
	t.mu.Unlock()
	if needLow {
		t.sendLow(st, img)
	}
	t.mu.Lock()
	hot := t.hot[key]
	t.mu.Unlock()
	if hot {
		t.sendFull(st, img)
	}
}

// pixels is the raster size for the current span; low shrinks it so the
// long edge is lowPx (the terminal stretches it to the cells).
func (t *thumbs) pixels(cols, rows int, low bool) (int, int) {
	sz := term.GetSize(t.out.f)
	w, h := cols*sz.CellW, rows*sz.CellH
	if low {
		if w >= h {
			w, h = lowPx, imax(lowPx*h/imax(w, 1), 1)
		} else {
			w, h = imax(lowPx*w/imax(h, 1), 1), lowPx
		}
	}
	return w, h
}

func (t *thumbs) sendLow(st *thumbState, img image.Image) {
	t.mu.Lock()
	cols, rows := t.cols, t.rows
	t.mu.Unlock()
	w, h := t.pixels(cols, rows, true)
	res, err := render.Render(render.Job{Pages: []image.Image{t.fit(img, w, h)}, W: w, H: h})
	if err != nil {
		return
	}
	t.mu.Lock()
	st.low, st.lowW, st.lowH = res.PNG, res.W, res.H
	if st.level == 0 {
		st.level = 1
	}
	buf := t.appendLow(nil, st)
	t.mu.Unlock()
	t.write(buf)
}

func (t *thumbs) sendFull(st *thumbState, img image.Image) {
	t.mu.Lock()
	cols, rows := t.cols, t.rows
	t.mu.Unlock()
	w, h := t.pixels(cols, rows, false)
	res, err := render.Render(render.Job{Pages: []image.Image{t.fit(img, w, h)}, W: w, H: h})
	if err != nil {
		return
	}
	t.mu.Lock()
	if cols != t.cols || rows != t.rows {
		t.mu.Unlock()
		return // resized meanwhile; the next frame asks again
	}
	buf := t.appendTransmit(nil, st, res.PNG, res.W, res.H)
	st.level = 2
	t.mu.Unlock()
	t.write(buf)
}

func (t *thumbs) fit(img image.Image, w, h int) image.Image {
	if !t.fill {
		return img
	}
	return cropToFill(img, w, h)
}

// cropToFill center-crops img to the w:h aspect so the fitted raster fills
// the whole thumbnail box (no letterbox bars next to the row highlight).
func cropToFill(img image.Image, w, h int) image.Image {
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	b := img.Bounds()
	if !ok || w <= 0 || h <= 0 || b.Dx() <= 0 || b.Dy() <= 0 {
		return img
	}
	if b.Dx()*h > b.Dy()*w {
		// Too wide: trim the sides.
		nw := b.Dy() * w / h
		x := b.Min.X + (b.Dx()-nw)/2
		return sub.SubImage(image.Rect(x, b.Min.Y, x+nw, b.Max.Y))
	}
	if b.Dx()*h < b.Dy()*w {
		// Too tall: trim top and bottom.
		nh := b.Dx() * h / w
		y := b.Min.Y + (b.Dy()-nh)/2
		return sub.SubImage(image.Rect(b.Min.X, y, b.Max.X, y+nh))
	}
	return img
}

// appendLow appends st's low-res raster at the current span. Caller holds mu.
func (t *thumbs) appendLow(buf []byte, st *thumbState) []byte {
	return t.appendTransmit(buf, st, st.low, st.lowW, st.lowH)
}

// appendTransmit appends a transmit of png under st's id, freeing what the
// id held before. Caller holds mu.
func (t *thumbs) appendTransmit(buf []byte, st *thumbState, png []byte, w, h int) []byte {
	if st.id == 0 {
		st.id = t.next
		t.next++
	} else {
		buf = append(buf, "\x1b_Ga=d,d=I,i="...)
		buf = strconv.AppendInt(buf, int64(st.id), 10)
		buf = append(buf, ",q=2\x1b\\"...)
	}
	buf, _ = kitty.New().Transmit(buf, png, w, h, st.id, t.cols, t.rows)
	st.cols, st.rows = t.cols, t.rows
	return buf
}

func (t *thumbs) write(buf []byte) {
	if len(buf) > 0 && t.out != nil {
		_, _ = t.out.Write(buf)
	}
}

func get(u, referer string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", constant.UserAgentFor(u))
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := network.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", u, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}

// Span is the current thumbnail cell box.
func (t *thumbs) Span() (cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cols, t.rows
}

// Cells returns row r of the thumbnail for key: placeholder cells once
// anything (preview or full) is in the terminal, blanks before that.
// Always exactly cols cells wide.
func (t *thumbs) Cells(key string, r int) string {
	t.mu.Lock()
	cols := t.cols
	st := t.state[key]
	id := 0
	if st != nil && st.cols == t.cols && st.rows == t.rows {
		id = st.id
	}
	t.mu.Unlock()
	if !t.enabled || id == 0 {
		return strings.Repeat(" ", cols)
	}

	var b strings.Builder
	b.WriteString("\x1b[38;2;")
	b.WriteString(strconv.Itoa((id >> 16) & 0xff))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa((id >> 8) & 0xff))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(id & 0xff))
	b.WriteByte('m')
	var tmp [utf8.UTFMax]byte
	row := string(tmp[:utf8.EncodeRune(tmp[:], kitty.Diacritics[r])])
	for c := 0; c < cols; c++ {
		b.WriteRune(kitty.Placeholder)
		b.WriteString(row)
		b.WriteRune(kitty.Diacritics[c])
	}
	b.WriteString("\x1b[39m")
	return b.String()
}

// Clear frees every thumbnail image in the terminal. The disk cache stays.
func (t *thumbs) Clear() {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	var buf []byte
	for _, st := range t.state {
		if st.id != 0 {
			buf = append(buf, fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", st.id)...)
		}
	}
	t.state = map[string]*thumbState{}
	t.mu.Unlock()
	if len(buf) > 0 {
		_, _ = t.out.Write(buf)
	}
}

// mutedExec silences the TUI's output while cmd owns the terminal.
// tea.Exec has already left the alt screen when Run starts; any repaint
// during the read would land on the main screen and survive exit.
type mutedExec struct {
	tea.ExecCommand
	out *syncOut
}

func (m *mutedExec) Run() error {
	m.out.setMuted(true)
	err := m.ExecCommand.Run()
	// Unmuting happens before bubbletea's RestoreTerminal re-enters the
	// alt screen, so a repaint in that window would land on the main
	// screen and survive exit. Re-enter the alt screen first (bubbletea
	// doing it again is harmless).
	m.out.unmuteAlt()
	return err
}

// Retain marks keep as hot (full-res) and swaps every other full raster
// back to its low-res version. The terminal's image store is finite, so
// only the pages around the cursor hold full rasters, while the low-res
// copies (a few KB each) stay for the whole list.
func (t *thumbs) Retain(keep map[string]bool) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	t.hot = keep
	var buf []byte
	for key, st := range t.state {
		if st.level == 2 && !keep[key] && st.low != nil {
			st.level = 1
			buf = t.appendLow(buf, st)
		}
	}
	t.mu.Unlock()
	t.write(buf)
}
