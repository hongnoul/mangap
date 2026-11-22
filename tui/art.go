package tui

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/hongnoul/mangaz/style"
)

// homeArtRaw is the home screen art (braille). Common leading and trailing
// blank braille cells are trimmed at init so it centers cleanly.
const homeArtRaw = `
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⡠⠔⠒⠊⠉⠉⠉⠉⠙⠋⠛⠻⠶⣢⢄⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⠔⠊⠁⠉⠀⠀⠀⠀⠀⠀⠀⠀⢰⡆⠀⠀⣈⣑⢌⠳⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣜⠁⣀⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⡊⣟⢿⣷⡌⠳⡄⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣿⡷⠊⡁⠀⠀⠀⠀⣀⣠⠤⠤⠖⠒⠒⠒⠒⠐⠒⣓⡛⠊⠈⢷⢤⠜⢆⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣿⣿⣷⡯⠟⢀⡄⠀⠈⠁⠀⠀⠀⠀⠀⠀⠀⠀⣀⣀⣙⣁⣂⣀⡀⠹⠕⡢⢝⡢⢄⡀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣾⣿⣿⣯⣴⠞⠁⠀⠀⣤⠀⠀⢀⣀⡠⣴⣶⡋⠉⠩⢿⡧⠤⠹⣿⣿⣿⣿⣶⣶⣬⣝⣻⢦⡀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢰⣿⣿⡿⠋⠀⠀⠀⠀⢀⣩⢴⣾⠟⠛⠙⠂⡇⠀⠀⠀⠈⢷⠐⠒⠻⣿⣿⣿⣿⣿⣿⣿⣿⣷⢵
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣼⡿⠋⠀⠀⠀⣀⣴⣾⡿⣿⡿⢿⣿⣶⡀⠈⠀⠀⠀⠀⠀⠀⢠⣤⣤⡘⡏⣿⣿⣿⣿⣿⣿⣿⡏
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣰⠫⠀⠀⣠⣴⣿⣿⣿⣿⡿⠋⣼⣿⣿⣽⣆⠀⠀⠀⠀⠀⠀⠀⣽⣿⣿⣟⣆⢹⣿⣿⣿⣿⣿⠏⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⡜⢁⢀⣴⣿⣿⣿⣿⣿⣿⣿⢧⣄⢻⢟⣿⣿⠏⠀⠀⠀⠀⠀⠀⠀⣿⢿⣿⡿⢸⢸⣿⣿⣿⡟⠁⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⡎⣠⣴⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⠀⠉⠀⢀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠙⠋⠁⠈⢸⡞⠋⠈⢷⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠸⡶⣿⣿⣿⣿⣿⣿⠷⢾⣿⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠁⠀⠀⡼⡇⠀⠀⠈⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠛⠿⠿⣿⣿⡿⣇⠘⣿⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣰⣧⠁⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⣶⠖⠒⠒⠛⠭⠷⣿⣿⣿⡀⠀⠀⠀⠀⠀⠀⠀⠒⠙⠀⠀⠀⠀⠀⢀⡠⣾⣿⣿⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⠀⠀⢀⣀⠀⠀⠀⠸⣿⣿⣿⣶⣦⣤⣄⣀⣀⠀⠀⢀⣀⣀⣤⣴⣾⣉⣀⣿⣿⡏⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢧⠀⠀⠈⠻⢷⣤⡀⠀⢿⢿⡿⣿⣿⣿⣿⣻⣶⡭⣹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⣷⡤⣀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⣆⠠⢀⠀⠀⠈⢙⣦⣁⠈⠧⣿⣿⣿⡉⠻⢿⣷⣾⣿⣿⣿⣿⣿⣿⣿⠿⠿⠟⠛⢋⢳⠁⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠠⣾⡟⣿⡩⡤⠖⢾⠉⠀⠊⠳⢬⣸⣿⣿⣷⠂⠄⠙⢿⢿⣿⣿⣿⣿⣿⡄⠀⠀⠀⣀⣦⠃⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢏⠀⠀⢰⠛⠂⠘⡆⠀⢸⣿⣷⠙⣿⣿⣿⡇⠀⣠⠵⡭⠛⠿⡟⣿⣿⣿⡤⠖⠻⣻⡃⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⡄⢀⡏⠀⠐⠀⢹⡀⢸⣿⣟⠁⣿⣿⡟⣿⠚⠁⠀⢯⣀⠀⡎⠹⣿⣿⣿⡀⠄⠐⡇⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠹⡼⠀⠀⠀⠀⠀⣇⢸⣿⡇⢀⣾⣿⣿⣿⣄⠀⠀⢸⡹⣿⣹⠯⣿⣿⣿⡇⠀⡸⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⠋⠀⠀⠀⠀⠀⢸⣾⣿⣠⣿⡼⣿⣿⣿⣧⠤⣄⡀⣇⠙⠋⣇⠘⣿⣿⣿⣶⠃⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⡎⠀⠀⠀⠀⠀⠀⠨⣿⣿⣿⡇⢴⣿⣿⣿⣿⣇⣸⡀⢸⠂⠀⢸⣶⣿⣿⣿⣟⡄⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣼⠰⠀⠀⠀⠠⢀⠀⣿⣿⣿⣯⣯⠉⣿⣿⣿⣿⡿⡉⣇⠌⡗⢛⢣⣽⣿⣿⣿⣿⣹⠀⠀⠀⠀⠀⠀⠀
`

