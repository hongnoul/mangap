//go:build windows

package term

import (
	"os"
	"time"

	xterm "golang.org/x/term"
)

// GetSize reads the console size. Windows consoles report no pixel size,
// so cells use the default pixel size.
func GetSize(f *os.File) Size {
	cols, rows, err := xterm.GetSize(int(f.Fd()))
	if err != nil {
		return fill(0, 0, 0, 0)
	}
	return fill(cols, rows, 0, 0)
}

// Resizes polls the console size (Windows has no SIGWINCH).
func Resizes() (<-chan struct{}, func()) {
	out := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		last := GetSize(os.Stdout)
		t := time.NewTicker(150 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if s := GetSize(os.Stdout); s != last {
					last = s
					select {
					case out <- struct{}{}:
					default:
					}
				}
			case <-done:
				return
			}
		}
	}()
	return out, func() { close(done) }
}
