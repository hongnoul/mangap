package cmd

import (
	"fmt"
	"strings"

	"github.com/hongnoul/mangaz/constant"
	"github.com/muesli/termenv"
)

// logoGradient are the color stops the logo fades through, left to right.
var logoGradient = [][3]float64{
	{0xff, 0x5f, 0x87}, // pink
	{0xaf, 0x5f, 0xff}, // purple
	{0x5f, 0xd7, 0xff}, // cyan
}

// coloredLogo renders the ascii logo with a horizontal gradient.
// It uses termenv directly (not lipgloss, which is forced to monochrome)
// and falls back to plain text when the terminal has no color support
// or NO_COLOR is set.
func coloredLogo() string {
	profile := termenv.EnvColorProfile()
	if profile == termenv.Ascii {
		return constant.AsciiArtLogo
	}

	lines := strings.Split(constant.AsciiArtLogo, "\n")
	width := 0
	for _, line := range lines {
		if n := len([]rune(line)); n > width {
			width = n
		}
	}

	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		for col, r := range []rune(line) {
			if r == ' ' {
				b.WriteRune(r)
				continue
			}
			b.WriteString(termenv.String(string(r)).
				Foreground(profile.Color(gradientAt(col, width))).
				Bold().
				String())
		}
	}
	return b.String()
}

// gradientAt returns the hex color at column col of width.
func gradientAt(col, width int) string {
	t := 0.0
	if width > 1 {
		t = float64(col) / float64(width-1)
	}
	segments := len(logoGradient) - 1
	pos := t * float64(segments)
	idx := int(pos)
	if idx > segments-1 {
		idx = segments - 1
	}
	frac := pos - float64(idx)
	from, to := logoGradient[idx], logoGradient[idx+1]

	var c [3]int
	for k := range c {
		c[k] = int(from[k] + (to[k]-from[k])*frac + 0.5)
	}
	return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
}
