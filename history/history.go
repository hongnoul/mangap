package history

import (
	"sync"
	"time"

	"github.com/hongnoul/mangap/filesystem"
	"github.com/hongnoul/mangap/integration"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/where"
	"github.com/metafates/gache"
	"github.com/spf13/viper"
)

var cacher = gache.New[map[string]*SavedChapter](
	&gache.Options{
		Path:       where.History(),
		FileSystem: &filesystem.GacheFs{},
	},
)

// mu guards the read-modify-write of the history file.
var mu sync.Mutex

// Get returns all chapters from the history file
func Get() (chapters map[string]*SavedChapter, err error) {
	cached, expired, err := cacher.Get()

	if err != nil {
		return nil, err
	}

	if expired || cached == nil {
		return make(map[string]*SavedChapter), nil
	}

	return cached, nil
}

func markAnilist(chapter *source.Chapter) {
	if !viper.GetBool(key.AnilistEnable) {
		return
	}
	go func() {
		log.Info("Saving chapter to anilist")
		if err := integration.Anilist.MarkRead(chapter); err != nil {
			log.Warn("Saving chapter to anilist failed: " + err.Error())
		}
	}()
}

func put(c *SavedChapter) error {
	mu.Lock()
	defer mu.Unlock()

	saved, err := Get()
	if err != nil {
		return err
	}
	c.UpdatedAt = time.Now()
	saved[c.encode()] = c
	return cacher.Set(saved)
}

// Save saves the chapter to the history file (page 0) and marks it read
// on anilist. Used for downloads.
func Save(chapter *source.Chapter) error {
	markAnilist(chapter)
	return put(newSavedChapter(chapter))
}

// Opened records that reading started in chapter. It keeps the saved page
// when the chapter is the one already bookmarked.
func Opened(chapter *source.Chapter) error {
	c := newSavedChapter(chapter)
	if prev, ok := Lookup(chapter.Manga); ok && prev.Matches(chapter) {
		c.Page, c.Pages, c.Finished = prev.Page, prev.Pages, prev.Finished
	}
	return put(c)
}

// SaveProgress bookmarks the exact page (0-based) the reader stopped at.
// A finished chapter is marked read on anilist.
func SaveProgress(chapter *source.Chapter, page, pages int, finished bool) error {
	if finished {
		markAnilist(chapter)
	}
	c := newSavedChapter(chapter)
	c.Page, c.Pages, c.Finished = page, pages, finished
	return put(c)
}

// Lookup returns the bookmark for manga, if any.
func Lookup(manga *source.Manga) (*SavedChapter, bool) {
	if manga == nil || manga.Source == nil {
		return nil, false
	}
	saved, err := Get()
	if err != nil {
		return nil, false
	}
	c, ok := saved[encodeKey(manga.Name, manga.Source.ID())]
	return c, ok
}

// Remove removes the chapter from the history file
func Remove(chapter *SavedChapter) error {
	mu.Lock()
	defer mu.Unlock()

	saved, err := Get()
	if err != nil {
		return err
	}

	delete(saved, chapter.encode())

	return cacher.Set(saved)
}
