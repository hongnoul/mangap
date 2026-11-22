package generic

import (
	"sync"

	"github.com/hongnoul/mangap/source"
)

// Scraper is a generic scraper downloads html pages and parses them.
// It is safe for concurrent use: live search, chapter prefetch and the
// downloader may all call it at once.
type Scraper struct {
	mu       sync.Mutex
	searches map[string]searchEntry
	chapters map[string][]*source.Chapter
	pages    map[string][]*source.Page
	// inflight dedupes concurrent ChaptersOf calls for the same manga
	// (a hover prefetch and the user pressing enter).
	inflight map[string]*chapterCall

	config *Configuration
}

type chapterCall struct {
	done     chan struct{}
	chapters []*source.Chapter
	err      error
}

// Name of the scraper
func (s *Scraper) Name() string {
	return s.config.Name
}

// ID of the scraper
func (s *Scraper) ID() string {
	return s.config.ID()
}
