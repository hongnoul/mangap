package reader

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hongnoul/mangap/open"
	"github.com/hongnoul/mangap/reader/pages"
	"github.com/hongnoul/mangap/reader/render"
	"github.com/hongnoul/mangap/reader/term"
	"github.com/hongnoul/mangap/where"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// Viewer preference values (reader.viewer).
const (
	ViewerAuto     = "auto"
	ViewerTerminal = "terminal"
	ViewerExternal = "external"
)

// UseTerminal resolves a reader.viewer preference: the in-terminal reader
// when asked for (or auto) and the terminal speaks kitty graphics, else the
// external viewer.
func UseTerminal(pref string) bool {
	if !term.SupportsKittyGraphics() {
		return false
	}
	return strings.ToLower(strings.TrimSpace(pref)) != ViewerExternal
}

// OpenExternal writes src as a PDF in the temp dir, named after name, and
// opens it with the system's default viewer. It does not wait for the
// viewer to exit.
func OpenExternal(src pages.Source, name string) (string, error) {
	imgs := make([]io.Reader, 0, src.Len())
	for i := 0; i < src.Len(); i++ {
		b, err := src.Read(i)
		if err != nil {
			return "", err
		}
		b, err = pdfImage(b)
		if err != nil {
			return "", fmt.Errorf("%s: %w", src.Name(i), err)
		}
		imgs = append(imgs, bytes.NewReader(b))
	}
	if len(imgs) == 0 {
		return "", pages.ErrNoPages
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "" {
		name = "chapter"
	}
	path := filepath.Join(where.Temp(), name+".pdf")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if err := api.ImportImages(nil, f, imgs, nil, nil); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, open.Start(path)
}

// pdfImage passes jpeg and png through and re-encodes anything else
// (webp, gif) as png, which pdfcpu cannot embed directly.
func pdfImage(b []byte) ([]byte, error) {
	switch http.DetectContentType(b) {
	case "image/jpeg", "image/png":
		return b, nil
	}
	img, err := render.Decode(b)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
