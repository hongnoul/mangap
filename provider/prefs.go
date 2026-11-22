package provider

import (
	"strings"

	"github.com/hongnoul/mangap/key"
	"github.com/spf13/viper"
)

// Ordered returns the enabled providers: sources.order with
// sources.disabled removed. Unknown names are skipped. Builtins missing
// from sources.order are appended, so a new builtin is never silently lost.
func Ordered() []*Provider {
	disabled := map[string]bool{}
	for _, n := range viper.GetStringSlice(key.SourcesDisabled) {
		disabled[strings.ToLower(n)] = true
	}

	var (
		out  []*Provider
		seen = map[string]bool{}
	)
	add := func(p *Provider) {
		k := strings.ToLower(p.Name)
		if seen[k] || disabled[k] {
			return
		}
		seen[k] = true
		out = append(out, p)
	}

	for _, n := range viper.GetStringSlice(key.SourcesOrder) {
		if p, ok := Get(n); ok {
			add(p)
		}
	}
	for _, p := range Builtins() {
		add(p)
	}
	// Custom Lua sources show up unless explicitly disabled.
	for _, p := range Customs() {
		add(p)
	}
	return out
}

// SaveMangadexLanguage persists the preferred MangaDex language.
func SaveMangadexLanguage(code string) error {
	viper.Set(key.MangadexLanguage, code)
	err := viper.WriteConfig()
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return viper.SafeWriteConfig()
	}
	return err
}
