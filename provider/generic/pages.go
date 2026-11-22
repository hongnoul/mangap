package generic

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/hongnoul/mangap/source"
)

// PagesOf given source.Chapter
func (s *Scraper) PagesOf(chapter *source.Chapter) ([]*source.Page, error) {
	s.mu.Lock()
	if pages, ok := s.pages[chapter.URL]; ok {
		s.mu.Unlock()
		return pages, nil
	}
	s.mu.Unlock()

	doc, err := s.fetchDoc(context.Background(), chapter.URL, chapter.URL)
	if err != nil {
		return nil, err
	}
	ex := s.config.PageExtractor
	sel := doc.Find(ex.Selector)
	pages := make([]*source.Page, sel.Length())
	sel.Each(func(i int, selection *goquery.Selection) {
		link := ex.URL(selection)
		ext := filepath.Ext(link)
		// remove some query params from the extension
		ext = strings.Split(ext, "?")[0]
		pages[i] = &source.Page{
			URL:       link,
			Index:     uint16(i),
			Chapter:   chapter,
			Extension: ext,
		}
	})
	chapter.Pages = pages

	s.mu.Lock()
	s.pages[chapter.URL] = pages
	s.mu.Unlock()
	return pages, nil
}
