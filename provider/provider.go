package provider

import (
	"github.com/hongnoul/mangap/filesystem"
	"github.com/hongnoul/mangap/provider/custom"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/util"
	"github.com/hongnoul/mangap/where"
	"github.com/samber/lo"
	"os"
	"path/filepath"
)

type Provider struct {
	ID   string
	Name string
	// PingURL is a cheap endpoint the TUI hits to show whether the source
	// is reachable. Custom Lua sources use their `-- @url` header (or the
	// first URL in the script). Empty means unknown.
	PingURL      string
	UsesHeadless bool
	IsCustom     bool
	CreateSource func() (source.Source, error)
}

func (p Provider) String() string {
	return p.Name
}

func Builtins() []*Provider {
	return builtinProviders
}

func Customs() []*Provider {
	files, err := filesystem.Api().ReadDir(where.Sources())

	if err != nil {
		return make([]*Provider, 0)
	}

	paths := lo.FilterMap(files, func(f os.FileInfo, _ int) (string, bool) {
		if filepath.Ext(f.Name()) == CustomProviderExtension {
			return filepath.Join(where.Sources(), f.Name()), true
		}
		return "", false
	})
	providers := make([]*Provider, len(paths))

	for i, path := range paths {
		// Check if source contains line `require("headless")`
		// if so, set UsesHeadless to true.
		// This approach is not ideal, but it's the only way to do it without
		// actually loading the source.
		usesHeadless, _ := filesystem.Api().FileContainsAnyBytes(path, [][]byte{
			[]byte("require(\"headless\")"),
			[]byte("require('headless')"),
			[]byte("require(headless)"),
			[]byte("require'headless'"),
		})

		name := util.FileStem(path)
		path := path
		providers[i] = &Provider{
			ID:           custom.IDfromName(name),
			UsesHeadless: usesHeadless,
			PingURL:      customPingURL(path),
			IsCustom:     true,
			Name:         name,
			CreateSource: func() (source.Source, error) {
				return custom.LoadSource(path, true)
			},
		}
	}

	return providers
}

func Get(name string) (*Provider, bool) {
	for _, provider := range Builtins() {
		if provider.Name == name {
			return provider, true
		}
	}

	for _, provider := range Customs() {
		if provider.Name == name {
			return provider, true
		}
	}

	return nil, false
}

// GetByID finds a builtin or custom provider by its ID.
func GetByID(id string) (*Provider, bool) {
	for _, p := range Builtins() {
		if p.ID == id {
			return p, true
		}
	}
	for _, p := range Customs() {
		if p.ID == id {
			return p, true
		}
	}
	return nil, false
}
