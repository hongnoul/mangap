package where

import (
	"github.com/hongnoul/mangap/constant"
	"github.com/hongnoul/mangap/filesystem"
	"github.com/hongnoul/mangap/key"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

const EnvConfigPath = "MANGAP_CONFIG_PATH"

// mkdir creates a directory and all parent directories if they don't exist
// will return the path of the directory
func mkdir(path string) string {
	lo.Must0(filesystem.Api().MkdirAll(path, os.ModePerm))
	return path
}

// Config path
// Will create the directory if it doesn't exist
func Config() string {
	var path string

	if customDir, present := os.LookupEnv(EnvConfigPath); present {
		path = customDir
	} else {
		base := lo.Must(os.UserConfigDir())
		migrateLegacyConfig(base)
		path = filepath.Join(base, constant.App)
	}

	return mkdir(path)
}

// Sources path
// Will create the directory if it doesn't exist
func Sources() string {
	return mkdir(filepath.Join(Config(), "sources"))
}

func AnilistBinds() string {
	return filepath.Join(Config(), "anilist.json")
}

// Logs path
// Will create the directory if it doesn't exist
func Logs() string {
	return mkdir(filepath.Join(Config(), "logs"))
}

// Queries path
// Will create the directory if it doesn't exist
func Queries() string {
	return filepath.Join(Cache(), "queries.json")
}

// History path to the file
// Will create the directory if it doesn't exist
func History() string {
	return filepath.Join(Config(), "history.json")
}

// Downloads path
// Will create the directory if it doesn't exist
func Downloads() string {
	path, err := filepath.Abs(viper.GetString(key.DownloaderPath))

	if err != nil {
		path, err = os.Getwd()
		if err != nil {
			path = "."
		}
	}

	return mkdir(path)
}

// Cache path
// Will create the directory if it doesn't exist
func Cache() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = filepath.Join(".", "cache")
	}

	cacheDir = filepath.Join(cacheDir, constant.App)
	return mkdir(cacheDir)
}

// Temp path
// Will create the directory if it doesn't exist
func Temp() string {
	tempDir := filepath.Join(os.TempDir(), constant.App)
	return mkdir(tempDir)
}
