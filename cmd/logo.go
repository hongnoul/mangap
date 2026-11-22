package cmd

import (
	"strings"

	"github.com/hongnoul/mangap/constant"
	"github.com/hongnoul/mangap/style"
)

// coloredLogo renders the ascii logo with the shared gradient.
func coloredLogo() string {
	return style.Gradient(strings.Split(constant.AsciiArtLogo, "\n"))
}
