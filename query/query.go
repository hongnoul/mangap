package query

import (
	"github.com/hongnoul/mangaz/filesystem"
	"github.com/hongnoul/mangaz/where"
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
