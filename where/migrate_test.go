package where

import (
	"path/filepath"
	"testing"

	"github.com/hongnoul/mangap/filesystem"
	"github.com/samber/lo"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
)

func TestMigrateDir(t *testing.T) {
	Convey("Given a legacy mangal config dir", t, func() {
		filesystem.SetMemMapFs()
		fs := filesystem.Api()
		base := "/cfg"
		from := filepath.Join(base, "mangal")
		to := filepath.Join(base, "mangap")

		lo.Must0(fs.MkdirAll(filepath.Join(from, "sources"), 0o755))
		lo.Must0(afero.WriteFile(fs, filepath.Join(from, "history.json"), []byte(`{"a":1}`), 0o644))
		lo.Must0(afero.WriteFile(fs, filepath.Join(from, "anilist.json"), []byte(`{}`), 0o644))
		lo.Must0(afero.WriteFile(fs, filepath.Join(from, "mangal.toml"), []byte(`x = 1`), 0o644))
		lo.Must0(afero.WriteFile(fs, filepath.Join(from, "sources", "s.lua"), []byte(`-- lua`), 0o644))

		Convey("When the mangap dir does not exist", func() {
			So(migrateDir(from, to), ShouldBeNil)

			Convey("Then files are copied and the config file is renamed", func() {
				So(string(lo.Must(afero.ReadFile(fs, filepath.Join(to, "history.json")))), ShouldEqual, `{"a":1}`)
				So(lo.Must(afero.Exists(fs, filepath.Join(to, "anilist.json"))), ShouldBeTrue)
				So(lo.Must(afero.Exists(fs, filepath.Join(to, "sources", "s.lua"))), ShouldBeTrue)
				So(string(lo.Must(afero.ReadFile(fs, filepath.Join(to, "mangap.toml")))), ShouldEqual, `x = 1`)
				So(lo.Must(afero.Exists(fs, filepath.Join(to, "mangal.toml"))), ShouldBeFalse)
			})

			Convey("And the legacy dir is left untouched", func() {
				So(lo.Must(afero.Exists(fs, filepath.Join(from, "history.json"))), ShouldBeTrue)
			})
		})

		Convey("When the mangap dir already exists", func() {
			lo.Must0(fs.MkdirAll(to, 0o755))
			lo.Must0(afero.WriteFile(fs, filepath.Join(to, "history.json"), []byte(`{"new":1}`), 0o644))
			So(migrateDir(from, to), ShouldBeNil)

			Convey("Then nothing is overwritten", func() {
				So(string(lo.Must(afero.ReadFile(fs, filepath.Join(to, "history.json")))), ShouldEqual, `{"new":1}`)
				So(lo.Must(afero.Exists(fs, filepath.Join(to, "anilist.json"))), ShouldBeFalse)
			})
		})
	})

	Convey("Given no legacy dir", t, func() {
		filesystem.SetMemMapFs()
		So(migrateDir("/none/mangal", "/none/mangap"), ShouldBeNil)
		So(lo.Must(afero.Exists(filesystem.Api(), "/none/mangap")), ShouldBeFalse)
	})
}

func TestRenameLegacyFile(t *testing.T) {
	Convey("Legacy config file names map to mangap", t, func() {
		So(renameLegacyFile("mangal.toml"), ShouldEqual, "mangap.toml")
		So(renameLegacyFile("mangal.yaml"), ShouldEqual, "mangap.yaml")
		So(renameLegacyFile("mangaz.toml"), ShouldEqual, "mangap.toml")
		So(renameLegacyFile("history.json"), ShouldEqual, "history.json")
		So(renameLegacyFile(filepath.Join("sources", "mangal.lua")), ShouldEqual, filepath.Join("sources", "mangal.lua"))
	})
}
