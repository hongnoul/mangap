// Package archive reads the downloads folder back as a manga source, so
// downloaded chapters can be browsed and read offline.
package archive

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hongnoul/mangap/filesystem"
	"github.com/hongnoul/mangap/reader/pages"
	"github.com/hongnoul/mangap/source"
)

// Entry is one downloaded manga folder.
type Entry struct {
	Name     string
	Dir      string
	Cover    string // cover image path, "" if none was saved
	Chapters []ChapterFile
	ModTime  time.Time // newest chapter
}

// ChapterFile is one downloaded chapter: a pdf/cbz/zip or a folder of images.
type ChapterFile struct {
	Path   string
	Volume string
}

var archiveExt = map[string]bool{".pdf": true, ".cbz": true, ".zip": true}

func hidden(name string) bool { return strings.HasPrefix(name, ".") }

// hasImages reports whether dir directly holds page images.
func hasImages(dir string) bool {
	subs, err := filesystem.Api().ReadDir(dir)
	if err != nil {
		return false
	}
	for _, s := range subs {
		if !s.IsDir() && pages.IsImage(s.Name()) {
			return true
		}
	}
	return false
}

// chaptersIn lists the chapters directly in dir. A subfolder with images
// is a plain-format chapter; one without is read as a volume folder.
func chaptersIn(dir, volume string, depth int, latest *time.Time) []ChapterFile {
	subs, err := filesystem.Api().ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []ChapterFile
	for _, s := range subs {
		name := s.Name()
		if hidden(name) {
			continue
		}
		path := filepath.Join(dir, name)
		switch {
		case s.IsDir() && hasImages(path):
			out = append(out, ChapterFile{Path: path, Volume: volume})
		case s.IsDir() && depth == 0:
			out = append(out, chaptersIn(path, name, 1, latest)...)
			continue
		case !s.IsDir() && archiveExt[strings.ToLower(filepath.Ext(name))]:
			out = append(out, ChapterFile{Path: path, Volume: volume})
		default:
			continue
		}
		if s.ModTime().After(*latest) {
			*latest = s.ModTime()
		}
	}
	return out
}

func findCover(dir string) string {
	subs, err := filesystem.Api().ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, s := range subs {
		n := strings.ToLower(s.Name())
		if !s.IsDir() && strings.HasPrefix(n, "cover.") && pages.IsImage(n) {
			return filepath.Join(dir, s.Name())
		}
	}
	return ""
}

// Scan lists the manga folders under dir that hold at least one chapter,
// most recently downloaded first. Chapters are in natural name order,
// which follows the {padded-index} file names.
func Scan(dir string) []Entry {
	entries, err := filesystem.Api().ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Entry
	for _, e := range entries {
		if !e.IsDir() || hidden(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		latest := e.ModTime()
		chs := chaptersIn(path, "", 0, &latest)
		if len(chs) == 0 {
			continue
		}
		sort.SliceStable(chs, func(a, b int) bool {
			if chs[a].Volume != chs[b].Volume {
				return pages.NaturalLess(chs[a].Volume, chs[b].Volume)
			}
			return pages.NaturalLess(filepath.Base(chs[a].Path), filepath.Base(chs[b].Path))
		})
		out = append(out, Entry{Name: e.Name(), Dir: path, Cover: findCover(path), Chapters: chs, ModTime: latest})
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].ModTime.After(out[b].ModTime) })
	return out
}

// FileURL marks a local path as a thumbnail key.
func FileURL(path string) string { return "file://" + path }

// PathOfURL undoes FileURL, ok false for anything else.
func PathOfURL(u string) (string, bool) {
	if !strings.HasPrefix(u, "file://") {
		return "", false
	}
	return strings.TrimPrefix(u, "file://"), true
}

// indexPrefix is the "[0012] " the default chapter name template adds.
var indexPrefix = regexp.MustCompile(`^\[\d+\]\s*`)

// chapterName turns a chapter file name back into a display name.
func chapterName(path string) string {
	n := filepath.Base(path)
	if ext := filepath.Ext(n); archiveExt[strings.ToLower(ext)] {
		n = strings.TrimSuffix(n, ext)
	}
	if s := indexPrefix.ReplaceAllString(n, ""); s != "" {
		n = s
	}
	return n
}

// Source serves a downloads folder as a source.
type Source struct {
	dir     string
	entries map[*source.Manga]Entry
	paths   map[*source.Chapter]string
}

var _ source.LocalSource = (*Source)(nil)

// New scans dir.
func New(dir string) *Source {
	return &Source{dir: dir, entries: map[*source.Manga]Entry{}, paths: map[*source.Chapter]string{}}
}

func (s *Source) Name() string { return "Downloads" }
func (s *Source) ID() string   { return "downloads" }

// Mangas rescans the folder and returns its manga.
func (s *Source) Mangas() []*source.Manga {
	s.entries = map[*source.Manga]Entry{}
	var out []*source.Manga
	for i, e := range Scan(s.dir) {
		m := &source.Manga{Name: e.Name, URL: FileURL(e.Dir), ID: e.Dir, Index: uint16(i), Source: s}
		cover := e.Cover
		if cover == "" {
			cover = e.Chapters[0].Path // first page of the first chapter
		}
		m.Metadata.Cover.ExtraLarge = FileURL(cover)
		s.entries[m] = e
		out = append(out, m)
	}
	return out
}

// Search filters the downloaded manga by name.
func (s *Source) Search(query string) ([]*source.Manga, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []*source.Manga
	for _, m := range s.Mangas() {
		if q == "" || strings.Contains(strings.ToLower(m.Name), q) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Source) ChaptersOf(m *source.Manga) ([]*source.Chapter, error) {
	e, ok := s.entries[m]
	if !ok {
		latest := time.Time{}
		e = Entry{Dir: m.ID, Chapters: chaptersIn(m.ID, "", 0, &latest)}
	}
	chs := make([]*source.Chapter, 0, len(e.Chapters))
	for i, f := range e.Chapters {
		c := &source.Chapter{
			Name:   chapterName(f.Path),
			URL:    FileURL(f.Path),
			ID:     f.Path,
			Index:  uint16(i + 1),
			Volume: f.Volume,
			Manga:  m,
		}
		s.paths[c] = f.Path
		chs = append(chs, c)
	}
	m.Chapters = chs
	return chs, nil
}

// PagesOf is unused: local chapters are opened in place (see PathOf).
func (s *Source) PagesOf(*source.Chapter) ([]*source.Page, error) {
	return nil, os.ErrInvalid
}

func (s *Source) PathOf(c *source.Chapter) string {
	if p, ok := s.paths[c]; ok {
		return p
	}
	return c.ID
}
