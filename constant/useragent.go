package constant

import (
	"net/url"
	"strings"
)

// HonestUserAgent identifies mangap plainly. MangaDex wants API clients to
// do so, and its CDN answers spoofed browser agents with a 302 to a bot
// challenge page instead of the image.
func HonestUserAgent() string { return App + "/" + Version }

// UserAgentFor picks the agent for a request to rawURL: honest for
// MangaDex hosts, the browser agent for scraped sites that expect one.
func UserAgentFor(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		host := strings.ToLower(u.Hostname())
		if host == "mangadex.org" || strings.HasSuffix(host, ".mangadex.org") ||
			strings.HasSuffix(host, ".mangadex.network") {
			return HonestUserAgent()
		}
	}
	return UserAgent
}
