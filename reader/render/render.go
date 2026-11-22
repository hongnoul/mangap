// Package render turns one decoded manga page into the PNG that fills the
// pane: aspect-fit, centered, black gutters, optional spread,
// progress thumb in the gutter, and a 1-channel PNG whenever the result is
// neutral gray (most manga), which is a quarter of the samples to deflate
// and for the terminal to inflate.
//
// Ported from pdfpane's Renderer.swift with the PDF rasterizer replaced by
// Go image decoding and scaling. Everything here is pure Go, so it runs on
// every OS.
package render

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
)

// Job is one render request.
type Job struct {
	// Pages to compose left to right into one sheet: one page, or two in
	// spread view (already in display order: RTL callers pass the later
	// page first).
	Pages []image.Image
	// W, H is the raster size in pixels (the pane's pixel box).
	W, H int
	// Rotation in quarter turns clockwise, 0-3.
	Rotation int
	// Progress thumb: page index (0-based) of Total. Total <= 1 hides it.
	Index, Total int
	// RTL puts page 1 of the thumb on the right (horizontal bar) to match
	// reading direction. The vertical bar is direction-free.
	RTL bool
	// NativeDensity rasters an upscaled page at its own pixel density
	// instead of the pane's (floored at minNativeUnit of full), and the
	// kitty placement, which always spans the full cell grid, upscales it
	// in the terminal. pdfpane's scan fast path: same pixels on screen,
	// a fraction of the bytes to scale, encode, send, and decode.
	NativeDensity bool
}

// minNativeUnit floors the native-density raster at half the pane's
// density, so a tiny source still looks no worse than half-res.
const minNativeUnit = 0.5

// Result is the encoded frame.
type Result struct {
	PNG        []byte
	W, H       int
	Gray       bool
	ScaleMs    float64
	EncodeMs   float64
	TotalMs    float64
	SourceSize image.Point
}

// Decode decodes a page image (jpeg, png, gif, webp).
func Decode(b []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(b))
	return img, err
}

var encoderPool = &bufferPool{}

type bufferPool struct{ p sync.Pool }

func (b *bufferPool) Get() *png.EncoderBuffer {
	if v, ok := b.p.Get().(*png.EncoderBuffer); ok {
		return v
	}
	return nil
}
func (b *bufferPool) Put(e *png.EncoderBuffer) { b.p.Put(e) }

var encoder = png.Encoder{CompressionLevel: png.BestSpeed, BufferPool: encoderPool}

