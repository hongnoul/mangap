package render

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"sync"
)

// encodePNG writes an 8-bit gray or RGB PNG using the Up filter on every
// row. image/png picks the best of five filters per row by trial, which
// profiled at ~55% of a frame; Up alone is what pdfpane ships (ImageIO's
// PNG_FILTER_UP) and on manga it costs a few percent of size for a
// several-fold faster encode. Output is a valid PNG any decoder reads.
//
// opaque asserts an *image.RGBA has alpha 255 everywhere (Render's frames
// do by construction), which skips a full-frame scan.
func encodePNG(buf *bytes.Buffer, img image.Image, opaque bool) error {
	var (
		pix    []uint8
		stride int
		w, h   int
		ch     int
		ctype  byte
	)
	switch m := img.(type) {
	case *image.Gray:
		pix, stride, ch, ctype = m.Pix, m.Stride, 1, 0
		w, h = m.Rect.Dx(), m.Rect.Dy()
	case *image.RGBA:
		// image.RGBA is premultiplied and PNG is straight alpha: raw bytes
		// are only the same pixels when opaque. Render always produces
		// opaque frames; anything else takes the exact std path.
		if !opaque && !m.Opaque() {
			return encoder.Encode(buf, img)
		}
		// Opaque: drop alpha and ship RGB (color type 2). Measured on manga
		// color pages: ~12% smaller and ~35% faster to deflate than RGBA,
		// since the constant alpha byte only costs.
		pix, stride, ch, ctype = m.Pix, m.Stride, 4, 2
		w, h = m.Rect.Dx(), m.Rect.Dy()
	default:
		return encoder.Encode(buf, img)
	}

	buf.Write([]byte("\x89PNG\r\n\x1a\n"))
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8] = 8 // bit depth
	ihdr[9] = ctype
	writeChunk(buf, "IHDR", ihdr[:])

	idat := idatPool.Get().(*bytes.Buffer)
	idat.Reset()
	defer idatPool.Put(idat)
	zw, err := zlib.NewWriterLevel(idat, zlib.BestSpeed)
	if err != nil {
		return err
	}
	// out is the sample width per pixel on the wire: RGBA input is
	// packed to RGB.
	out := ch
	if ctype == 2 {
		out = 3
	}
	rowLen := w * out
	line := make([]byte, 1+rowLen)
	line[0] = 2 // Up
	cur := make([]byte, rowLen)
	prev := make([]byte, rowLen)
	for y := 0; y < h; y++ {
		src := pix[y*stride : y*stride+w*ch]
		if ch == out {
			copy(cur, src)
		} else {
			for x, o := 0, 0; x < len(src); x, o = x+4, o+3 {
				cur[o], cur[o+1], cur[o+2] = src[x], src[x+1], src[x+2]
			}
		}
		dst := line[1:]
		if y == 0 {
			copy(dst, cur)
		} else {
			for i := range dst {
				dst[i] = cur[i] - prev[i]
			}
		}
		if _, err := zw.Write(line); err != nil {
			return err
		}
		prev, cur = cur, prev
	}
	if err := zw.Close(); err != nil {
		return err
	}
	writeChunk(buf, "IDAT", idat.Bytes())
	writeChunk(buf, "IEND", nil)
	return nil
}

var idatPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func writeChunk(buf *bytes.Buffer, typ string, data []byte) {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(data)))
	buf.Write(n[:])
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(data)
	buf.WriteString(typ)
	buf.Write(data)
	binary.BigEndian.PutUint32(n[:], crc.Sum32())
	buf.Write(n[:])
}
