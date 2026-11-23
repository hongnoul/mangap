package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/color"
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

// homeArtStyled is homeArt with a red ring and a white capital "P" drawn
// over the figure. Braille cells are 2x4 dots, and dots are roughly square,
// so the circle is computed in dot space.
var homeArtStyled = overlayLogo(homeArt)

var brailleBit = [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

func overlayLogo(art []string) []string {
	rows := make([][]rune, len(art))
	for i, l := range art {
		rows[i] = []rune(l)
	}
	if len(rows) == 0 {
		return art
	}
	wDots, hDots := float64(len(rows[0])*2), float64(len(rows)*4)
	cx, cy := wDots/2, hDots/2
	r := min(wDots, hDots) / 2 * 0.86
	inner := r * 0.78
	// ring is the red donut band.
	ring := func(x, y float64) bool {
		d2 := (x-cx)*(x-cx) + (y-cy)*(y-cy)
		return d2 <= r*r && d2 >= inner*inner
	}
	// pDot is a solid capital P: a full-height stem plus a bowl on the
	// upper half with a counter cut out.
	top, bot := cy-0.5*r, cy+0.5*r
	stemL, stemR := cx-0.30*r, cx-0.08*r
	bowlCY, bowlR, holeR := cy-0.22*r, 0.28*r, 0.11*r
	pDot := func(x, y float64) bool {
		if y < top || y > bot {
			return false
		}
		if x >= stemL && x <= stemR {
			return true
		}
		if x < stemR || y > bowlCY+bowlR {
			return false
		}
		bx := cx + 0.02*r // bowl center, flat side joins the stem
		dx, dy := x-bx, y-bowlCY
		if x < bx {
			dx = 0
		}
		d2 := dx*dx + dy*dy
		return d2 <= bowlR*bowlR && !(d2 < holeR*holeR && x > stemR+0.06*r)
	}
	red := lipgloss.NewStyle().Foreground(color.Red)
	white := lipgloss.NewStyle().Foreground(color.White)
	out := make([]string, len(rows))
	for i, row := range rows {
		var sb strings.Builder
		for j, c := range row {
			var ringBits, pBits rune
			touched := false
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					x, y := float64(j*2+dx)+0.5, float64(i*4+dy)+0.5
					if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
						touched = true
					}
					if ring(x, y) {
						ringBits |= brailleBit[dy][dx]
					} else if pDot(x, y) {
						pBits |= brailleBit[dy][dx]
					}
				}
			}
			// Original dots outside the ring and P, so the girl shows
			// through the hole.
			var orig rune
			if c >= blankCell && c <= blankCell+0xFF {
				orig = (c - blankCell) &^ (ringBits | pBits)
			}
			switch {
			case !touched:
				sb.WriteRune(c)
			case pBits != 0:
				sb.WriteString(white.Render(string(blankCell + pBits)))
			case ringBits != 0:
				sb.WriteString(red.Render(string(blankCell + ringBits)))
			default:
				sb.WriteRune(blankCell + orig)
			}
		}
		out[i] = sb.String()
	}
	return out
}