// Render composes, fits, and encodes one frame.
func Render(j Job) (*Result, error) {
	if len(j.Pages) == 0 || j.W <= 0 || j.H <= 0 {
		return nil, errors.New("render: empty job")
	}
	t0 := time.Now()

	// Every page converts to gray when it is neutral, which most manga is.
	// Working in image.Gray quarters the memory traffic of every pass
	// below and makes the PNG 1-channel for free.
	gray := true
	for _, p := range j.Pages {
		if !isNeutral(p) {
			gray = false
			break
		}
	}

	// Sheet: pages side by side, each vertically centered, no seam.
	sheetW, sheetH := 0, 0
	for _, p := range j.Pages {
		b := p.Bounds()
		sheetW += b.Dx()
		if b.Dy() > sheetH {
			sheetH = b.Dy()
		}
	}
	if sheetW <= 0 || sheetH <= 0 {
		return nil, errors.New("render: empty page")
	}

	turns := ((j.Rotation % 4) + 4) % 4
	rw, rh := sheetW, sheetH
	if turns%2 == 1 {
		rw, rh = sheetH, sheetW
	}

	// Native density: when the sheet would be enlarged to fill the pane,
	// shrink the whole raster (pane aspect kept) until the sheet maps 1:1,
	// floored at minNativeUnit. Only worth it when meaningfully smaller.
	unit := 1.0
	if j.NativeDensity {
		full := float64(j.W) / float64(rw)
		if s := float64(j.H) / float64(rh); s < full {
			full = s
		}
		if u := 1 / full; u < 0.9 {
			if u < minNativeUnit {
				u = minNativeUnit
			}
			unit = u
			j.W = imax(int(float64(j.W)*unit+0.5), 1)
			j.H = imax(int(float64(j.H)*unit+0.5), 1)
		}
	}
	f := fit(rw, rh, j.W, j.H)

	var dst draw.Image
	if gray {
		dst = image.NewGray(image.Rect(0, 0, j.W, j.H))
	} else {
		rgba := image.NewRGBA(image.Rect(0, 0, j.W, j.H))
		// Opaque black gutters; NewRGBA is transparent black.
		for i := 3; i < len(rgba.Pix); i += 4 {
			rgba.Pix[i] = 0xff
		}
		dst = rgba
	}

	// Each page's slot inside the fitted rect, in the unrotated sheet's
	// coordinates scaled to the fit, then rotated into the frame.
	x := 0
	var slots []image.Rectangle
	for _, p := range j.Pages {
		b := p.Bounds()
		y := (sheetH - b.Dy()) / 2
		slot := image.Rect(x, y, x+b.Dx(), y+b.Dy())
		x += b.Dx()
		dr := sheetToDevice(slot, sheetW, sheetH, f, turns)
		scaleInto(dst, dr, p, turns)
		slots = append(slots, dr)
	}
	tScale := time.Now()

	drawThumb(dst, f, j, unit)

	var buf bytes.Buffer
	buf.Grow(j.W * j.H / 4)
	if err := encodePNG(&buf, dst, true); err != nil {
		return nil, err
	}
	tEnd := time.Now()

	src := j.Pages[0].Bounds().Size()
	return &Result{
		PNG:        buf.Bytes(),
		W:          j.W,
		H:          j.H,
		Gray:       gray,
		ScaleMs:    ms(tScale.Sub(t0)),
		EncodeMs:   ms(tEnd.Sub(tScale)),
		TotalMs:    ms(tEnd.Sub(t0)),
		SourceSize: src,
	}, nil
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// fit returns the aspect-fit rect of a w x h footprint inside W x H,
// rounded to whole pixels and centered (pdfpane: a fractional edge would
// antialias against the gutter and leave a fringe).
func fit(w, h, W, H int) image.Rectangle {
	sx := float64(W) / float64(w)
	sy := float64(H) / float64(h)
	s := sx
	if sy < s {
		s = sy
	}
	pw := clamp(int(float64(w)*s+0.5), 1, W)
	ph := clamp(int(float64(h)*s+0.5), 1, H)
	ox := (W - pw) / 2
	oy := (H - ph) / 2
	return image.Rect(ox, oy, ox+pw, oy+ph)
}

// sheetToDevice maps a slot of the unrotated sheet into the frame: scale
// the sheet into the fitted rect f (whose aspect already accounts for the
// rotation), then rotate quarter turns clockwise about f's center.
func sheetToDevice(slot image.Rectangle, sheetW, sheetH int, f image.Rectangle, turns int) image.Rectangle {
	// Scale from sheet space into the unrotated footprint of f.
	uw, uh := f.Dx(), f.Dy()
	if turns%2 == 1 {
		uw, uh = uh, uw
	}
	sx := float64(uw) / float64(sheetW)
	sy := float64(uh) / float64(sheetH)
	x0 := int(float64(slot.Min.X)*sx + 0.5)
	x1 := int(float64(slot.Max.X)*sx + 0.5)
	y0 := int(float64(slot.Min.Y)*sy + 0.5)
	y1 := int(float64(slot.Max.Y)*sy + 0.5)
	// Rotate the rect inside the uw x uh footprint, clockwise.
	var r image.Rectangle
	switch turns {
	case 0:
		r = image.Rect(x0, y0, x1, y1)
	case 1:
		r = image.Rect(uh-y1, x0, uh-y0, x1)
	case 2:
		r = image.Rect(uw-x1, uh-y1, uw-x0, uh-y0)
	case 3:
		r = image.Rect(y0, uw-x1, y1, uw-x0)
	}
	return r.Add(f.Min).Intersect(f)
}

// imax is max for go1.18 (the builtin arrived in go1.21).
func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
