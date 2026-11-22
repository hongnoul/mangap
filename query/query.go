package query

import (
	"github.com/hongnoul/mangap/filesystem"
	"github.com/hongnoul/mangap/where"
	"github.com/metafates/gache"
)

type queryRecord struct {
	Rank  int    `json:"rank"`
	Query string `json:"query"`
}

var cacher = gache.New[map[string]*queryRecord](
	&gache.Options{
		Path:       where.Queries(),
		FileSystem: &filesystem.GacheFs{},
	},
)
