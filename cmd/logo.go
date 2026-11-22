package cmd

import (
	"strings"

	"github.com/hongnoul/mangaz/constant"
	"github.com/hongnoul/mangaz/style"
)

// coloredLogo renders the ascii logo with the shared gradient.
func coloredLogo() string {
	return style.Gradient(strings.Split(constant.AsciiArtLogo, "\n"))
}
