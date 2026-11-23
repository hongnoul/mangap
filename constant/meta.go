package constant

const (
	// App is the application name: binary, config dir, cache dir, env prefix.
	App = "mangap"
	// LegacyApp is the name mangap was forked from.
	LegacyApp = "mangal"
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36"
)

// LegacyApps are earlier names whose config dir is migrated once on first
// run (see where.Config), newest first: mangaz was the working name of
// mangap, mangal is the project it was forked from.
var LegacyApps = []string{"mangaz", LegacyApp}