const blankCell = '\u2800'

var homeArt = trimArt(homeArtRaw)

// trimArt drops the common blank-cell margin on both sides and pads lines
// to equal width, so the block centers on its visible pixels.
func trimArt(raw string) []string {
	var rows [][]rune
	for _, l := range strings.Split(strings.Trim(raw, "\n"), "\n") {
		rows = append(rows, []rune(l))
	}
	width := 0
	for _, r := range rows {
		width = imax(width, len(r))
	}
	for i := range rows {
		for len(rows[i]) < width {
			rows[i] = append(rows[i], blankCell)
		}
	}
	blankCol := func(c int) bool {
		for _, r := range rows {
			if r[c] != blankCell && r[c] != ' ' {
				return false
			}
		}
		return true
	}
	lo, hi := 0, width
	for lo < hi && blankCol(lo) {
		lo++
	}
	for hi > lo && blankCol(hi-1) {
		hi--
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = string(r[lo:hi])
	}
	return out
}

// homeArtColored is homeArt painted with homeArtColor, built once.
var (
	homeArtOnce sync.Once
	homeArtStr  string
)

func homeArtColored() string {
	homeArtOnce.Do(func() { homeArtStr = style.Paint(homeArt, homeArtColor) })
	return homeArtStr
}

// Palette for the home art (Chito from Girls' Last Tour). Braille dots are
// lit where the source image was dark, so dense cells are her black hair
// and eyes, sparse cells are line art. Colors are lifted so they still read
// on a black terminal.
const (
	artEye        = "#3a4170" // dark eyes
	artEyeShine   = "#9fb0e0"
	artSkin       = "#f0c2a2" // face lines
	artMouth      = "#e0908c"
	artHelmet     = "#8d925e" // olive Brodie helmet
	artHelmetLine = "#b9bc84"
	artJacket     = "#6c7f4c" // green military jacket
	artJacketLine = "#9aae70"
)

// dots counts the raised dots in a braille cell.
func dots(r rune) int {
	if r < 0x2800 || r > 0x28ff {
		return 8
	}
	n := 0
	for b := r - 0x2800; b != 0; b &= b - 1 {
		n++
	}
	return n
}

// homeArtColor paints the art by region, like coloring in line art.
func homeArtColor(x, y int, r rune) string {
	d := dots(r)
	dense := d >= 5
	in := func(y0, y1, x0, x1 int) bool { return y >= y0 && y <= y1 && x >= x0 && x <= x1 }
	hair := func() string { return hairShade(x, y, d) }

	switch {
	// Eyes.
	case in(7, 9, 13, 19) || in(10, 10, 15, 17):
		if d <= 2 {
			return artEyeShine
		}
		return artEye
	case in(7, 9, 25, 29) || in(10, 10, 25, 29):
		if d <= 2 {
			return artEyeShine
		}
		return artEye

	// Helmet: dome and brim are thin lines above the hair.
	case y <= 2:
		return artHelmetLine
	case y <= 4 && x >= 9:
		if y == 4 && x >= 22 {
			return artHelmetLine // brim edge
		}
		return artHelmet
	case in(5, 6, 10, 12) && !dense:
		return artHelmet

	// Jaw / cheek outline on the right side of the face.
	case in(10, 13, 29, 32) && !dense:
		return artSkin

	// Hair: left twintail mass, bangs, right side lock.
	case y <= 12 && x <= 12:
		return hair()
	case in(5, 6, 13, 28):
		return hair()
	case y <= 14 && x >= 27:
		return hair()
	case in(13, 13, 10, 13):
		return hair()

	// Sleeve / arm reaching across on the left.
	case x <= 9 && y >= 13:
		return artJacketLine

	// Face lines.
	case in(13, 13, 19, 23):
		return artMouth
	case y <= 13:
		return artSkin

	// Body: twintails hang over the jacket. Dense = hair.
	case x >= 10 && dense:
		return hair()
	case !dense && x >= 10:
		return artJacketLine
	default:
		return artJacket
	}
}

