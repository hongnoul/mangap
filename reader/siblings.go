package reader

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hongnoul/mangap/reader/pages"
)

// Siblings crosses between chapters stored side by side on disk: the
// folders and .cbz/.zip files next to the opened one, in natural order
// (ch1, ch2, ..., ch10).
type Siblings struct {
	paths []string
	at    int
}

var _ Chapters = (*Siblings)(nil)

// NewSiblings lists path's siblings. It returns nil when there are none.
func NewSiblings(path string) *Siblings {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	dir := filepath.Dir(abs)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var paths []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if e.IsDir() || ext == ".cbz" || ext == ".zip" {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	sort.Slice(paths, func(a, b int) bool {
		return pages.NaturalLess(filepath.Base(paths[a]), filepath.Base(paths[b]))
	})
	for i, p := range paths {
		if p == abs {
			if len(paths) < 2 {
				return nil
			}
			return &Siblings{paths: paths, at: i}
		}
	}
	return nil
}

func (s *Siblings) get(pos int) (string, bool) {
	i := s.at + pos
	if i < 0 || i >= len(s.paths) {
		return "", false
	}
	return s.paths[i], true
}

// Title implements Chapters.
func (s *Siblings) Title(pos int) (string, bool) {
	p, ok := s.get(pos)
	if !ok {
		return "", false
	}
	return strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)), true
}

// Number implements Chapters.
func (s *Siblings) Number(pos int) (int, int, bool) {
	if _, ok := s.get(pos); !ok {
		return 0, 0, false
	}
	return s.at + pos + 1, len(s.paths), true
}

// Load implements Chapters.
func (s *Siblings) Load(pos int) (pages.Source, error) {
	p, ok := s.get(pos)
	if !ok {
		return nil, os.ErrNotExist
	}
	return pages.Open(p)
}

// Entered implements Chapters.
func (s *Siblings) Entered(int) {}
