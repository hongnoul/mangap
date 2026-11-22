package query

import (
	"sync"

	"github.com/hongnoul/mangap/filesystem"
	"github.com/hongnoul/mangap/where"
	"github.com/metafates/gache"
)

type queryRecord struct {
	Rank  int    `json:"rank"`
	Query string `json:"query"`
}

// mu guards cacher and suggestionCache: the TUI calls Remember from
// goroutines (two at once when a live result is opened), and gache's
// lazy load plus the shared map crash with concurrent map writes.
var mu sync.Mutex

var cacher = gache.New[map[string]*queryRecord](
	&gache.Options{
		Path:       where.Queries(),
		FileSystem: &filesystem.GacheFs{},
	},
)
