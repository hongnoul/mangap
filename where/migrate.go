package where

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hongnoul/mangap/constant"
	"github.com/hongnoul/mangap/filesystem"
	"github.com/spf13/afero"
)

var migrateOnce sync.Once

// migrateLegacyConfig copies the config dir of an earlier name (mangaz, then
// mangal) into the mangap config dir, once, so sources, history, and
// anilist bindings carry over. It only runs when the mangap dir does not
// exist yet and a legacy dir does: an existing mangap dir is never
// touched. The legacy config file (mangal.toml, mangaz.toml) is renamed to
// mangap.toml so viper picks it up. Failures are swallowed: a half-copied
// dir is removed and mangap starts fresh, which is the behavior without
// migration.
func migrateLegacyConfig(base string) {
	migrateOnce.Do(func() {
		to := filepath.Join(base, constant.App)
		for _, legacy := range constant.LegacyApps {
			if isDir, _ := afero.IsDir(filesystem.Api(), filepath.Join(base, legacy)); isDir {
				_ = migrateDir(filepath.Join(base, legacy), to)
				return
			}
		}
	})
}

func migrateDir(from, to string) error {
	fs := filesystem.Api()

	if exists, _ := afero.Exists(fs, to); exists {
		return nil
	}

	if isDir, _ := afero.IsDir(fs, from); !isDir {
		return nil
	}

	err := afero.Walk(fs, from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}

		target := filepath.Join(to, renameLegacyFile(rel))

		if info.IsDir() {
			return fs.MkdirAll(target, os.ModePerm)
		}

		return copyFile(fs, path, target, info.Mode())
	})

	if err != nil {
		_ = fs.RemoveAll(to)
	}

	return err
}

// renameLegacyFile maps the legacy top-level config file name to the new
// one. Everything else keeps its relative path.
func renameLegacyFile(rel string) string {
	if filepath.Dir(rel) != "." {
		return rel
	}
	for _, name := range constant.LegacyApps {
		if legacy := name + "."; strings.HasPrefix(rel, legacy) {
			return constant.App + "." + strings.TrimPrefix(rel, legacy)
		}
	}

	return rel
}

func copyFile(fs afero.Fs, from, to string, mode os.FileMode) error {
	src, err := fs.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := fs.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err = io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}

	return dst.Close()
}
