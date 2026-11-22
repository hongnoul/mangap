package history

import (
	"fmt"
	"time"

	"github.com/hongnoul/mangap/source"
)

type SavedChapter struct {
	SourceID           string `json:"source_id"`
	MangaName          string `json:"manga_name"`
	MangaURL           string `json:"manga_url"`
	MangaChaptersTotal int    `json:"manga_chapters_total"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	ID                 string `json:"id"`
	Index              int    `json:"index"`
	MangaID            string `json:"manga_id"`

	// Page is the 0-based page the reader stopped at.
	Page int `json:"page,omitempty"`
	// Pages is the chapter's page count when it was saved (0 = unknown).
	Pages int `json:"pages,omitempty"`
	// Finished is set when the reader stopped on the last sheet.
	Finished  bool      `json:"finished,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func encodeKey(manga, sourceID string) string {
	return fmt.Sprintf("%s (%s)", manga, sourceID)
}

func (c *SavedChapter) encode() string {
	return encodeKey(c.MangaName, c.SourceID)
}

func (c *SavedChapter) String() string {
	return fmt.Sprintf("%s : %d / %d", c.MangaName, c.Index, c.MangaChaptersTotal)
}

// Matches reports whether chapter is the bookmarked one: by URL, then ID,
// then name (some sources rotate URLs).
func (c *SavedChapter) Matches(chapter *source.Chapter) bool {
	if c == nil || chapter == nil {
		return false
	}
	switch {
	case c.URL != "" && chapter.URL == c.URL:
		return true
	case c.ID != "" && chapter.ID == c.ID:
		return true
	default:
		return c.Name != "" && chapter.Name == c.Name
	}
}

// ResumePage is the page to open chapter at: the bookmark if it matches
// and is unfinished, else 0.
func (c *SavedChapter) ResumePage(chapter *source.Chapter) int {
	if !c.Matches(chapter) || c.Finished {
		return 0
	}
	return c.Page
}

func newSavedChapter(chapter *source.Chapter) *SavedChapter {
	return &SavedChapter{
		SourceID:           chapter.Manga.Source.ID(),
		MangaName:          chapter.Manga.Name,
		MangaURL:           chapter.Manga.URL,
		Name:               chapter.Name,
		URL:                chapter.URL,
		ID:                 chapter.ID,
		MangaID:            chapter.Manga.ID,
		MangaChaptersTotal: len(chapter.Manga.Chapters),
		Index:              int(chapter.Index),
	}
}
