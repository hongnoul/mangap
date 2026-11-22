package provider

import (
	"errors"
	"testing"
	"time"

	"github.com/hongnoul/mangaz/source"
)

type fake struct {
	name  string
	n     int
	err   error
	delay time.Duration
}

func (f *fake) Name() string { return f.name }
func (f *fake) ID() string   { return f.name }
func (f *fake) Search(string) ([]*source.Manga, error) {
	time.Sleep(f.delay)
	ms := make([]*source.Manga, f.n)
	for i := range ms {
		ms[i] = &source.Manga{Name: f.name}
	}
	return ms, f.err
}
func (f *fake) ChaptersOf(*source.Manga) ([]*source.Chapter, error) { return nil, nil }
func (f *fake) PagesOf(*source.Chapter) ([]*source.Page, error)     { return nil, nil }

func TestSearchFallback(t *testing.T) {
	SearchTimeout = 50 * time.Millisecond
	cases := []struct {
		name string
		srcs []source.Source
		want string
		err  bool
	}{
		{"first wins", []source.Source{&fake{name: "a", n: 2}, &fake{name: "b", n: 5}}, "a", false},
		{"empty falls back", []source.Source{&fake{name: "a"}, &fake{name: "b", n: 1}}, "b", false},
		{"error falls back", []source.Source{&fake{name: "a", err: errors.New("x")}, &fake{name: "b", n: 1}}, "b", false},
		{"timeout falls back", []source.Source{&fake{name: "a", n: 1, delay: time.Second}, &fake{name: "b", n: 1}}, "b", false},
		{"all empty", []source.Source{&fake{name: "a"}, &fake{name: "b"}}, "", false},
		{"all fail", []source.Source{&fake{name: "a", err: errors.New("x")}}, "", true},
	}
	for _, c := range cases {
		ms, used, err := SearchFallback(c.srcs, "q")
		if (err != nil) != c.err {
			t.Fatalf("%s: err = %v", c.name, err)
		}
		got := ""
		if used != nil {
			got = used.Name()
			if len(ms) == 0 {
				t.Fatalf("%s: used set but no results", c.name)
			}
		}
		if got != c.want {
			t.Fatalf("%s: used %q, want %q", c.name, got, c.want)
		}
	}
}
