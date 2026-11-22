package version

import (
	"fmt"
	"github.com/hongnoul/mangap/color"
	"github.com/hongnoul/mangap/constant"
	"github.com/hongnoul/mangap/icon"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/style"
	"github.com/hongnoul/mangap/util"
	"github.com/spf13/viper"
)

func Notify() {
	if !viper.GetBool(key.CliVersionCheck) {
		return
	}

	erase := util.PrintErasable(fmt.Sprintf("%s Checking if new version is available...", icon.Get(icon.Progress)))
	version, err := Latest()
	erase()
	if err != nil {
		return
	}

	comp, err := Compare(version, constant.Version)
	if err != nil || comp <= 0 {
		return
	}

	fmt.Printf(`
%s New version is available %s %s
%s

`,
		style.Fg(color.Green)("▇▇▇"),
		style.Bold(version),
		style.Faint(fmt.Sprintf("(You're on %s)", constant.Version)),
		style.Faint("https://github.com/hongnoul/mangap/releases/tag/v"+version),
	)

}
