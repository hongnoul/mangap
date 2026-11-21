package provider

import (
	"github.com/hongnoul/mangaz/provider/generic"
	"github.com/hongnoul/mangaz/provider/mangadex"
	"github.com/hongnoul/mangaz/provider/manganato"
	"github.com/hongnoul/mangaz/provider/manganelo"
	"github.com/hongnoul/mangaz/provider/mangapill"
	"github.com/hongnoul/mangaz/source"
)

const CustomProviderExtension = ".lua"

var builtinProviders = []*Provider{
	{
		ID:   mangadex.ID,
		Name: mangadex.Name,
		CreateSource: func() (source.Source, error) {
			return mangadex.New(), nil
		},
	},
}

func init() {
	for _, conf := range []*generic.Configuration{
		manganelo.Config,
		manganato.Config,
		mangapill.Config,
	} {
		conf := conf
		builtinProviders = append(builtinProviders, &Provider{
			ID:   conf.ID(),
			Name: conf.Name,
			CreateSource: func() (source.Source, error) {
				return generic.New(conf), nil
			},
		})
	}
}
