package source

import (
	"context"
	"sync"
)

// ContextSearcher is a source whose search can be cancelled. Implementing
// it also promises the source is safe to call from several goroutines at
// once (search and chapter lookups may overlap), which is what lets the
// TUI fire overlapping live searches and prefetch chapters.
type ContextSearcher interface {
	SearchContext(ctx context.Context, query string) ([]*Manga, error)
}

// Warmer opens (or keeps open) the network connection a source searches
// over, so the first real query skips DNS, TCP and TLS setup.
type Warmer interface {
	Warm(ctx context.Context) error
}

// legacyLocks serializes plain Search calls per source. Sources that do not
// implement ContextSearcher (Lua scripts) keep interpreter state that is not
// safe for concurrent use.
var legacyLocks sync.Map // source ID -> *sync.Mutex

// SearchContext searches s, honouring ctx. Sources without native
// cancellation run under a per-source lock in a goroutine. A cancelled call
// returns at once while the underlying search finishes in the background.
func SearchContext(ctx context.Context, s Source, query string) ([]*Manga, error) {
	if cs, ok := s.(ContextSearcher); ok {
		return cs.SearchContext(ctx, query)
	}
	type result struct {
		mangas []*Manga
		err    error
	}
	done := make(chan result, 1)
	go func() {
		mu, _ := legacyLocks.LoadOrStore(s.ID(), &sync.Mutex{})
		mu.(*sync.Mutex).Lock()
		defer mu.(*sync.Mutex).Unlock()
		if ctx.Err() != nil {
			done <- result{err: ctx.Err()}
			return
		}
		m, err := s.Search(query)
		done <- result{m, err}
	}()
	select {
	case r := <-done:
		return r.mangas, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ConcurrencySafe reports whether s may be called from several goroutines.
func ConcurrencySafe(s Source) bool {
	_, ok := s.(ContextSearcher)
	return ok
}
