package downloader

import (
	"github.com/hongnoul/mangaz/reader"
	"github.com/hongnoul/mangaz/reader/pages"
	"github.com/hongnoul/mangaz/source"
)

// Neighbors lets the reader cross into the chapters around the one it
// opened, in the manga's chapter order (providers list chapters in
// reading order).
type Neighbors struct {
	list []*source.Chapter
	at   int
	// Current is the chapter shown, updated as the reader crosses.
	Current *source.Chapter
}

var _ reader.Chapters = (*Neighbors)(nil)

// NewNeighbors locates chapter in its manga's chapter list. It returns
// nil when the chapter has no siblings to cross into.
func NewNeighbors(chapter *source.Chapter) *Neighbors {
	if chapter == nil || chapter.Manga == nil {
		return nil
	}
	list := chapter.Manga.Chapters
	at := -1
	for i, c := range list {
		if c == chapter {
			at = i
			break
		}
	}
	if at < 0 {
		for i, c := range list {
			if c != nil && c.URL != "" && c.URL == chapter.URL {
				at = i
				break
			}
		}
	}
	if at < 0 || len(list) < 2 {
		return nil
	}
	return &Neighbors{list: list, at: at, Current: chapter}
}

func (n *Neighbors) get(pos int) *source.Chapter {
	i := n.at + pos
	if i < 0 || i >= len(n.list) {
		return nil
	}
	return n.list[i]
}

// Title implements reader.Chapters.
func (n *Neighbors) Title(pos int) (string, bool) {
	c := n.get(pos)
	if c == nil {
		return "", false
	}
	return c.Name, true
}

// Load implements reader.Chapters.
func (n *Neighbors) Load(pos int) (pages.Source, error) {
	return Prepare(n.get(pos), func(string) {})
}

// Entered implements reader.Chapters.
func (n *Neighbors) Entered(pos int) {
	if c := n.get(pos); c != nil {
		n.Current = c
		SaveReadHistory(c)
	}
}

// ReaderOptionsFor is ReaderOptions with chapter crossing around chapter.
func ReaderOptionsFor(chapter *source.Chapter) (reader.Options, *Neighbors) {
	opt := ReaderOptions()
	nb := NewNeighbors(chapter)
	if nb != nil {
		opt.Chapters = nb
	}
	return opt, nb
}
