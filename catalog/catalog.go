// Package catalog is a local index of every manga the user has seen in
// search results, plus a stale-while-revalidate cache of whole queries.
// Both live in memory and persist to disk, so typing gets matches in
// microseconds before (and while) the network answers.
package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/where"
)

const (
	fileName = "catalog.json"
	// maxEntries / maxQueries cap the file. Oldest (by last seen) go first.
	maxEntries = 20000
	maxQueries = 2000
	// FreshFor is how long a cached query counts as fresh. Older entries are
	// still shown instantly but trigger a refresh.
	FreshFor = 30 * time.Minute
)

// Entry is one manga as last returned by a source.
type Entry struct {
	SourceID string   `json:"s"`
	Name     string   `json:"n"`
	URL      string   `json:"u"`
	ID       string   `json:"i,omitempty"`
	Cover    string   `json:"c,omitempty"`
	Alt      []string `json:"a,omitempty"`
	// Hits counts how often it was opened, Seen how often returned.
	Hits int   `json:"h,omitempty"`
	Seen int   `json:"z,omitempty"`
	Last int64 `json:"t"`

	norm []string // normalized Name + Alt, built on load/insert
}

// queryEntry is one cached result list, by manga URL.
type queryEntry struct {
	URLs []string `json:"u"`
	At   int64    `json:"t"`
}

type store struct {
	Entries map[string]*Entry     `json:"entries"` // key: source ID + "\x00" + URL
	Queries map[string]queryEntry `json:"queries"` // key: source ID + "\x00" + normalized query
}

// Catalog is safe for concurrent use.
type Catalog struct {
	mu    sync.RWMutex
	data  store
	path  string
	dirty bool
	saveT *time.Timer
}

var (
	defaultOnce sync.Once
	defaultCat  *Catalog
)

// Default is the process-wide catalog under the cache directory, loaded on
// first use.
func Default() *Catalog {
	defaultOnce.Do(func() {
		defaultCat = Open(filepath.Join(where.Cache(), fileName))
	})
	return defaultCat
}

// Open loads the catalog at path (an empty one if it does not exist).
func Open(path string) *Catalog {
	c := &Catalog{path: path, data: store{
		Entries: map[string]*Entry{},
		Queries: map[string]queryEntry{},
	}}
	if raw, err := os.ReadFile(path); err == nil {
		var s store
		if json.Unmarshal(raw, &s) == nil {
			if s.Entries != nil {
				c.data.Entries = s.Entries
			}
			if s.Queries != nil {
				c.data.Queries = s.Queries
			}
		}
	}
	for _, e := range c.data.Entries {
		e.index()
	}
	return c
}

func entryKey(sourceID, url string) string { return sourceID + "\x00" + url }

// Normalize lowercases and drops punctuation so "Kaguya-sama: Love" and
// "kaguya sama love" compare equal.
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// Manga rebuilds a search result for src from the entry.
func (e Entry) Manga(src source.Source) *source.Manga {
	m := &source.Manga{
		Name:     e.Name,
		URL:      e.URL,
		ID:       e.ID,
		Source:   src,
		Chapters: make([]*source.Chapter, 0),
	}
	m.Metadata.Cover.ExtraLarge = e.Cover
	m.Metadata.Synonyms = e.Alt
	return m
}

func (e *Entry) index() {
	// A fresh slice: copies handed out by Search share the old one.
	e.norm = make([]string, 0, 1+len(e.Alt))
	e.norm = append(e.norm, Normalize(e.Name))
	for _, a := range e.Alt {
		if n := Normalize(a); n != "" {
			e.norm = append(e.norm, n)
		}
	}
}

// Add records mangas as returned by a source for query. They become
// searchable at once, and the query's result list is cached for Cached.
func (c *Catalog) Add(sourceID, query string, mangas []*source.Manga) {
	if c == nil || sourceID == "" {
		return
	}
	now := time.Now().Unix()
	urls := make([]string, 0, len(mangas))
	c.mu.Lock()
	for _, m := range mangas {
		if m == nil || m.URL == "" || m.Name == "" {
			continue
		}
		urls = append(urls, m.URL)
		k := entryKey(sourceID, m.URL)
		e := c.data.Entries[k]
		if e == nil {
			e = &Entry{SourceID: sourceID, URL: m.URL}
			c.data.Entries[k] = e
		}
		e.Name, e.ID = m.Name, m.ID
		if cover, err := m.GetCover(); err == nil {
			e.Cover = cover
		}
		if len(m.Metadata.Synonyms) > 0 {
			e.Alt = m.Metadata.Synonyms
		}
		e.Seen++
		e.Last = now
		e.index()
	}
	if q := Normalize(query); q != "" {
		c.data.Queries[entryKey(sourceID, q)] = queryEntry{URLs: urls, At: now}
	}
	c.dirty = true
	c.mu.Unlock()
	c.scheduleSave()
}

