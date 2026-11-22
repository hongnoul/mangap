package reader

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hongnoul/mangap/reader/pages"
	"github.com/hongnoul/mangap/reader/viewer"
)

type fakeChapters struct {
	mu      sync.Mutex
	min     int
	max     int
	pages   int
	delay   time.Duration
	fail    map[int]bool
	loads   []int
	entered []int
}

func (f *fakeChapters) Title(pos int) (string, bool) {
	if pos < f.min || pos > f.max {
		return "", false
	}
	return "Ch " + string(rune('A'+pos+5)), true
}

func (f *fakeChapters) Number(pos int) (int, int, bool) {
	if pos < f.min || pos > f.max {
		return 0, 0, false
	}
	return pos - f.min + 1, f.max - f.min + 1, true
}

func (f *fakeChapters) Load(pos int) (pages.Source, error) {
	time.Sleep(f.delay)
	f.mu.Lock()
	f.loads = append(f.loads, pos)
	fail := f.fail[pos]
	f.mu.Unlock()
	if fail {
		return nil, errors.New("boom")
	}
	src := memSource{}
	for i := 0; i < f.pages; i++ {
		src = append(src, page(uint8(100+pos*20+i*5)))
	}
	return src, nil
}

func (f *fakeChapters) Entered(pos int) {
	f.mu.Lock()
	f.entered = append(f.entered, pos)
	f.mu.Unlock()
}

func TestCrossIntoNextChapterAndBack(t *testing.T) {
	fc := &fakeChapters{min: -1, max: 1, pages: 4}
	h := start(t, 3, Options{Chapters: fc})
	h.waitFrames(1)
	h.press("jj") // page 2, last of chapter 0
	h.waitFrames(2)
	time.Sleep(100 * time.Millisecond) // next chapter preloads meanwhile
	h.press("j")                       // cross
	h.waitFor(func(s string) bool { return strings.Contains(s, "Chapter 3/3  Ch G") })
	h.press("j")
	time.Sleep(100 * time.Millisecond)
	h.quit()
	if h.res.Chapter != 1 || h.res.Page != 1 {
		t.Fatalf("stopped at chapter %d page %d, want 1/1", h.res.Chapter, h.res.Page)
	}
	if len(fc.entered) != 1 || fc.entered[0] != 1 {
		t.Fatalf("entered %v want [1]", fc.entered)
	}
}

func TestCrossBackLandsOnLastPage(t *testing.T) {
	fc := &fakeChapters{min: -1, max: 0, pages: 5}
	h := start(t, 3, Options{Chapters: fc})
	h.waitFrames(1)
	h.press("k") // on page 0: previous chapter, loads on demand
	h.waitFor(func(s string) bool { return strings.Contains(s, "Ch E") && strings.Count(s, "\x1b[?2026l") >= 2 })
	time.Sleep(100 * time.Millisecond)
	h.quit()
	if h.res.Chapter != -1 || h.res.Page != 4 {
		t.Fatalf("stopped at chapter %d page %d, want -1/4", h.res.Chapter, h.res.Page)
	}
}

func TestCrossPendingWhileLoadingThenCancel(t *testing.T) {
	fc := &fakeChapters{min: 0, max: 1, pages: 4, delay: 300 * time.Millisecond}
	h := start(t, 2, Options{Chapters: fc})
	h.waitFrames(1)
	h.press("j")
	h.waitFrames(2)
	h.press("j") // edge: loading toast
	h.waitFor(func(s string) bool { return strings.Contains(s, "Loading Chapter 2/2  Ch G") })
	h.press("k") // change of mind: stays in chapter 0
	time.Sleep(500 * time.Millisecond)
	h.quit()
	if h.res.Chapter != 0 || h.res.Page != 0 {
		t.Fatalf("stopped at chapter %d page %d, want 0/0", h.res.Chapter, h.res.Page)
	}
}

func TestLastChapterToastAndFailure(t *testing.T) {
	fc := &fakeChapters{min: -1, max: 0, pages: 2, fail: map[int]bool{-1: true}}
	h := start(t, 1, Options{Chapters: fc})
	h.waitFrames(1)
	h.press("j")
	h.waitFor(func(s string) bool { return strings.Contains(s, "Last chapter") })
	h.press("k")
	h.waitFor(func(s string) bool { return strings.Contains(s, "Could not load") })
	h.quit()
	if h.res.Chapter != 0 {
		t.Fatalf("chapter %d want 0", h.res.Chapter)
	}
}

func TestStateEdges(t *testing.T) {
	s := viewer.NewState(6)
	if !s.AtEnd(-1) || s.AtEnd(1) || s.SheetsLeft(1) != 5 {
		t.Fatal("start edges wrong")
	}
	s.ToggleSpread() // [0] [1 2] [3 4] [5]
	s.GoTo(3)
	if s.SheetsLeft(1) != 1 || s.SheetsLeft(-1) != 2 {
		t.Fatalf("spread sheets left %d/%d", s.SheetsLeft(1), s.SheetsLeft(-1))
	}
}

func TestCtrlJKJumpsChaptersFromAnyPage(t *testing.T) {
	fc := &fakeChapters{min: -1, max: 1, pages: 4}
	h := start(t, 5, Options{Chapters: fc})
	h.waitFrames(1)
	h.press("j\n") // page 1, then ctrl-j: next chapter at its first page
	h.waitFor(func(s string) bool { return strings.Contains(s, "Chapter 3/3") })
	h.press("j")
	time.Sleep(100 * time.Millisecond)
	h.press("\x0b") // ctrl-k: back to chapter 0, first page
	h.waitFor(func(s string) bool { return strings.Count(s, "Chapter 2/3") >= 1 })
	time.Sleep(100 * time.Millisecond)
	h.quit()
	if h.res.Chapter != 0 || h.res.Page != 0 {
		t.Fatalf("stopped at chapter %d page %d, want 0/0", h.res.Chapter, h.res.Page)
	}
}
