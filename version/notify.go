package version

import (
	"fmt"
	"github.com/hongnoul/mangaz/color"
	"github.com/hongnoul/mangaz/constant"
	"github.com/hongnoul/mangaz/icon"
	"github.com/hongnoul/mangaz/key"
	"github.com/hongnoul/mangaz/style"
	"github.com/hongnoul/mangaz/util"
	"github.com/spf13/viper"
)

func Notify() {
	if !viper.GetBool(key.CliVersionCheck) {
		return
	}

	erase := util.PrintErasable(fmt.Sprintf("%s Checking if new version is available...", icon.Get(icon.Progress)))
	version, err := Latest()
	erase()
	if err == nil {
		comp, err := Compare(version, constant.Version)
		if err == nil && comp <= 0 {
			return
		}
	}

	fmt.Printf(`
%s New version is available %s %s
%s

`,
		style.Fg(color.Green)("▇▇▇"),
		style.Bold(version),
		style.Faint(fmt.Sprintf("(You're on %s)", constant.Version)),
		style.Faint("https://github.com/hongnoul/mangaz/releases/tag/v"+version),
	)

}
