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
	"github.com/hongnoul/mangaz/constant"
	"github.com/hongnoul/mangaz/network"
	"github.com/hongnoul/mangaz/provider/generic"
	"github.com/hongnoul/mangaz/reader/kitty"
	"github.com/hongnoul/mangaz/reader/render"
	"github.com/hongnoul/mangaz/reader/term"
	"github.com/hongnoul/mangaz/source"
	"github.com/hongnoul/mangaz/where"
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
	previewPx = 24 // long edge of the low-res preview
	cacheDir  = "thumbs"
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
func (s *syncOut) Read(p []byte) (int, error) { return s.f.Read(p) }
func (s *syncOut) Fd() uintptr                { return s.f.Fd() }

// thumbReadyMsg asks for a repaint once a thumbnail reached the terminal.
type thumbReadyMsg struct{}

// thumbState is one thumbnail's progress.
type thumbState struct {
	id   int  // kitty image id, 0 until something was transmitted
	full bool // the full-res raster replaced the preview
	busy bool
	fail bool
	// queuedVisible: a job sits in the visible lane. started: a worker
	// claimed the load, so later duplicate jobs drop out.
	queuedVisible bool
	started       bool
	img           image.Image // decoded source, kept to re-raster on size change
	cols          int         // span the image was transmitted for
	rows          int
}

// thumbs fetches manga covers and chapter first pages and shows them as
// kitty images: a low-res preview first, then the full raster under the
// same id. Sources are cached on disk, and visible rows always jump the
// queue ahead of prefetches.
type thumbs struct {
	enabled bool
	out     *syncOut
	send    func(tea.Msg)

	mu    sync.Mutex
	state map[string]*thumbState
	next  int

	// Two lanes: on-screen work first, prefetch only with spare capacity.
	visible  chan struct{}
	prefetch chan struct{}
	// Cell span every thumbnail is drawn at; set by the layout.
	cols, rows int
}

func newThumbs(out *syncOut) *thumbs {
	return &thumbs{
		enabled:  out != nil && term.SupportsKittyGraphics(),
		out:      out,
		state:    map[string]*thumbState{},
		next:     thumbBaseID,
		visible:  make(chan struct{}, 16),
		prefetch: make(chan struct{}, 6),
		cols:     6,
		rows:     4,
	}
}

// SetSpan changes the thumbnail cell box. Transmitted images are
// re-spanned in the terminal (zero bytes), and full rasters re-render in
// the background at the new pixel size.
func (t *thumbs) SetSpan(cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cols == t.cols && rows == t.rows {
		return
	}
	t.cols, t.rows = cols, rows
	for key, st := range t.state {
		if st.id == 0 || st.img == nil {
			continue
		}
		st.full = false
		img := st.img
		go t.raster(key, img, false)
	}
}

func coverURL(m *source.Manga) string {
	u, _ := m.GetCover()
	return u
}

// WantManga requests a manga's cover. visible rows are fetched first.
func (t *thumbs) WantManga(m *source.Manga, visible bool) {
	u, ref := coverURL(m), m.URL
	if u == "" {
		return
	}
	t.want(u, visible, func() (string, string, error) { return u, ref, nil })
}

// WantChapter requests a chapter's first page. Scraper sources only:
// MangaDex would download the whole chapter to learn its first page.
func (t *thumbs) WantChapter(c *source.Chapter, visible bool) {
	if c == nil || c.Manga == nil || c.URL == "" {
		return
	}
	sc, ok := c.Source().(*generic.Scraper)
	if !ok {
		return
	}
	t.want(c.URL, visible, func() (string, string, error) {
		u, err := sc.FirstPageURL(c)
		return u, c.URL, err
	})
}

