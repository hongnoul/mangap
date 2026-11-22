package constant

var (
	BuiltAt  = "Unknown"
	BuiltBy  = "Unknown"
	Revision = "Unknown"

	// AnilistClientID is the public AniList API client bundled with mangaz.
	// Its redirect URL is http://localhost:47563/callback. Override at build
	// time with -X github.com/hongnoul/mangaz/constant.AnilistClientID=<id>.
	AnilistClientID = "57796"
)
