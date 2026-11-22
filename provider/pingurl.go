package provider

import (
	"net/url"
	"regexp"

	"github.com/hongnoul/mangap/filesystem"
)

var (
	luaURLTag = regexp.MustCompile(`(?m)^\s*--+\s*@url\s+(\S+)`)
	luaAnyURL = regexp.MustCompile(`https?://[A-Za-z0-9.\-]+(:\d+)?`)
)

// customPingURL finds a URL to probe for a custom Lua source: the
// `-- @url` header if present, else the first http(s) host in the script.
func customPingURL(path string) string {
	data, err := filesystem.Api().ReadFile(path)
	if err != nil {
		return ""
	}
	if m := luaURLTag.FindSubmatch(data); m != nil {
		if u, err := url.Parse(string(m[1])); err == nil && u.Host != "" {
			return u.String()
		}
	}
	if m := luaAnyURL.Find(data); m != nil {
		return string(m)
	}
	return ""
}
