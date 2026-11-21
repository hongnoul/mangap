package render

import (
	"image"
	"image/color"
	"image/draw"
)

// neutralTol is the largest channel spread still counted as gray. Manga
// JPEGs saved as color carry a few levels of chroma noise from encoding,
// and treating those pages as color would quadruple every pass and the
// PNG for no visible difference.
const neutralTol = 6

// isNeutral reports whether every pixel of img is gray within neutralTol.
// Exits at the first colored pixel, so color pages pay almost nothing.
func isNeutral(img image.Image) bool {
	switch m := img.(type) {
	case *image.Gray, *image.Gray16:
		return true
	case *image.YCbCr:
		lo, hi := uint8(128-neutralTol/2), uint8(128+neutralTol/2)
		for _, c := range m.Cb {
			if c < lo || c > hi {
				return false
			}
		}
		for _, c := range m.Cr {
			if c < lo || c > hi {
				return false
			}
		}
		return true
	case *image.Paletted:
		// Only entries actually used matter: palettes often carry
		// unused colored slots.
		var used [256]bool
		for _, ix := range m.Pix {
			used[ix] = true
		}
		for i, c := range m.Palette {
			if i < 256 && used[i] && !grayColor(c) {
				return false
			}
		}
		return true
	case *image.NRGBA:
		return neutralPix(m.Pix)
	case *image.RGBA:
		return neutralPix(m.Pix)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !grayColor(img.At(x, y)) {
				return false
			}
		}
	}
	return true
}

func neutralPix(p []uint8) bool {
	for i := 0; i+3 < len(p); i += 4 {
		r, g, b := int(p[i]), int(p[i+1]), int(p[i+2])
		if spread3(r, g, b) > neutralTol {
			return false
		}
	}
	return true
}

func grayColor(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return spread3(int(r>>8), int(g>>8), int(b>>8)) <= neutralTol
}

func spread3(r, g, b int) int {
	mx, mn := r, r
	if g > mx {
		mx = g
	}
	if b > mx {
		mx = b
	}
	if g < mn {
		mn = g
	}
	if b < mn {
		mn = b
	}
	return mx - mn
}

// toGray converts img to a tightly packed 8-bit gray buffer. Fast paths
// cover what manga sources actually serve: gray and color JPEG (Y plane
// is already luma), paletted PNG, and RGBA/NRGBA PNG.
func toGray(img image.Image) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	switch m := img.(type) {
	case *image.Gray:
		if m.Stride == w && m.Rect.Min == (image.Point{}) {
			return m
		}
		out := image.NewGray(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			copy(out.Pix[y*w:(y+1)*w], m.Pix[m.PixOffset(b.Min.X, b.Min.Y+y):])
		}
		return out
	case *image.YCbCr:
		out := image.NewGray(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			copy(out.Pix[y*w:(y+1)*w], m.Y[m.YOffset(b.Min.X, b.Min.Y+y):])
		}
		return out
	case *image.Paletted:
		var lut [256]uint8
		for i, c := range m.Palette {
			if i < 256 {
				lut[i] = color.GrayModel.Convert(c).(color.Gray).Y
			}
		}
		out := image.NewGray(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			src := m.Pix[m.PixOffset(b.Min.X, b.Min.Y+y):]
			dst := out.Pix[y*w : (y+1)*w]
			for x := range dst {
				dst[x] = lut[src[x]]
			}
		}
		return out
	case *image.NRGBA:
		return rgbaToGray(m.Pix, m.Stride, m.PixOffset(b.Min.X, b.Min.Y), w, h, true)
	case *image.RGBA:
		return rgbaToGray(m.Pix, m.Stride, m.PixOffset(b.Min.X, b.Min.Y), w, h, false)
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

// rgbaToGray takes the luma of each pixel composited over white (a
// transparent PNG margin should read as paper, not ink).
func rgbaToGray(pix []uint8, stride, off, w, h int, straight bool) *image.Gray {
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		row := pix[off+y*stride:]
		dst := out.Pix[y*w : (y+1)*w]
		for x := range dst {
			r, g, b, a := int(row[x*4]), int(row[x*4+1]), int(row[x*4+2]), int(row[x*4+3])
			l := (r*77 + g*150 + b*29) >> 8
			if a != 255 {
				if straight {
					l = (l*a + 255*(255-a)) / 255
				} else {
					l = l + (255 - a)
				}
				if l > 255 {
					l = 255
				}
			}
			dst[x] = uint8(l)
		}
	}
	return out
}

// toRGBA converts img to opaque RGBA (alpha composited over white).
func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	switch m := img.(type) {
	case *image.YCbCr, *image.Gray:
		// Opaque sources: draw.Src takes the stdlib's YCbCr/Gray fast
		// paths and skips compositing.
		draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
		return out
	case *image.Paletted:
		var lut [256][4]uint8
		for i := range lut {
			lut[i] = [4]uint8{255, 255, 255, 255}
		}
		for i, c := range m.Palette {
			if i >= 256 {
				break
			}
			r, g, bl, a := c.RGBA()
			// Over white: premultiplied color + (1-a) * white.
			w := 0xffff - a
			lut[i] = [4]uint8{uint8((r + w) >> 8), uint8((g + w) >> 8), uint8((bl + w) >> 8), 255}
		}
		w := b.Dx()
		for y := 0; y < b.Dy(); y++ {
			src := m.Pix[m.PixOffset(b.Min.X, b.Min.Y+y):]
			dst := out.Pix[y*out.Stride:]
			for x := 0; x < w; x++ {
				copy(dst[x*4:x*4+4], lut[src[x]][:])
			}
		}
		return out
	}
	for i := range out.Pix {
		out.Pix[i] = 0xff
	}
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Over)
	return out
}

