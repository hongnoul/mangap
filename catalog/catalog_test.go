package catalog

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hongnoul/mangap/source"
)

type fakeSrc struct{ id string }

func (f fakeSrc) Name() string                                        { return f.id }
func (f fakeSrc) ID() string                                          { return f.id }
func (f fakeSrc) Search(string) ([]*source.Manga, error)              { return nil, nil }
func (f fakeSrc) ChaptersOf(*source.Manga) ([]*source.Chapter, error) { return nil, nil }
func (f fakeSrc) PagesOf(*source.Chapter) ([]*source.Page, error)     { return nil, nil }

func manga(name, url string, alt ...string) *source.Manga {
	m := &source.Manga{Name: name, URL: url}
	m.Metadata.Synonyms = alt
	return m
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Kaguya-sama: Love Is War": "kaguya sama love is war",
		"  ONE   piece!! ":         "one piece",
		"":                         "",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSearchRanksAndMatchesAlt(t *testing.T) {
	c := Open(filepath.Join(t.TempDir(), "c.json"))
	c.Add("mp", "kaguya", []*source.Manga{
		manga("Kaguya-hime", "/3"),
		manga("Kaguya-sama - Love Is War", "/1", "Kaguya-sama wa Kokurasetai"),
		manga("Kaguya-sama wo Kataritai", "/2"),
	})
	c.Add("other", "kaguya", []*source.Manga{manga("Kaguya Other Source", "/9")})

	got := c.Search("mp", "kaguy", 10)
	if len(got) != 3 {
		t.Fatalf("got %d results, want 3", len(got))
	}
	for _, e := range got {
		if e.SourceID != "mp" {
			t.Fatalf("leaked entry from %s", e.SourceID)
		}
	}
	if r := c.Search("mp", "kokurasetai", 10); len(r) != 1 || r[0].URL != "/1" {
		t.Fatalf("alt title match failed: %+v", r)
	}
	if r := c.Search("mp", "love war", 10); len(r) != 1 || r[0].URL != "/1" {
		t.Fatalf("multi word match failed: %+v", r)
	}

	// Opening one ranks it first.
	c.Opened(manga("Kaguya-sama wo Kataritai", "/2"))
	m := manga("Kaguya-sama wo Kataritai", "/2")
	m.Source = fakeSrc{"mp"}
	c.Opened(m)
	if r := c.Search("mp", "kaguya", 10); r[0].URL != "/2" {
		t.Fatalf("opened entry not ranked first: %s", r[0].URL)
	}
}

func TestCachedAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	c := Open(path)
	c.Add("mp", "Berserk", []*source.Manga{manga("Berserk", "/1"), manga("Berserk of Gluttony", "/2")})
	entries, fresh, ok := c.Cached("mp", "  berserk ")
	if !ok || !fresh || len(entries) != 2 || entries[0].URL != "/1" {
		t.Fatalf("Cached = %v %v %+v", ok, fresh, entries)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	d := Open(path)
	entries, _, ok = d.Cached("mp", "berserk")
	if !ok || len(entries) != 2 {
		t.Fatalf("not persisted: %v %+v", ok, entries)
	}
	if r := d.Search("mp", "glut", 5); len(r) != 1 {
		t.Fatalf("index not rebuilt on load: %+v", r)
	}
	if m := entries[1].Manga(fakeSrc{"mp"}); m.Name != "Berserk of Gluttony" || m.Source == nil {
		t.Fatalf("Manga() = %+v", m)
	}
}

func TestMatches(t *testing.T) {
	if !Matches("kaguy", "Kaguya-sama", nil) || Matches("naruto", "Kaguya-sama", nil) {
		t.Fatal("name matching")
	}
	if !Matches("kokura", "Kaguya-sama", []string{"Kaguya-sama wa Kokurasetai"}) {
		t.Fatal("alt matching")
	}
}

func BenchmarkSearch20k(b *testing.B) {
	c := Open("")
	ms := make([]*source.Manga, 0, 20000)
	for i := 0; i < 20000; i++ {
		ms = append(ms, manga("Some Manga Title Number "+time.Duration(i).String(), "/"+time.Duration(i).String(), "Alt Name "+time.Duration(i).String()))
	}
	c.Add("mp", "", ms)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Search("mp", "title number 1", 30)
	}
}
