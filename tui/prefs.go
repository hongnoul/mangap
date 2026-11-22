package tui

import (
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/reader"
	"github.com/hongnoul/mangap/reader/term"
	"github.com/spf13/viper"
)

// savePref persists one config value.
func savePref(k string, v any) error {
	viper.Set(k, v)
	err := viper.WriteConfig()
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return viper.SafeWriteConfig()
	}
	return err
}

// useTerminalViewer reports whether chapters open in the built-in reader.
func useTerminalViewer() bool {
	return reader.UseTerminal(viper.GetString(key2.ReaderViewer))
}

// toggleViewer flips between the in-terminal and external viewer. Without
// kitty graphics only the external viewer works, so nothing changes.
func (b *statefulBubble) toggleViewer() error {
	if !term.SupportsKittyGraphics() {
		return nil
	}
	v := reader.ViewerExternal
	if !useTerminalViewer() {
		v = reader.ViewerTerminal
	}
	return savePref(key2.ReaderViewer, v)
}

// toggleThumbnails flips thumbnail loading. Off drops the images the
// terminal holds and the lists turn into one dense line per entry.
func (b *statefulBubble) toggleThumbnails() error {
	on := !viper.GetBool(key2.TUIThumbnails)
	if !on {
		b.thumbs.Clear()
		b.preview.Clear()
		b.recent.Clear()
	}
	return savePref(key2.TUIThumbnails, on)
}
