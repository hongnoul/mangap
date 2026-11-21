// Package kitty emits the kitty graphics protocol in the exact shape gwae's
// graphics_legacy path accepts, which kitty and ghostty accept as well:
// quiet U=1 virtual placements positioned by U+10EEEE placeholder cells.
//
// Contract (from pdfpane's Transport.swift, validated against gwae):
//   - Transfer: a=T,U=1,t=d,f=100,q=2 with nonzero i, s/v pixel size and
//     c/r cell span on the FIRST chunk only. Chunks are at most 4096
//     base64 bytes.
//   - Continuation chunks carry ONLY m and q. Any other key makes gwae
//     reject the chunk, and q=2 means the whole image dies silently.
//   - Every transfer names placement p=1, so a later a=p,U=1,i,p=1,c,r
//     re-spans it in place (a resize can stretch the cached raster to the
//     new pane with zero image bytes).
//   - Placeholder cells: fg = image id via SGR 38;5;id, then U+10EEEE plus
//     the row and column diacritics. Repainting cells of an already
//     transmitted id costs zero image bytes.
//   - Deletes (a=d,d=I) must precede any new transfer in the same write:
//     gwae aborts a pending transfer on delete.
//
// Every emitter appends to a caller-owned buffer so a whole frame goes out
// as one write.
package kitty

import (
	"encoding/base64"
	"strconv"
	"unicode/utf8"
)

// ChunkSize is the largest base64 payload per APC chunk gwae accepts.
const ChunkSize = 4096

// MaxSpan is the largest placement span in cells on either axis: the
// diacritics table has 256 entries.
const MaxSpan = len(Diacritics)

type span struct{ cols, rows int }

type frameKey struct{ id, cols, rows int }

// Transport tracks which image ids live in the terminal and how their
// placement is currently spanned. Not safe for concurrent use: only the
// goroutine that owns the terminal output may call it.
type Transport struct {
	transmitted map[int]bool
	placement   map[int]span
	frames      map[frameKey][]byte
}

// New returns an empty transport.
func New() *Transport {
	return &Transport{
		transmitted: map[int]bool{},
		placement:   map[int]span{},
		frames:      map[frameKey][]byte{},
	}
}

// Transmitted reports whether id already lives in the terminal.
func (t *Transport) Transmitted(id int) bool { return t.transmitted[id] }

// IDs returns every transmitted id (any order).
func (t *Transport) IDs() []int {
	ids := make([]int, 0, len(t.transmitted))
	for id := range t.transmitted {
		ids = append(ids, id)
	}
	return ids
}

// Transmit appends a transfer of png under id with placement 1 spanning
// cols x rows cells, unless id is already transmitted. It reports whether
// bytes were appended.
func (t *Transport) Transmit(out, png []byte, pxW, pxH, id, cols, rows int) ([]byte, bool) {
	if t.transmitted[id] {
		return out, false
	}
	enc := base64.StdEncoding
	n := enc.EncodedLen(len(png))
	b64 := make([]byte, n)
	enc.Encode(b64, png)

	first := true
	for len(b64) > 0 {
		chunk := b64
		if len(chunk) > ChunkSize {
			chunk = chunk[:ChunkSize]
		}
		b64 = b64[len(chunk):]
		m := 0
		if len(b64) > 0 {
			m = 1
		}
		if first {
			out = append(out, "\x1b_Ga=T,U=1,t=d,f=100,q=2,i="...)
			out = strconv.AppendInt(out, int64(id), 10)
			out = append(out, ",s="...)
			out = strconv.AppendInt(out, int64(pxW), 10)
			out = append(out, ",v="...)
			out = strconv.AppendInt(out, int64(pxH), 10)
			out = append(out, ",c="...)
			out = strconv.AppendInt(out, int64(cols), 10)
			out = append(out, ",r="...)
			out = strconv.AppendInt(out, int64(rows), 10)
			out = append(out, ",p=1,m="...)
			first = false
		} else {
			out = append(out, "\x1b_Gq=2,m="...)
		}
		out = strconv.AppendInt(out, int64(m), 10)
		out = append(out, ';')
		out = append(out, chunk...)
		out = append(out, "\x1b\\"...)
	}
	t.transmitted[id] = true
	t.placement[id] = span{cols, rows}
	return out, true
}

// RePlace re-spans placement 1 of a transmitted image to cols x rows: the
// terminal stretches the existing pixels, zero image bytes.
func (t *Transport) RePlace(out []byte, id, cols, rows int) []byte {
	if !t.transmitted[id] {
		return out
	}
	out = append(out, "\x1b_Ga=p,U=1,i="...)
	out = strconv.AppendInt(out, int64(id), 10)
	out = append(out, ",p=1,c="...)
	out = strconv.AppendInt(out, int64(cols), 10)
	out = append(out, ",r="...)
	out = strconv.AppendInt(out, int64(rows), 10)
	out = append(out, ",q=2\x1b\\"...)
	t.placement[id] = span{cols, rows}
	return out
}

// EnsurePlacement re-spans id's placement if it differs from cols x rows,
// so every placeholder cell about to be painted resolves to pixels.
func (t *Transport) EnsurePlacement(out []byte, id, cols, rows int) []byte {
	p, ok := t.placement[id]
	if !ok || (p == span{cols, rows}) {
		return out
	}
	return t.RePlace(out, id, cols, rows)
}

// Delete appends a free of id (placements and stored data).
func (t *Transport) Delete(out []byte, id int) []byte {
	if !t.transmitted[id] {
		return out
	}
	out = append(out, "\x1b_Ga=d,d=I,i="...)
	out = strconv.AppendInt(out, int64(id), 10)
	out = append(out, ",q=2\x1b\\"...)
	delete(t.transmitted, id)
	delete(t.placement, id)
	return out
}

// DeleteAll appends a free of every transmitted id.
func (t *Transport) DeleteAll(out []byte) []byte {
	for _, id := range t.IDs() {
		out = t.Delete(out, id)
	}
	return out
}

// PlaceholderFrame returns the cols x rows grid of placeholder cells for
// id. Every cell is written so the previous frame is fully overwritten
// with no clear, and the last row has no trailing CRLF so a full-pane
// paint never scrolls. The result is memoized and must not be modified.
func (t *Transport) PlaceholderFrame(id, cols, rows int) []byte {
	if cols > MaxSpan {
		cols = MaxSpan
	}
	if rows > MaxSpan {
		rows = MaxSpan
	}
	key := frameKey{id, cols, rows}
	if f, ok := t.frames[key]; ok {
		return f
	}
	if rows <= 0 || cols <= 0 {
		return nil
	}
	// Per cell: placeholder (4 bytes) + two diacritics (up to 3 each).
	out := make([]byte, 0, rows*(cols*10+24))
	var tmp [utf8.UTFMax]byte
	for r := 0; r < rows; r++ {
		out = append(out, "\x1b[38;5;"...)
		out = strconv.AppendInt(out, int64(id), 10)
		out = append(out, 'm')
		n := utf8.EncodeRune(tmp[:], Diacritics[r])
		rowMark := append([]byte(nil), tmp[:n]...)
		for c := 0; c < cols; c++ {
			out = utf8.AppendRune(out, Placeholder)
			out = append(out, rowMark...)
			out = utf8.AppendRune(out, Diacritics[c])
		}
		out = append(out, "\x1b[39m"...)
		if r+1 < rows {
			out = append(out, '\r', '\n')
		}
	}
	// Bound the memo: ids churn under LRU eviction and grids under resize.
	if len(t.frames) >= 32 {
		t.frames = map[frameKey][]byte{}
	}
	t.frames[key] = out
	return out
}
