package downloader

import (
	"regexp"
	"sort"
	"strconv"

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
	list := readingOrder(chapter.Manga.Chapters)
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

var chapterNum = regexp.MustCompile(`(?i)(?:ch(?:apter)?\.?\s*)?(\d+(?:\.\d+)?)`)

// number pulls the chapter number out of a name like "Chapter 10.5".
func number(name string) (float64, bool) {
	m := chapterNum.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}
	f, err := strconv.ParseFloat(m[1], 64)
	return f, err == nil
}

// readingOrder returns chapters oldest first. Sources disagree: some list
// newest first, and Index is not always meaningful. Sort by Index, then
// trust the chapter numbers in the names when they say it runs backwards.
func readingOrder(in []*source.Chapter) []*source.Chapter {
	list := make([]*source.Chapter, 0, len(in))
	for _, c := range in {
		if c != nil {
			list = append(list, c)
		}
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].Index < list[b].Index })
	up, down := 0, 0
	prev, havePrev := 0.0, false
	for _, c := range list {
		n, ok := number(c.Name)
		if !ok {
			continue
		}
		if havePrev {
			if n > prev {
				up++
			} else if n < prev {
				down++
			}
		}
		prev, havePrev = n, true
	}
	if down > up {
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}
	return list
}
