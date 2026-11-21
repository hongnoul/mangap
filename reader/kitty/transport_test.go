package kitty

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf8"
)

// apcs splits out every ESC _ G ... ESC \ command as (control, payload).
func apcs(b []byte) [][2]string {
	var out [][2]string
	for {
		i := bytes.Index(b, []byte("\x1b_G"))
		if i < 0 {
			return out
		}
		b = b[i+3:]
		j := bytes.Index(b, []byte("\x1b\\"))
		if j < 0 {
			return out
		}
		cmd := string(b[:j])
		b = b[j+2:]
		ctl, payload := cmd, ""
		if k := strings.IndexByte(cmd, ';'); k >= 0 {
			ctl, payload = cmd[:k], cmd[k+1:]
		}
		out = append(out, [2]string{ctl, payload})
	}
}

func keys(ctl string) map[string]string {
	m := map[string]string{}
	for _, kv := range strings.Split(ctl, ",") {
		if p := strings.SplitN(kv, "=", 2); len(p) == 2 {
			m[p[0]] = p[1]
		}
	}
	return m
}

func TestTransmitMatchesGwaeContract(t *testing.T) {
	png := bytes.Repeat([]byte{0xAB, 0xCD, 0xEF}, 7000) // > 2 chunks of base64
	tr := New()
	out, sent := tr.Transmit(nil, png, 640, 480, 12, 80, 24)
	if !sent {
		t.Fatal("first transmit sent nothing")
	}
	cmds := apcs(out)
	if len(cmds) < 3 {
		t.Fatalf("want >= 3 chunks, got %d", len(cmds))
	}
	first := keys(cmds[0][0])
	for k, v := range map[string]string{"a": "T", "U": "1", "t": "d", "f": "100", "q": "2",
		"i": "12", "s": "640", "v": "480", "c": "80", "r": "24", "p": "1", "m": "1"} {
		if first[k] != v {
			t.Fatalf("first chunk %s=%q want %q (ctl %q)", k, first[k], v, cmds[0][0])
		}
	}
	var payload strings.Builder
	for i, c := range cmds {
		if len(c[1]) > ChunkSize {
			t.Fatalf("chunk %d is %d bytes > %d", i, len(c[1]), ChunkSize)
		}
		payload.WriteString(c[1])
		if i == 0 {
			continue
		}
		k := keys(c[0])
		wantM := "1"
		if i == len(cmds)-1 {
			wantM = "0"
		}
		if len(k) != 2 || k["q"] != "2" || k["m"] != wantM {
			t.Fatalf("continuation %d ctl %q: must carry only q=2,m=%s", i, c[0], wantM)
		}
	}
	got, err := base64.StdEncoding.DecodeString(payload.String())
	if err != nil || !bytes.Equal(got, png) {
		t.Fatal("reassembled payload differs from the PNG")
	}

	// Re-transmitting a live id is free.
	if out2, sent := tr.Transmit(nil, png, 640, 480, 12, 80, 24); sent || len(out2) != 0 {
		t.Fatal("re-transmit of a live id wrote bytes")
	}
}

func TestPlaceholderFrame(t *testing.T) {
	tr := New()
	f := tr.PlaceholderFrame(42, 3, 2)
	s := string(f)
	if strings.Count(s, "\x1b[38;5;42m") != 2 {
		t.Fatalf("want fg=42 per row: %q", s)
	}
	if strings.HasSuffix(s, "\r\n") || strings.Count(s, "\r\n") != 1 {
		t.Fatal("rows must be CRLF separated with no trailing CRLF (no scroll)")
	}
	// Row 1, col 2 = placeholder + Diacritics[1] + Diacritics[2].
	cell := string([]rune{Placeholder, Diacritics[1], Diacritics[2]})
	if !strings.Contains(s, cell) {
		t.Fatal("missing row/col diacritics for cell (1,2)")
	}
	if n := strings.Count(s, string(Placeholder)); n != 6 {
		t.Fatalf("want 6 cells, got %d", n)
	}
	if !utf8.Valid(f) {
		t.Fatal("frame is not valid UTF-8")
	}
	if &tr.PlaceholderFrame(42, 3, 2)[0] != &f[0] {
		t.Fatal("frame not memoized")
	}
}

func TestDeleteAndReplace(t *testing.T) {
	tr := New()
	out, _ := tr.Transmit(nil, []byte("x"), 1, 1, 10, 4, 4)
	out = tr.EnsurePlacement(out[:0], 10, 4, 4)
	if len(out) != 0 {
		t.Fatal("EnsurePlacement wrote bytes for an unchanged span")
	}
	out = tr.EnsurePlacement(nil, 10, 8, 6)
	if !strings.Contains(string(out), "a=p,U=1,i=10,p=1,c=8,r=6") {
		t.Fatalf("re-place = %q", out)
	}
	out = tr.Delete(nil, 10)
	if string(out) != "\x1b_Ga=d,d=I,i=10,q=2\x1b\\" || tr.Transmitted(10) {
		t.Fatalf("delete = %q", out)
	}
	if len(tr.Delete(nil, 10)) != 0 {
		t.Fatal("deleting a dead id wrote bytes")
	}
}
