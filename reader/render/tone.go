package render

import (
	"image"
	"image/draw"
)

// darkLUT is the gray dark tone: paper (255) becomes exact black so the
// page merges with the gutters, ink (0) lands at 224, a soft gray that
// avoids halation on dark panels (pdfpane's 224/255 cap).
var darkLUT = func() (t [256]uint8) {
	for i := range t {
		t[i] = uint8((224*(255-i) + 127) / 255)
	}
	return
}()

// dimLUT is 85% brightness (pdfpane's photo dim).
var dimLUT = func() (t [256]uint8) {
	for i := range t {
		t[i] = uint8((i*85 + 50) / 100)
	}
	return
}()

// smart is pdfpane's hue-preserving luma flip for color pages, int fixed
// point with divisor 256: x' = (224/255)(255 - hueRotate180(x)). Rows are
// output R,G,B over input R,G,B, each summing to -225, so white maps to
// exactly black and grays stay neutral.
var smart = [3][3]int{
	{129, -322, -32},
	{-96, -97, -32},
	{-96, -322, 193},
}

// applyTone runs the tone pass over rect r only, so the gutters keep
// their black.
func applyTone(dst draw.Image, r image.Rectangle, tone Tone) {
	if tone == Light || r.Empty() {
		return
	}
	switch d := dst.(type) {
	case *image.Gray:
		lut := &darkLUT
		if tone == Dim {
			lut = &dimLUT
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			row := d.Pix[y*d.Stride+r.Min.X : y*d.Stride+r.Max.X]
			for i, v := range row {
				row[i] = lut[v]
			}
		}
	case *image.RGBA:
		for y := r.Min.Y; y < r.Max.Y; y++ {
			row := d.Pix[y*d.Stride+r.Min.X*4 : y*d.Stride+r.Max.X*4]
			for i := 0; i+3 < len(row); i += 4 {
				if tone == Dim {
					row[i] = dimLUT[row[i]]
					row[i+1] = dimLUT[row[i+1]]
					row[i+2] = dimLUT[row[i+2]]
					continue
				}
				in := [3]int{int(row[i]), int(row[i+1]), int(row[i+2])}
				for c := 0; c < 3; c++ {
					v := (smart[c][0]*in[0] + smart[c][1]*in[1] + smart[c][2]*in[2] + 224*256) >> 8
					if v < 0 {
						v = 0
					} else if v > 255 {
						v = 255
					}
					row[i+c] = uint8(v)
				}
			}
		}
	}
}

// Thumb geometry in raster pixels (pdfpane's, at retina density).
const (
	barW         = 6
	barInset     = 4
	barClearance = 4
	minThumb     = 24
	thumbGray    = 107 // 0.42 * 255
)

// drawThumb draws the progress thumb into the leftover gutter: vertical
// in a pillarbox's right gutter (top = first page), horizontal in a
// letterbox's bottom gutter (first page on the reading-start side). A
// gutter too thin to hold it gets nothing: the page always keeps its full
// fitted size.
func drawThumb(dst draw.Image, f image.Rectangle, j Job, unit float64) {
	n := j.Total
	if n <= 1 {
		return
	}
	px := func(v int) int { return imax(int(float64(v)*unit+0.5), 1) }
	barW, barInset, minThumb := imax(px(barW), 2), px(barInset), px(minThumb)
	idx := clamp(j.Index, 0, n-1)
	W, H := j.W, j.H
	minGutter := barInset + barW + px(barClearance)
	var r image.Rectangle
	if W-f.Max.X >= minGutter {
		track := H - 2*barInset
		if track < barW {
			return
		}
		thumb := clamp(track/n, minThumb, track)
		off := idx * (track - thumb) / (n - 1)
		r = image.Rect(W-barInset-barW, barInset+off, W-barInset, barInset+off+thumb)
	} else if H-f.Max.Y >= minGutter {
		track := W - 2*barInset
		if track < barW {
			return
		}
		thumb := clamp(track/n, minThumb, track)
		off := idx * (track - thumb) / (n - 1)
		x0 := barInset + off
		if j.RTL {
			x0 = W - barInset - off - thumb
		}
		r = image.Rect(x0, H-barInset-barW, x0+thumb, H-barInset)
	} else {
		return
	}
	fillRounded(dst, r)
}

// fillRounded fills r in thumb gray with fully rounded short-axis ends.
func fillRounded(dst draw.Image, r image.Rectangle) {
	rad := r.Dx()
	if r.Dy() < rad {
		rad = r.Dy()
	}
	rad /= 2
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if !insideRounded(x, y, r, rad) {
				continue
			}
			switch d := dst.(type) {
			case *image.Gray:
				d.Pix[y*d.Stride+x] = thumbGray
			case *image.RGBA:
				o := y*d.Stride + x*4
				d.Pix[o], d.Pix[o+1], d.Pix[o+2], d.Pix[o+3] = thumbGray, thumbGray, thumbGray, 0xff
			}
		}
	}
}

func insideRounded(x, y int, r image.Rectangle, rad int) bool {
	if rad <= 0 {
		return true
	}
	cx := clamp(x, r.Min.X+rad, r.Max.X-1-rad)
	cy := clamp(y, r.Min.Y+rad, r.Max.Y-1-rad)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= rad*rad
}
