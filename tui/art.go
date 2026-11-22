package tui

import "strings"

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
