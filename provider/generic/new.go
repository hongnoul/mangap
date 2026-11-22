package generic

import (
	"github.com/hongnoul/mangap/source"
)

// New generates a new scraper with given configuration
func New(conf *Configuration) source.Source {
	return &Scraper{
		searches: make(map[string]searchEntry),
		chapters: make(map[string][]*source.Chapter),
		pages:    make(map[string][]*source.Page),
		inflight: make(map[string]*chapterCall),
		config:   conf,
	}
}
