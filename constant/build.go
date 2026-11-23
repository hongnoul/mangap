package constant

import (
	"regexp"
	"runtime/debug"
	"strings"
)

var pseudoVersion = regexp.MustCompile(`\d{14}-[0-9a-f]{12}`)

var (
	// Version is set at release time with
	// -X github.com/hongnoul/mangap/constant.Version=<semver>.
	// Builds via `go install ...@vX.Y.Z` fall back to the module version.
	Version  = "0.1.0"
	BuiltAt  = "Unknown"
	BuiltBy  = "Unknown"
	Revision = "Unknown"

	// AnilistClientID is the public AniList API client bundled with mangap.
	// Its redirect URL is http://localhost:47563/callback. Override at build
	// time with -X github.com/hongnoul/mangap/constant.AnilistClientID=<id>.
	AnilistClientID = "57796"
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	// Only real tags (go install ...@v1.2.3). Local builds get pseudo
	// versions like v0.0.0-20261123001342-cff9dc6499c7+dirty.
	if v := info.Main.Version; Version == "0.1.0" && v != "" && v != "(devel)" && !pseudoVersion.MatchString(v) && !strings.Contains(v, "+") {
		Version = strings.TrimPrefix(v, "v")
	}
	if Revision == "Unknown" {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				Revision = s.Value[:7]
			}
		}
	}
}
