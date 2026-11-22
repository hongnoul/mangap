package generic

import (
	"context"
	"path/filepath"

	"github.com/PuerkitoBio/goquery"
	"github.com/hongnoul/mangap/source"
)

// ChaptersOf given source.Manga
func (s *Scraper) ChaptersOf(manga *source.Manga) ([]*source.Chapter, error) {
	return s.ChaptersOfContext(context.Background(), manga)
}

// ChaptersOfContext fetches the chapter list once per manga URL. Concurrent
// callers for the same manga share one request.
func (s *Scraper) ChaptersOfContext(ctx context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	s.mu.Lock()
	if chapters, ok := s.chapters[manga.URL]; ok {
		s.mu.Unlock()
		return s.adopt(manga, chapters), nil
	}
	if c, ok := s.inflight[manga.URL]; ok {
		s.mu.Unlock()
		select {
		case <-c.done:
			if c.err != nil {
				return nil, c.err
			}
			return s.adopt(manga, c.chapters), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	c := &chapterCall{done: make(chan struct{})}
	s.inflight[manga.URL] = c
	s.mu.Unlock()

	// The fetch itself is not tied to ctx: another caller may be waiting
	// on it, and a finished prefetch is exactly what makes enter instant.
	c.chapters, c.err = s.fetchChapters(manga)

	s.mu.Lock()
	delete(s.inflight, manga.URL)
	if c.err == nil {
		s.chapters[manga.URL] = c.chapters
	}
	s.mu.Unlock()
	close(c.done)

	if c.err != nil {
		return nil, c.err
	}
	return s.adopt(manga, c.chapters), nil
}

// adopt points the cached chapters at manga (a fresh *Manga for the same
// URL may come from a newer search) and records them on it.
func (s *Scraper) adopt(manga *source.Manga, chapters []*source.Chapter) []*source.Chapter {
	if len(chapters) > 0 && chapters[0].Manga != manga {
		s.mu.Lock()
		for _, c := range chapters {
			c.Manga = manga
		}
		s.mu.Unlock()
	}
	manga.Chapters = chapters
	return chapters
}

func (s *Scraper) fetchChapters(manga *source.Manga) ([]*source.Chapter, error) {
	doc, err := s.fetchDoc(context.Background(), manga.URL, manga.URL)
	if err != nil {
		return nil, err
	}
	ex := s.config.ChapterExtractor
	sel := doc.Find(ex.Selector)
	chapters := make([]*source.Chapter, sel.Length())
	sel.Each(func(i int, selection *goquery.Selection) {
		link := ex.URL(selection)
		u := link
		if ref, err := doc.Url.Parse(link); err == nil {
			u = ref.String()
		}
		chapters[i] = &source.Chapter{
			Name:   ex.Name(selection),
			URL:    u,
			Index:  uint16(i + 1),
			Pages:  make([]*source.Page, 0),
			ID:     filepath.Base(u),
			Manga:  manga,
			Volume: ex.Volume(selection),
		}
	})

	if s.config.ReverseChapters {
		n := len(chapters)
		reversed := make([]*source.Chapter, n)
		for i, chapter := range chapters {
			reversed[n-i-1] = chapter
			chapter.Index = uint16(n - i)
		}
		chapters = reversed
	}
	return chapters, nil
}
