package provider

import (
	"github.com/hongnoul/mangaz/provider/generic"
	"github.com/hongnoul/mangaz/provider/mangadex"
	"github.com/hongnoul/mangaz/provider/mangapill"
	"github.com/hongnoul/mangaz/source"
)

const CustomProviderExtension = ".lua"

// builtinProviders in default priority order: Mangapill had full chapter
// lists for 20/20 test titles and answered fastest; MangaDex is the
// fallback (official API, other languages, but licensed series are often
// missing chapters).
var builtinProviders = []*Provider{
	{
		ID:   mangapill.Config.ID(),
		Name: mangapill.Config.Name,
		CreateSource: func() (source.Source, error) {
			return generic.New(mangapill.Config), nil
		},
	},
	{
		ID:   mangadex.ID,
		Name: mangadex.Name,
		CreateSource: func() (source.Source, error) {
			return mangadex.New(), nil
		},
	},
}
