package provider

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/source"
	"github.com/spf13/viper"
)

// SearchTimeout bounds one source's search in a fallback chain, so a dead
// site costs seconds, not the HTTP client's full timeout.
var SearchTimeout = 8 * time.Second

// Ordered returns the providers to use in auto mode: sources.order with
// sources.disabled removed. Unknown names are skipped. Builtins missing
// from sources.order are appended, so a new builtin is never silently lost.
func Ordered() []*Provider {
	disabled := map[string]bool{}
	for _, n := range viper.GetStringSlice(key.SourcesDisabled) {
		disabled[strings.ToLower(n)] = true
	}

	var (
		out  []*Provider
		seen = map[string]bool{}
	)
	add := func(p *Provider) {
		k := strings.ToLower(p.Name)
		if seen[k] || disabled[k] {
			return
		}
		seen[k] = true
		out = append(out, p)
	}

	for _, n := range viper.GetStringSlice(key.SourcesOrder) {
		if p, ok := Get(n); ok {
			add(p)
		}
	}
	for _, p := range Builtins() {
		add(p)
	}
	return out
}

// SearchFallback searches sources in order and returns the first
// non-empty result. A source that errors, times out, or finds nothing
// passes to the next. used is the source whose results were returned.
func SearchFallback(sources []source.Source, query string) (mangas []*source.Manga, used source.Source, err error) {
	var errs []string
	for _, s := range sources {
		ms, err := searchWithTimeout(s, query)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", s.Name(), err))
			continue
		}
		if len(ms) > 0 {
			return ms, s, nil
		}
	}
	if len(errs) == len(sources) && len(errs) > 0 {
		return nil, nil, errors.New("all sources failed: " + strings.Join(errs, "; "))
	}
	return []*source.Manga{}, nil, nil
}

func searchWithTimeout(s source.Source, query string) ([]*source.Manga, error) {
	type res struct {
		ms  []*source.Manga
		err error
	}
	ch := make(chan res, 1)
	go func() {
		ms, err := s.Search(query)
		ch <- res{ms, err}
	}()
	select {
	case r := <-ch:
		return r.ms, r.err
	case <-time.After(SearchTimeout):
		return nil, fmt.Errorf("timed out after %s", SearchTimeout)
	}
}

// IsAuto reports whether search should use the fallback chain.
func IsAuto() bool {
	return !strings.EqualFold(viper.GetString(key.SourcesMode), "manual")
}

// SaveSourcePrefs writes mode, order, and disabled to the config file.
func SaveSourcePrefs(mode string, order, disabled []string) error {
	viper.Set(key.SourcesMode, mode)
	viper.Set(key.SourcesOrder, order)
	viper.Set(key.SourcesDisabled, disabled)
	err := viper.WriteConfig()
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return viper.SafeWriteConfig()
	}
	return err
}

// Auto is a source that searches its sources in priority order and keeps
// the first non-empty result. Mangas it returns still belong to the real
// source that found them, so chapters, pages, and history use that source.
type Auto struct {
	Sources []source.Source

	mu   sync.Mutex
	last source.Source
}

// NewAuto builds the auto source from the configured order.
func NewAuto() (*Auto, error) {
	var srcs []source.Source
	for _, p := range Ordered() {
		s, err := p.CreateSource()
		if err != nil {
			log.Warnf("source %s unavailable: %s", p.Name, err)
			continue
		}
		srcs = append(srcs, s)
	}
	if len(srcs) == 0 {
		return nil, errors.New("no sources enabled (check sources.disabled)")
	}
	return &Auto{Sources: srcs}, nil
}

func (a *Auto) Name() string { return "Auto" }
func (a *Auto) ID() string   { return "auto" }

// Last is the source that answered the most recent search, or nil.
func (a *Auto) Last() source.Source {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last
}

func (a *Auto) Search(query string) ([]*source.Manga, error) {
	ms, used, err := SearchFallback(a.Sources, query)
	a.mu.Lock()
	a.last = used
	a.mu.Unlock()
	return ms, err
}

func (a *Auto) ChaptersOf(m *source.Manga) ([]*source.Chapter, error) {
	return m.Source.ChaptersOf(m)
}

func (a *Auto) PagesOf(c *source.Chapter) ([]*source.Page, error) {
	return c.Source().PagesOf(c)
}
