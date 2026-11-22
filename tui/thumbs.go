package tui

import (
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
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
)

// Thumbnail size in cells. Covers are about 2:3 and cells about 1:2, so
// 5x4 cells is close to cover-shaped.
const (
	thumbCols = 5
	thumbRows = 4
	// Thumbnail image ids start well above the reader's 10..249 range and
	// are addressed with 24-bit SGR, so the two never collide.
	thumbBaseID = 1 << 16
)

// syncOut serializes writes to the terminal: bubbletea's frames and the
// background thumbnail transfers share stdout, and an interleaved escape
// sequence would corrupt both.
type syncOut struct {
	mu sync.Mutex
	f  *os.File
}

func (s *syncOut) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Write(p)
}
func (s *syncOut) Read(p []byte) (int, error) { return s.f.Read(p) }
func (s *syncOut) Fd() uintptr                { return s.f.Fd() }

// thumbReadyMsg asks for a repaint once a cover has been transmitted.
type thumbReadyMsg struct{}

// thumbs fetches manga covers and sends them to the terminal as kitty
// images, then hands out placeholder cells that show them.
type thumbs struct {
	enabled bool
	out     *syncOut
	send    func(tea.Msg)

	mu    sync.Mutex
	ids   map[string]int  // thumbnail key -> image id
	ready map[int]bool    // transmitted
	busy  map[string]bool // fetch in flight
	next  int
	sem   chan struct{}
	// pagesMu serializes scraper page lookups: the generic scraper's
	// page cache is a plain map written from colly callbacks.
	pagesMu sync.Mutex
}

func newThumbs(out *syncOut) *thumbs {
	return &thumbs{
		enabled: out != nil && term.SupportsKittyGraphics(),
		out:     out,
		ids:     map[string]int{},
		ready:   map[int]bool{},
		busy:    map[string]bool{},
		next:    thumbBaseID,
		sem:     make(chan struct{}, 6),
	}
}

func coverURL(m *source.Manga) string {
	u, _ := m.GetCover()
	return u
}

// Load starts fetching covers for mangas that do not have one yet.
func (t *thumbs) Load(mangas []*source.Manga) {
	for _, m := range mangas {
		u, ref := coverURL(m), m.URL
		if u == "" {
			continue
		}
		t.Want(u, func() ([]byte, error) { return get(u, ref) })
	}
}

// WantChapter fetches the first page of a chapter as its thumbnail. Only
// scraper sources qualify: their page list is one small HTML request,
// while MangaDex would download the whole chapter to name its pages.
func (t *thumbs) WantChapter(c *source.Chapter) {
	if c == nil || c.Manga == nil || c.URL == "" {
		return
	}
	if _, ok := c.Source().(*generic.Scraper); !ok {
		return
	}
	t.Want(c.URL, func() ([]byte, error) {
		t.pagesMu.Lock()
		pages, err := c.Source().PagesOf(c)
		t.pagesMu.Unlock()
		if err != nil {
			return nil, err
		}
		if len(pages) == 0 || pages[0] == nil || pages[0].URL == "" {
			return nil, fmt.Errorf("no pages")
		}
		return get(pages[0].URL, c.URL)
	})
}

// Want fetches and transmits the image for key once. Safe to call on
// every render: repeat calls for a known or in-flight key are free.
func (t *thumbs) Want(key string, fetch func() ([]byte, error)) {
	if !t.enabled || key == "" {
		return
	}
	t.mu.Lock()
	_, have := t.ids[key]
	if have || t.busy[key] {
		t.mu.Unlock()
		return
	}
	t.busy[key] = true
	t.mu.Unlock()
	go t.fetch(key, fetch)
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

func (t *thumbs) fetch(key string, fetch func() ([]byte, error)) {
	t.sem <- struct{}{}
	defer func() { <-t.sem }()

	ok := false
	defer func() {
		t.mu.Lock()
		delete(t.busy, key)
		if !ok {
			t.ids[key] = 0 // remember the failure, don't refetch every frame
		}
		t.mu.Unlock()
	}()

	data, err := fetch()
	if err != nil {
		return
	}
	img, err := render.Decode(data)
	if err != nil {
		return
	}

	sz := term.GetSize(t.out.f)
	res, err := render.Render(render.Job{
		Pages: []image.Image{img},
		W:     thumbCols * sz.CellW,
		H:     thumbRows * sz.CellH,
	})
	if err != nil {
		return
	}

	t.mu.Lock()
	id := t.next
	t.next++
	t.mu.Unlock()

	var buf []byte
	buf, _ = kitty.New().Transmit(buf, res.PNG, res.W, res.H, id, thumbCols, thumbRows)
	if _, err := t.out.Write(buf); err != nil {
		return
	}

	t.mu.Lock()
	t.ids[key] = id
	t.ready[id] = true
	t.mu.Unlock()
	ok = true
	if t.send != nil {
		t.send(thumbReadyMsg{})
	}
}

// Cells returns row r (0-based) of the thumbnail for m: placeholder cells
// when the cover is in the terminal, blanks otherwise. Always thumbCols
// cells wide.
func (t *thumbs) Cells(key string, r int) string {
	blank := strings.Repeat(" ", thumbCols)
	if !t.enabled {
		return blank
	}
	t.mu.Lock()
	id, ok := t.ids[key]
	ok = ok && t.ready[id]
	t.mu.Unlock()
	if !ok {
		return blank
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
	for c := 0; c < thumbCols; c++ {
		b.WriteRune(kitty.Placeholder)
		b.WriteString(row)
		b.WriteRune(kitty.Diacritics[c])
	}
	b.WriteString("\x1b[39m")
	return b.String()
}

// Clear frees every thumbnail image in the terminal.
func (t *thumbs) Clear() {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	var buf []byte
	for id := range t.ready {
		buf = append(buf, fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)...)
	}
	t.ids = map[string]int{}
	t.ready = map[int]bool{}
	t.mu.Unlock()
	if len(buf) > 0 {
		_, _ = t.out.Write(buf)
	}
}
