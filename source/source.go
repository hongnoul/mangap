package source

// Source is the interface that all sources must implement.
type Source interface {
	Name() string
	Search(query string) ([]*Manga, error)
	ChaptersOf(manga *Manga) ([]*Chapter, error)
	PagesOf(chapter *Chapter) ([]*Page, error)
	ID() string
}

// LocalSource is a source backed by files already on disk (the downloads
// archive). Its chapters are opened in place instead of fetched.
type LocalSource interface {
	Source
	// PathOf is the chapter's file or directory on disk.
	PathOf(chapter *Chapter) string
}

// IsLocal reports whether chapter comes from a LocalSource.
func IsLocal(chapter *Chapter) bool {
	if chapter == nil || chapter.Manga == nil {
		return false
	}
	_, ok := chapter.Manga.Source.(LocalSource)
	return ok
}
