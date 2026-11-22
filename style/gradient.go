package style

import (
	"fmt"
	"strings"
	"sync"

	"github.com/muesli/termenv"
)

// GradientStops are the default color stops (pink -> purple -> cyan).
var GradientStops = [][3]float64{
	{0xff, 0x5f, 0x87},
	{0xaf, 0x5f, 0xff},
	{0x5f, 0xd7, 0xff},
}

var (
	profileOnce sync.Once
	profile     termenv.Profile
)

// colorProfile is the real terminal profile. lipgloss is forced to
// monochrome app-wide, so colored art goes through termenv directly.
func colorProfile() termenv.Profile {
	profileOnce.Do(func() { profile = termenv.EnvColorProfile() })
	return profile
}

// Gradient colors each visible rune of lines along a diagonal gradient
// (top-left to bottom-right). Spaces and blank braille cells stay
// uncolored. Without color support it returns the plain text.
func Gradient(lines []string) string {
	p := colorProfile()
	if p == termenv.Ascii {
		return strings.Join(lines, "\n")
	}

	width := 0
	for _, l := range lines {
		if n := len([]rune(l)); n > width {
			width = n
		}
	}
	height := len(lines)
	// Cells are about twice as tall as wide, so weight rows by 2.
	span := float64(width-1) + 2*float64(height-1)

	var b strings.Builder
	for y, l := range lines {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x, r := range []rune(l) {
			if r == ' ' || r == '\u2800' {
				b.WriteRune(r)
				continue
			}
			t := 0.0
			if span > 0 {
				t = (float64(x) + 2*float64(y)) / span
			}
			b.WriteString(termenv.String(string(r)).
				Foreground(p.Color(gradientAt(t))).
				String())
		}
	}
	return b.String()
}

// gradientAt returns the hex color at t in [0, 1] along GradientStops.
func gradientAt(t float64) string {
	segments := len(GradientStops) - 1
	pos := t * float64(segments)
	idx := int(pos)
	if idx > segments-1 {
		idx = segments - 1
	}
	if idx < 0 {
		idx = 0
	}
	frac := pos - float64(idx)
	from, to := GradientStops[idx], GradientStops[idx+1]

	var c [3]int
	for k := range c {
		c[k] = int(from[k] + (to[k]-from[k])*frac + 0.5)
	}
	return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
}
