package tui

import (
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
	artHair       = "#4f5d86" // black hair with a blue sheen
	artHairLight  = "#8090bd" // hair strands / highlights
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
	hair := func() string {
		if dense {
			return artHair
		}
		return artHairLight
	}

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
		return artHair
	case !dense && x >= 10:
		return artJacketLine
	default:
		return artJacket
	}
}