// hairRamp runs from deep shadow to specular shine for Chito's black hair.
// Blue-violet keeps "black" readable on a black terminal.
var hairRamp = [][3]float64{
	{0x24, 0x28, 0x46}, // deep shadow
	{0x3a, 0x43, 0x6e}, // base
	{0x55, 0x62, 0x96}, // mid
	{0x86, 0x9a, 0xd4}, // highlight
	{0xd2, 0xdc, 0xf6}, // specular
}

var hairTip = [3]float64{0x6e, 0x5c, 0x9a} // violet bounce light at the ends

// litAt reports the dot count of the art cell at (x, y), 0 off-grid.
func litAt(x, y int) int {
	if y < 0 || y >= len(homeArt) {
		return 0
	}
	row := []rune(homeArt[y])
	if x < 0 || x >= len(row) || row[x] == blankCell || row[x] == ' ' {
		return 0
	}
	return dots(row[x])
}

// hairShade shades a hair cell like painted anime hair: occlusion in the
// mass, rim light on edges, a shadow under the helmet brim, a glossy
// "angel ring" band following the skull, per-strand streaks, and violet
// bounce light on the hanging tips.
func hairShade(x, y, d int) string {
	// Fill of the 3x3 neighborhood: high inside the mass, low on edges.
	fill := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			fill += litAt(x+dx, y+dy)
		}
	}
	f := float64(fill) / 72

	s := 0.62 - 0.5*f // occlusion: dense interior sinks into shadow
	if litAt(x-1, y) == 0 || litAt(x+1, y) == 0 {
		s += 0.22 // rim light where the silhouette breaks
	}
	if d <= 3 {
		s += 0.15 // loose wisps catch light
	}

	// Shadow cast by the helmet brim.
	if y <= 6 {
		s -= 0.3 * (1 - float64(y-3)/4)
	}

	// Angel ring: an arc around the head center. Cells are ~2x tall, so
	// halve the horizontal distance.
	cx, cy := 20.0, 11.5
	dx, dy := (float64(x)-cx)/2, float64(y)-cy
	r := math.Sqrt(dx*dx + dy*dy)
	ring := math.Exp(-math.Pow((r-8.3)/0.9, 2))
	if y <= 12 {
		s += 0.6 * ring
	}

	// Hanging twintails: each strand is a vertical band with its own
	// sheen that rolls off downward, so strands read separately.
	if y >= 13 {
		band := math.Sin(float64(x)*1.7+float64(x%3)) * 0.5
		roll := math.Exp(-math.Pow((float64(y)-16)/2.5, 2))
		s += 0.22*band + 0.25*roll*math.Max(0, band)
	}

	// Fine per-column jitter so neighboring dots don't look flat.
	jit := float64((x*73+y*29+11)%7)/6 - 0.5
	s += 0.08 * jit

	s = math.Max(0, math.Min(1, s))
	c := rampAt(hairRamp, s)

	// Bounce light toward the ends of the twintails.
	if y >= 15 {
		t := math.Min(1, float64(y-15)/8) * 0.45
		for k := range c {
			c[k] += (hairTip[k] - c[k]) * t
		}
	}
	return fmt.Sprintf("#%02x%02x%02x", int(c[0]+0.5), int(c[1]+0.5), int(c[2]+0.5))
}

// rampAt samples a color ramp at t in [0, 1].
func rampAt(ramp [][3]float64, t float64) [3]float64 {
	pos := t * float64(len(ramp)-1)
	i := int(pos)
	if i >= len(ramp)-1 {
		i = len(ramp) - 2
	}
	frac := pos - float64(i)
	var c [3]float64
	for k := range c {
		c[k] = ramp[i][k] + (ramp[i+1][k]-ramp[i][k])*frac
	}
	return c
}