// want schedules a thumbnail once. Safe to call on every frame.
func (t *thumbs) want(key string, visible bool, locate func() (url, referer string, err error)) {
	if !t.enabled || key == "" {
		return
	}
	t.mu.Lock()
	st := t.state[key]
	if st != nil && (st.id != 0 || st.fail) {
		t.mu.Unlock()
		return
	}
	if st != nil && st.busy {
		// Queued as a prefetch but now on screen (the user paged onto
		// it): promote it with a second job in the visible lane, which
		// races the stale one; whichever starts first does the load.
		if !visible || st.queuedVisible || st.started {
			t.mu.Unlock()
			return
		}
	}
	if st == nil {
		st = &thumbState{}
		t.state[key] = st
	}
	st.busy = true
	if visible {
		st.queuedVisible = true
	}
	t.mu.Unlock()

	lane := t.prefetch
	if visible {
		lane = t.visible
	}
	go func() {
		lane <- struct{}{}
		defer func() { <-lane }()
		t.mu.Lock()
		if st.started {
			t.mu.Unlock()
			return
		}
		st.started = true
		t.mu.Unlock()
		t.load(key, locate)
	}()
}

func cachePath(key string) string {
	h := sha1.Sum([]byte(key))
	return filepath.Join(where.Cache(), cacheDir, hex.EncodeToString(h[:]))
}

func (t *thumbs) load(key string, locate func() (string, string, error)) {
	fail := func() {
		t.mu.Lock()
		if st := t.state[key]; st != nil {
			st.busy, st.fail = false, true
		}
		t.mu.Unlock()
	}

	path := cachePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		// One retry: a transient error would otherwise blank the card
		// for the rest of the session.
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
			fail()
			return
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, data, 0o644)
	}
	img, err := render.Decode(data)
	if err != nil {
		_ = os.Remove(path)
		fail()
		return
	}

	t.mu.Lock()
	if st := t.state[key]; st != nil {
		st.img = img
	}
	t.mu.Unlock()

	// Preview first, then the full raster under the same id.
	t.raster(key, img, true)
	t.raster(key, img, false)

	t.mu.Lock()
	if st := t.state[key]; st != nil {
		st.busy = false
	}
	t.mu.Unlock()
}

// raster renders img at preview or full size and transmits it, replacing
// whatever the id held before.
func (t *thumbs) raster(key string, img image.Image, preview bool) {
	t.mu.Lock()
	cols, rows := t.cols, t.rows
	st := t.state[key]
	if st == nil {
		t.mu.Unlock()
		return
	}
	id := st.id
	if id == 0 {
		id = t.next
		t.next++
	}
	t.mu.Unlock()

	sz := term.GetSize(t.out.f)
	w, h := cols*sz.CellW, rows*sz.CellH
	if preview {
		// Same aspect as the box, long edge previewPx.
		if w >= h {
			w, h = previewPx, imax(previewPx*h/imax(w, 1), 1)
		} else {
			w, h = imax(previewPx*w/imax(h, 1), 1), previewPx
		}
	}
	res, err := render.Render(render.Job{Pages: []image.Image{img}, W: w, H: h})
	if err != nil {
		return
	}

	// Free the old raster of this id, then send the new one, in one write.
	var buf []byte
	if st.id != 0 {
		buf = append(buf, "\x1b_Ga=d,d=I,i="...)
		buf = strconv.AppendInt(buf, int64(id), 10)
		buf = append(buf, ",q=2\x1b\\"...)
	}
	buf, _ = kitty.New().Transmit(buf, res.PNG, res.W, res.H, id, cols, rows)
	if _, err := t.out.Write(buf); err != nil {
		return
	}

	t.mu.Lock()
	st.id = id
	st.cols, st.rows = cols, rows
	if !preview {
		st.full = true
	}
	t.mu.Unlock()
	if t.send != nil {
		t.send(thumbReadyMsg{})
	}
}

func get(u, referer string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", constant.UserAgent)
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
	defer m.out.setMuted(false)
	return m.ExecCommand.Run()
}

// Retain frees every transmitted image whose key is not in keep. The
// terminal's image store is finite (Ghostty evicts or refuses uploads once
// it is full), so only the pages around the cursor stay resident. Sources
// remain on disk and reload in milliseconds when paged back to.
func (t *thumbs) Retain(keep map[string]bool) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	var buf []byte
	for key, st := range t.state {
		if keep[key] || st.busy {
			continue
		}
		if st.id != 0 {
			buf = append(buf, "\x1b_Ga=d,d=I,i="...)
			buf = strconv.AppendInt(buf, int64(st.id), 10)
			buf = append(buf, ",q=2\x1b\\"...)
		}
		delete(t.state, key)
	}
	t.mu.Unlock()
	if len(buf) > 0 {
		_, _ = t.out.Write(buf)
	}
}