// Opened bumps a manga's rank (the user picked it).
func (c *Catalog) Opened(m *source.Manga) {
	if c == nil || m == nil || m.Source == nil {
		return
	}
	c.mu.Lock()
	if e := c.data.Entries[entryKey(m.Source.ID(), m.URL)]; e != nil {
		e.Hits++
		e.Last = time.Now().Unix()
		c.dirty = true
	}
	c.mu.Unlock()
	c.scheduleSave()
}

// Cached returns the last result list for query on sourceID, and whether
// it is still fresh. ok is false when nothing is cached.
func (c *Catalog) Cached(sourceID, query string) (entries []Entry, fresh, ok bool) {
	if c == nil {
		return nil, false, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	q, ok := c.data.Queries[entryKey(sourceID, Normalize(query))]
	if !ok {
		return nil, false, false
	}
	for _, u := range q.URLs {
		if e := c.data.Entries[entryKey(sourceID, u)]; e != nil {
			entries = append(entries, *e)
		}
	}
	return entries, time.Since(time.Unix(q.At, 0)) < FreshFor, true
}

// match is a scored local hit, copied out under the read lock.
type match struct {
	entry Entry
	Score int
}

// Search returns up to limit entries from sourceID whose name or alternative
// titles match query, best first. Matching is word-prefix and substring
// based (typos are left to the network search).
func (c *Catalog) Search(sourceID, query string, limit int) []Entry {
	if c == nil {
		return nil
	}
	q := Normalize(query)
	if q == "" {
		return nil
	}
	words := strings.Fields(q)
	c.mu.RLock()
	var hits []match
	for _, e := range c.data.Entries {
		if e.SourceID != sourceID {
			continue
		}
		best := 0
		for _, n := range e.norm {
			if s := score(n, q, words); s > best {
				best = s
			}
		}
		if best == 0 {
			continue
		}
		best += imin(e.Hits, 20) * 15
		best += imin(e.Seen, 20)
		hits = append(hits, match{*e, best})
	}
	c.mu.RUnlock()
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].entry.Name < hits[j].entry.Name
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Entry, len(hits))
	for i, h := range hits {
		out[i] = h.entry
	}
	return out
}

// score rates how well normalized name n matches query q (words = fields
// of q). 0 means no match.
func score(n, q string, words []string) int {
	switch {
	case n == q:
		return 1000
	case strings.HasPrefix(n, q):
		return 800 - imin(len(n)-len(q), 200)
	}
	if i := strings.Index(n, q); i >= 0 {
		bonus := 0
		if n[i-1] == ' ' {
			bonus = 150 // starts at a word boundary
		}
		return 400 + bonus - imin(i, 100)
	}
	// Every query word prefixes some word of the name, in any order.
	if len(words) > 1 {
		nw := strings.Fields(n)
		for _, w := range words {
			found := false
			for _, x := range nw {
				if strings.HasPrefix(x, w) {
					found = true
					break
				}
			}
			if !found {
				return 0
			}
		}
		return 300
	}
	return 0
}

// Matches reports whether name or any alternative title matches query the
// way Search would. Used to narrow a result list as the query grows.
func Matches(query, name string, alt []string) bool {
	q := Normalize(query)
	if q == "" {
		return true
	}
	words := strings.Fields(q)
	if score(Normalize(name), q, words) > 0 {
		return true
	}
	for _, a := range alt {
		if score(Normalize(a), q, words) > 0 {
			return true
		}
	}
	return false
}

func (c *Catalog) scheduleSave() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == "" || c.saveT != nil {
		return
	}
	c.saveT = time.AfterFunc(2*time.Second, func() { _ = c.Save() })
}

// Save writes the catalog to disk if it changed (atomic rename).
func (c *Catalog) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	c.saveT = nil
	if !c.dirty || c.path == "" {
		c.mu.Unlock()
		return nil
	}
	c.prune()
	raw, err := json.Marshal(c.data)
	c.dirty = false
	c.mu.Unlock()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(c.path), 0o755)
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

// prune drops the least recently seen entries and queries over the caps.
// Caller holds mu.
func (c *Catalog) prune() {
	if over := len(c.data.Entries) - maxEntries; over > 0 {
		keys := make([]string, 0, len(c.data.Entries))
		for k := range c.data.Entries {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return c.data.Entries[keys[i]].Last < c.data.Entries[keys[j]].Last
		})
		for _, k := range keys[:over] {
			delete(c.data.Entries, k)
		}
	}
	if over := len(c.data.Queries) - maxQueries; over > 0 {
		keys := make([]string, 0, len(c.data.Queries))
		for k := range c.data.Queries {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return c.data.Queries[keys[i]].At < c.data.Queries[keys[j]].At
		})
		for _, k := range keys[:over] {
			delete(c.data.Queries, k)
		}
	}
}

func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
