package generic

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/hongnoul/mangap/constant"
	"github.com/hongnoul/mangap/network"
	"github.com/hongnoul/mangap/source"
)

// searchTTL is how long an in-memory search result is reused. The TUI keeps
// its own stale-while-revalidate cache on top, this only dedupes bursts.
const searchTTL = 5 * time.Minute

type searchEntry struct {
	mangas []*source.Manga
	at     time.Time
}

// Search for mangas by given title
func (s *Scraper) Search(query string) ([]*source.Manga, error) {
	return s.SearchContext(context.Background(), query)
}

// SearchContext searches with one plain HTTP request that ctx can cancel.
// It skips colly on purpose: colly's Wait() blocks on every request in
// flight on the collector (so abandoned live searches delayed the current
// one), its RandomDelay added up to Delay per query, and its CacheDir kept
// search pages forever.
//
// When the configuration has a quick-search endpoint (a lighter page with
// the same ranking) it is tried first, falling back to the full page.
func (s *Scraper) SearchContext(ctx context.Context, query string) ([]*source.Manga, error) {
	key := strings.TrimSpace(strings.ToLower(query))
	s.mu.Lock()
	if e, ok := s.searches[key]; ok && time.Since(e.at) < searchTTL {
		s.mu.Unlock()
		return e.mangas, nil
	}
	s.mu.Unlock()

	var (
		mangas []*source.Manga
		err    error
	)
	if s.config.QuickSearchURL != nil && s.config.QuickMangaExtractor != nil {
		mangas, err = s.searchPage(ctx, s.config.QuickSearchURL(query), s.config.QuickMangaExtractor)
	}
	if s.config.QuickSearchURL == nil || s.config.QuickMangaExtractor == nil || (err != nil && ctx.Err() == nil) {
		mangas, err = s.searchPage(ctx, s.config.GenerateSearchURL(query), s.config.MangaExtractor)
	}
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.searches[key] = searchEntry{mangas: mangas, at: time.Now()}
	s.mu.Unlock()
	return mangas, nil
}

func (s *Scraper) searchPage(ctx context.Context, address string, ex *Extractor) ([]*source.Manga, error) {
	doc, err := s.fetchDoc(ctx, address, "https://google.com")
	if err != nil {
		return nil, err
	}
	base := doc.Url
	sel := doc.Find(ex.Selector)
	mangas := make([]*source.Manga, 0, sel.Length())
	seen := map[string]bool{}
	sel.Each(func(i int, selection *goquery.Selection) {
		link := ex.URL(selection)
		if link == "" {
			return
		}
		u := link
		if base != nil {
			if ref, err := base.Parse(link); err == nil {
				u = ref.String()
			}
		}
		if seen[u] {
			return
		}
		seen[u] = true
		manga := &source.Manga{
			Name:     ex.Name(selection),
			URL:      u,
			Index:    uint16(len(mangas)),
			Chapters: make([]*source.Chapter, 0),
			ID:       filepath.Base(u),
			Source:   s,
		}
		if ex.Cover != nil {
			manga.Metadata.Cover.ExtraLarge = ex.Cover(selection)
		}
		if ex.AltNames != nil {
			manga.Metadata.Synonyms = ex.AltNames(selection)
		}
		mangas = append(mangas, manga)
	})
	return mangas, nil
}

// fetchDoc GETs address over the shared HTTP/2 client and parses it.
func (s *Scraper) fetchDoc(ctx context.Context, address, referer string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", constant.UserAgent)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "en-US")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := network.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", address, resp.Status)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	doc.Url = resp.Request.URL
	return doc, nil
}

// Warm opens a connection to the source (DNS, TCP, TLS, HTTP/2) so the
// first search does not pay for it. Repeated calls keep it from idling out.
func (s *Scraper) Warm(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.config.BaseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", constant.UserAgent)
	resp, err := network.Client.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
