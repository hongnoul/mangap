package tui

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hongnoul/mangap/catalog"
	"github.com/hongnoul/mangap/source"
)

// slowSource answers substring searches over a fixed title list after a
// network-like delay, honouring cancellation.
type slowSource struct {
	delay    time.Duration
	titles   []string
	calls    atomic.Int32
	canceled atomic.Int32
	mu       sync.Mutex
	queries  []string
}

func (s *slowSource) Name() string { return "slow" }
func (s *slowSource) ID() string   { return "slow" }
func (s *slowSource) Search(q string) ([]*source.Manga, error) {
	return s.SearchContext(context.Background(), q)
}
func (s *slowSource) ChaptersOf(*source.Manga) ([]*source.Chapter, error) { return nil, nil }
func (s *slowSource) PagesOf(*source.Chapter) ([]*source.Page, error)     { return nil, nil }

func (s *slowSource) SearchContext(ctx context.Context, q string) ([]*source.Manga, error) {
	s.calls.Add(1)
	s.mu.Lock()
	s.queries = append(s.queries, q)
	s.mu.Unlock()
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		s.canceled.Add(1)
		return nil, ctx.Err()
	}
	var out []*source.Manga
	for _, t := range s.titles {
		if strings.Contains(strings.ToLower(t), strings.ToLower(strings.TrimSpace(q))) {
			out = append(out, &source.Manga{Name: t, URL: "/" + catalog.Normalize(t), Source: s})
		}
	}
	return out, nil
}

var kaguyaTitles = []string{
	"Kaguya-sama - Love Is War",
	"Kaguya-sama wo Kataritai",
	"Kaguya-sama wa Kokurasetai: Doujin-ban",
	"Kaguya-hime",
	"Kagurabachi",
	"Naruto",
}

// harness drives the bubble like bubbletea does: Update, then run the
// returned commands on goroutines and feed their messages back.
type harness struct {
	t    *testing.T
	b    *statefulBubble
	msgs chan tea.Msg
}

func newHarness(t *testing.T, src source.Source) *harness {
	t.Helper()
	cat := catalog.Open(filepath.Join(t.TempDir(), "catalog.json"))
	old := liveCatalog
	liveCatalog = func() *catalog.Catalog { return cat }
	t.Cleanup(func() { liveCatalog = old })

	b := newBubble()
	b.selectedSources = []source.Source{src}
	b.setState(searchState)
	b.inputC.Focus()
	return &harness{t: t, b: b, msgs: make(chan tea.Msg, 256)}
}

func (h *harness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		msg := cmd()
		switch m := msg.(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range m {
				h.run(c)
			}
		default:
			h.msgs <- m
		}
	}()
}

func (h *harness) send(msg tea.Msg) {
	_, cmd := h.b.Update(msg)
	h.run(cmd)
}

func (h *harness) typeText(s string, gap time.Duration) {
	for _, r := range s {
		h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		h.pump(gap)
	}
}

// pump processes messages for d.
func (h *harness) pump(d time.Duration) {
	deadline := time.After(d)
	for {
		select {
		case m := <-h.msgs:
			h.send(m)
		case <-deadline:
			return
		}
	}
}

func (h *harness) names() []string {
	var out []string
	for _, it := range h.b.mangasC.Items() {
		out = append(out, it.(*listItem).internal.(*source.Manga).Name)
	}
	return out
}

func TestLiveFirstKeyFiresImmediately(t *testing.T) {
	src := &slowSource{delay: 100 * time.Millisecond, titles: kaguyaTitles}
	h := newHarness(t, src)
	h.typeText("k", 0)
	h.pump(20 * time.Millisecond)
	if src.calls.Load() != 1 {
		t.Fatalf("first key should fire at once, got %d calls", src.calls.Load())
	}
	h.pump(300 * time.Millisecond)
	if len(h.names()) != 5 {
		t.Fatalf("results for k: %v", h.names())
	}
}

func TestLiveNarrowsInstantlyWhileTyping(t *testing.T) {
	src := &slowSource{delay: 120 * time.Millisecond, titles: kaguyaTitles}
	h := newHarness(t, src)
	h.typeText("ka", 0)
	h.pump(400 * time.Millisecond)
	if n := len(h.names()); n != 5 {
		t.Fatalf("ka: %v", h.names())
	}
	// The very next frame after typing narrows locally: no network wait.
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("guy")})
	got := h.names()
	if len(got) != 4 {
		t.Fatalf("instant narrow for kaguy: %v", got)
	}
	for _, n := range got {
		if !strings.Contains(strings.ToLower(n), "kaguy") {
			t.Fatalf("narrowed list kept %q", n)
		}
	}
	if !h.b.livePending() {
		t.Fatal("network answer should be pending")
	}
	h.pump(400 * time.Millisecond)
	if h.b.livePending() {
		t.Fatal("still pending after the answer")
	}
}

func TestLiveCatalogAnswersRepeatQueryOffline(t *testing.T) {
	src := &slowSource{delay: 50 * time.Millisecond, titles: kaguyaTitles}
	h := newHarness(t, src)
	h.typeText("kaguya", 0)
	h.pump(400 * time.Millisecond)

	// Clear, then type a different prefix of a known title: the catalog
	// matches before any network call can answer.
	h.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.pump(10 * time.Millisecond)
	before := src.calls.Load()
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("love is")})
	got := h.names()
	if len(got) != 1 || got[0] != "Kaguya-sama - Love Is War" {
		t.Fatalf("catalog match: %v", got)
	}
	_ = before

	// Exact repeat of a cached query: served from cache, no new request.
	h.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.pump(10 * time.Millisecond)
	calls := src.calls.Load()
	h.b.inputC.SetValue("kaguya")
	h.run(h.b.scheduleLive())
	if len(h.names()) != 4 {
		t.Fatalf("cached kaguya: %v", h.names())
	}
	h.pump(300 * time.Millisecond)
	if src.calls.Load() != calls {
		t.Fatalf("fresh cached query hit the network (%d -> %d)", calls, src.calls.Load())
	}
}

func TestLiveCancelsDivergedAndDropsStale(t *testing.T) {
	src := &slowSource{delay: 200 * time.Millisecond, titles: kaguyaTitles}
	h := newHarness(t, src)
	h.typeText("kag", 0)
	h.pump(30 * time.Millisecond)
	// Diverge: backspace everything and type something unrelated.
	h.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.typeText("nar", 0)
	h.pump(600 * time.Millisecond)
	if src.canceled.Load() == 0 {
		t.Fatal("diverged request was not cancelled")
	}
	got := h.names()
	if len(got) != 1 || got[0] != "Naruto" {
		t.Fatalf("stale results leaked: %v", got)
	}
}

func TestLiveMergeKeepsRowsAndPickedSelection(t *testing.T) {
	src := &slowSource{delay: 40 * time.Millisecond, titles: kaguyaTitles}
	h := newHarness(t, src)
	h.typeText("kaguya", 0)
	h.pump(300 * time.Millisecond)
	items := h.b.mangasC.Items()
	if len(items) != 4 {
		t.Fatalf("kaguya: %v", h.names())
	}
	// Walk to the third result, then refine: it stays highlighted and
	// keeps its row object (and so its cover).
	h.send(tea.KeyMsg{Type: tea.KeyTab})
	h.send(tea.KeyMsg{Type: tea.KeyTab})
	sel := h.b.mangasC.SelectedItem()
	h.typeText("-sama", 0)
	h.pump(300 * time.Millisecond)
	if h.b.mangasC.SelectedItem() != sel {
		t.Fatalf("selection moved to %v", h.b.mangasC.SelectedItem())
	}
}
