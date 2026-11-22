package downloader

import (
	"testing"

	"github.com/hongnoul/mangaz/source"
)

func names(cs []*source.Chapter) (out []string) {
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return
}

func TestReadingOrderNewestFirstSource(t *testing.T) {
	// Mangapill-style: listed newest first, Index follows list position.
	in := []*source.Chapter{
		{Name: "Chapter 3", Index: 0}, {Name: "Chapter 2", Index: 1}, {Name: "Chapter 1", Index: 2},
	}
	got := names(readingOrder(in))
	if got[0] != "Chapter 1" || got[2] != "Chapter 3" {
		t.Fatalf("order %v", got)
	}
}

func TestNeighborsNextIsNewer(t *testing.T) {
	m := &source.Manga{}
	for i, n := range []string{"Chapter 3", "Chapter 2.5", "Chapter 2", "Chapter 1"} {
		m.Chapters = append(m.Chapters, &source.Chapter{Name: n, Index: uint16(i), Manga: m})
	}
	nb := NewNeighbors(m.Chapters[2]) // Chapter 2
	if next, _ := nb.Title(1); next != "Chapter 2.5" {
		t.Fatalf("next = %q", next)
	}
	if prev, _ := nb.Title(-1); prev != "Chapter 1" {
		t.Fatalf("prev = %q", prev)
	}
}
