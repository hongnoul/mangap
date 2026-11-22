package mangadex

import (
	"github.com/hongnoul/mangap/filesystem"
	"github.com/hongnoul/mangap/where"
	"github.com/metafates/gache"
	"github.com/samber/mo"
	"path/filepath"
	"sync"
	"time"
)

// cacher is a disk-backed map. gache hands out its internal map, so the
// mutex guards reads against the in-place writes of concurrent searches.
type cacher[T any] struct {
	mu       sync.Mutex
	internal *gache.Cache[map[string]T]
}

func newCacher[T any](name string) *cacher[T] {
	return &cacher[T]{
		internal: gache.New[map[string]T](
			&gache.Options{
				Path:       filepath.Join(where.Cache(), name+".json"),
				Lifetime:   time.Hour * 24,
				FileSystem: &filesystem.GacheFs{},
			},
		),
	}
}

func (c *cacher[T]) Get(key string) mo.Option[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, expired, err := c.internal.Get()
	if err != nil || expired || cached == nil {
		return mo.None[T]()
	}

	if value, ok := cached[key]; ok {
		return mo.Some[T](value)
	}

	return mo.None[T]()
}

func (c *cacher[T]) Set(key string, value T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, expired, err := c.internal.Get()
	if err != nil {
		return err
	}

	if expired || cached == nil {
		cached = map[string]T{}
	}

	cached[key] = value
	return c.internal.Set(cached)
}
