package constant

const (
	// App is the application name: binary, config dir, cache dir, env prefix.
	App = "mangaz"
	// LegacyApp is the name mangaz was forked from. Its config dir is
	// migrated once on first run (see where.Config).
	LegacyApp = "mangal"
	Version   = "4.0.6"
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36"
)
