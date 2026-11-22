// Package term owns the terminal while the reader runs: raw mode, the
// alternate screen, cursor, pane size in cells and pixels, and resize
// notifications. Platform specifics live in size_*.go.
package term

import (
	"os"
	"strings"

	xterm "golang.org/x/term"
)

// Size is the pane: cells plus pixels per cell.
type Size struct {
	Cols, Rows   int
	CellW, CellH int
}

// Default cell pixel size when the terminal reports none (pdfpane's).
const (
	defaultCellW = 9
	defaultCellH = 18
)

// Session is an acquired terminal. Restore must run on every exit path.
type Session struct {
	in, out *os.File
	state   *xterm.State
}

// Acquire puts in into raw mode and out onto the alternate screen with the
// cursor hidden.
func Acquire(in, out *os.File) (*Session, error) {
	st, err := xterm.MakeRaw(int(in.Fd()))
	if err != nil {
		return nil, err
	}
	s := &Session{in: in, out: out, state: st}
	_, _ = out.WriteString("\x1b[?1049h\x1b[?25l\x1b[2J\x1b[H")
	return s, nil
}

// Restore leaves the alternate screen, shows the cursor, and restores the
// saved terminal mode. extra is written first (image deletes), so the
// terminal frees its image store before the screen switches back.
func (s *Session) Restore(extra []byte) {
	if s == nil || s.state == nil {
		return
	}
	if len(extra) > 0 {
		_, _ = s.out.Write(extra)
	}
	_, _ = s.out.WriteString("\x1b[0m\x1b[2J\x1b[?25h\x1b[?1049l")
	_ = xterm.Restore(int(s.in.Fd()), s.state)
	s.state = nil
}

// SupportsKittyGraphics mirrors gwae's host_supports_kitty_graphics
// (and pdfpane's): MANGAP_KITTY_GRAPHICS override, then KITTY_WINDOW_ID,
// then TERM/TERM_PROGRAM. Inside gwae the pane env still carries the outer
// terminal's TERM, so this stays accurate in the mux.
func SupportsKittyGraphics() bool {
	if v, ok := os.LookupEnv("MANGAP_KITTY_GRAPHICS"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		}
		return false
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	t := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(t, "kitty") || strings.Contains(t, "ghostty") {
		return true
	}
	p := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	return strings.Contains(p, "kitty") || strings.Contains(p, "ghostty")
}

func fill(cols, rows, pxW, pxH int) Size {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	cw, ch := defaultCellW, defaultCellH
	if pxW > 0 {
		cw = pxW / cols
	}
	if pxH > 0 {
		ch = pxH / rows
	}
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	return Size{Cols: cols, Rows: rows, CellW: cw, CellH: ch}
}
