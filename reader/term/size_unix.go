//go:build !windows

package term

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

// GetSize reads the pane size, including pixel dimensions, from TIOCGWINSZ.
func GetSize(f *os.File) Size {
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return fill(0, 0, 0, 0)
	}
	return fill(int(ws.Col), int(ws.Row), int(ws.Xpixel), int(ws.Ypixel))
}

// Resizes delivers a value on every SIGWINCH. stop releases the signal.
func Resizes() (<-chan struct{}, func()) {
	sig := make(chan os.Signal, 1)
	out := make(chan struct{}, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-sig:
				select {
				case out <- struct{}{}:
				default:
				}
			case <-done:
				return
			}
		}
	}()
	return out, func() {
		signal.Stop(sig)
		close(done)
	}
}