// rotateGray rotates a gray buffer by quarter turns clockwise.
func rotateGray(src *image.Gray, turns int) *image.Gray {
	if turns == 0 {
		return src
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	ow, oh := w, h
	if turns%2 == 1 {
		ow, oh = h, w
	}
	out := image.NewGray(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		row := src.Pix[y*src.Stride:]
		for x := 0; x < w; x++ {
			var dx, dy int
			switch turns {
			case 1:
				dx, dy = h-1-y, x
			case 2:
				dx, dy = w-1-x, h-1-y
			case 3:
				dx, dy = y, w-1-x
			}
			out.Pix[dy*ow+dx] = row[x]
		}
	}
	return out
}

// rotateRGBA rotates an RGBA buffer by quarter turns clockwise.
func rotateRGBA(src *image.RGBA, turns int) *image.RGBA {
	if turns == 0 {
		return src
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	ow, oh := w, h
	if turns%2 == 1 {
		ow, oh = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch turns {
			case 1:
				dx, dy = h-1-y, x
			case 2:
				dx, dy = w-1-x, h-1-y
			case 3:
				dx, dy = y, w-1-x
			}
			copy(out.Pix[(dy*ow+dx)*4:(dy*ow+dx)*4+4], src.Pix[y*src.Stride+x*4:])
		}
	}
	return out
}

// scaleInto draws page p, rotated by turns, resampled into rect r of dst.
func scaleInto(dst draw.Image, r image.Rectangle, p image.Image, turns int) {
	if r.Empty() {
		return
	}
	switch d := dst.(type) {
	case *image.Gray:
		src := rotateGray(toGray(p), turns)
		resample(d.Pix, d.Stride, r, src.Pix, src.Stride, src.Rect.Dx(), src.Rect.Dy(), 1)
	case *image.RGBA:
		src := rotateRGBA(toRGBA(p), turns)
		resample(d.Pix, d.Stride, r, src.Pix, src.Stride, src.Rect.Dx(), src.Rect.Dy(), 4)
	}
}

// weights are the fixed-point (1<<14) filter taps of one output sample.
type weights struct {
	start int
	w     []int32
}

const wBits = 14

// contributions builds a triangle filter from src to dst samples. When
// shrinking, the triangle widens to the source footprint of one output
// sample, which area-averages screentone instead of aliasing it into
// moire. When enlarging, it is plain bilinear.
func contributions(src, dst int) []weights {
	scale := float64(dst) / float64(src)
	support := 1.0
	if scale < 1 {
		support = 1 / scale
	}
	out := make([]weights, dst)
	for i := range out {
		center := (float64(i)+0.5)/scale - 0.5
		lo := int(center - support + 1)
		if float64(lo) <= center-support {
			lo++
		}
		if lo < 0 {
			lo = 0
		}
		hi := int(center + support)
		if hi > src-1 {
			hi = src - 1
		}
		if hi < lo {
			hi = lo
		}
		ws := make([]float64, hi-lo+1)
		sum := 0.0
		for j := range ws {
			d := (float64(lo+j) - center) / support
			if d < 0 {
				d = -d
			}
			if d < 1 {
				ws[j] = 1 - d
				sum += ws[j]
			}
		}
		iw := make([]int32, len(ws))
		if sum == 0 {
			iw[0] = 1 << wBits
		} else {
			acc := int32(0)
			for j := range ws {
				iw[j] = int32(ws[j] / sum * (1 << wBits))
				acc += iw[j]
			}
			// Push rounding slack onto the largest tap so the weights sum
			// to exactly 1: a flat white page must stay exactly 255.
			best := 0
			for j := range iw {
				if iw[j] > iw[best] {
					best = j
				}
			}
			iw[best] += (1 << wBits) - acc
		}
		out[i] = weights{start: lo, w: iw}
	}
	return out
}

// resample is a separable two-pass resize of a ch-channel 8-bit buffer
// (sw x sh) into rect r of dst. An exact 1:1 size is a straight copy, the
// common case for native-density manga rasters.
func resample(dst []uint8, dstStride int, r image.Rectangle, src []uint8, srcStride, sw, sh, ch int) {
	dw, dh := r.Dx(), r.Dy()
	if dw == sw && dh == sh {
		for y := 0; y < dh; y++ {
			copy(dst[(r.Min.Y+y)*dstStride+r.Min.X*ch:][:dw*ch], src[y*srcStride:][:sw*ch])
		}
		return
	}
	cx := contributions(sw, dw)
	cy := contributions(sh, dh)

	// Horizontal pass: sh rows of dw samples, kept at 16 bits of
	// precision for the vertical pass.
	tmp := make([]int32, sh*dw*ch)
	for y := 0; y < sh; y++ {
		row := src[y*srcStride:]
		out := tmp[y*dw*ch:]
		for x, c := range cx {
			for k := 0; k < ch; k++ {
				var acc int32
				p := c.start*ch + k
				for _, w := range c.w {
					acc += int32(row[p]) * w
					p += ch
				}
				out[x*ch+k] = acc >> (wBits - 8) // 8.8 fixed point
			}
		}
	}
	// Vertical pass.
	for y, c := range cy {
		out := dst[(r.Min.Y+y)*dstStride+r.Min.X*ch:]
		for x := 0; x < dw*ch; x++ {
			var acc int64
			p := c.start*dw*ch + x
			for _, w := range c.w {
				acc += int64(tmp[p]) * int64(w)
				p += dw * ch
			}
			v := (acc + (1 << (wBits + 7))) >> (wBits + 8)
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			out[x] = uint8(v)
		}
	}
}
