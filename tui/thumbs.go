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
	ids   map[string]int  // cover url -> image id
	ready map[int]bool    // transmitted
	busy  map[string]bool // fetch in flight
	next  int
	sem   chan struct{}
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
	if !t.enabled {
		return
	}
	for _, m := range mangas {
		u := coverURL(m)
		if u == "" {
			continue
		}
		t.mu.Lock()
		_, have := t.ids[u]
		if have || t.busy[u] {
			t.mu.Unlock()
			continue
		}
		t.busy[u] = true
		t.mu.Unlock()
		go t.fetch(u, m.URL)
	}
}

func (t *thumbs) fetch(u, referer string) {
	t.sem <- struct{}{}
	defer func() { <-t.sem }()
	defer func() {
		t.mu.Lock()
		delete(t.busy, u)
		t.mu.Unlock()
	}()

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", constant.UserAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := network.Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
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
	t.ids[u] = id
	t.ready[id] = true
	t.mu.Unlock()
	if t.send != nil {
		t.send(thumbReadyMsg{})
	}
}

// Cells returns row r (0-based) of the thumbnail for m: placeholder cells
// when the cover is in the terminal, blanks otherwise. Always thumbCols
// cells wide.
func (t *thumbs) Cells(m *source.Manga, r int) string {
	blank := strings.Repeat(" ", thumbCols)
	if !t.enabled {
		return blank
	}
	t.mu.Lock()
	id, ok := t.ids[coverURL(m)]
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
