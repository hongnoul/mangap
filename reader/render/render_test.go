package render

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func white(w, h int) *image.Gray {
	g := image.NewGray(image.Rect(0, 0, w, h))
	for i := range g.Pix {
		g.Pix[i] = 255
	}
	return g
}

func decodePNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// pageBounds returns the bbox of non-black pixels.
func pageBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	var r image.Rectangle
	first := true
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if l := color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y; l > 0 {
				p := image.Rect(x, y, x+1, y+1)
				if first {
					r, first = p, false
				} else {
					r = r.Union(p)
				}
			}
		}
	}
	return r
}

func TestFitCentersAndKeepsAspect(t *testing.T) {
	// A 2:3 portrait page in a 400x300 landscape pane: pillarbox, height
	// fills, width = 200, centered at x=100.
	res, err := Render(Job{Pages: []image.Image{white(200, 300)}, W: 400, H: 300})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Gray {
		t.Fatal("white gray page should encode gray")
	}
	img := decodePNG(t, res.PNG)
	if _, ok := img.(*image.Gray); !ok {
		t.Fatalf("PNG decoded as %T, want 1-channel gray", img)
	}
	if got := pageBounds(img); got != image.Rect(100, 0, 300, 300) {
		t.Fatalf("page rect %v, want (100,0)-(300,300)", got)
	}
}

func TestRotateQuarterTurnSwapsAspect(t *testing.T) {
	res, err := Render(Job{Pages: []image.Image{white(200, 100)}, W: 300, H: 300, Rotation: 1})
	if err != nil {
		t.Fatal(err)
	}
	// 200x100 landscape rotated becomes portrait 1:2 -> 150x300.
	if got := pageBounds(decodePNG(t, res.PNG)); got != image.Rect(75, 0, 225, 300) {
		t.Fatalf("rotated page rect %v, want (75,0)-(225,300)", got)
	}
}

func TestRotationIsClockwise(t *testing.T) {
	p := image.NewGray(image.Rect(0, 0, 4, 2))
	p.Pix[0] = 255 // top-left marker
	res, err := Render(Job{Pages: []image.Image{p}, W: 2, H: 4, Rotation: 1})
	if err != nil {
		t.Fatal(err)
	}
	g := decodePNG(t, res.PNG).(*image.Gray)
	// Clockwise: top-left goes to top-right.
	if g.GrayAt(1, 0).Y != 255 {
		t.Fatalf("marker not at top-right after CW turn: %v", g.Pix)
	}
}

func TestSpreadPlacesPagesSideBySide(t *testing.T) {
	left := white(100, 150)
	right := image.NewGray(image.Rect(0, 0, 100, 150)) // black page
	res, err := Render(Job{Pages: []image.Image{left, right}, W: 200, H: 150})
	if err != nil {
		t.Fatal(err)
	}
	g := decodePNG(t, res.PNG).(*image.Gray)
	if g.GrayAt(50, 75).Y != 255 || g.GrayAt(150, 75).Y != 0 {
		t.Fatalf("spread order wrong: left=%d right=%d", g.GrayAt(50, 75).Y, g.GrayAt(150, 75).Y)
	}
}

func TestNearGrayJPEGCountsAsGray(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := 0; i < len(src.Pix); i += 4 {
		v := uint8(i / 4 % 256)
		src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = v, v, v, 255
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	img, err := Decode(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !isNeutral(img) {
		t.Fatal("gray content saved as color JPEG should count as neutral")
	}
}

func TestResampleFlatStaysFlat(t *testing.T) {
	// Weights must sum to exactly 1: white stays 255 at every scale.
	for _, sz := range [][2]int{{1000, 333}, {333, 1000}, {7, 3}, {3, 7}} {
		res, err := Render(Job{Pages: []image.Image{white(sz[0], sz[0])}, W: sz[1], H: sz[1]})
		if err != nil {
			t.Fatal(err)
		}
		g := decodePNG(t, res.PNG).(*image.Gray)
		for i, v := range g.Pix {
			if v != 255 {
				t.Fatalf("%dx -> %d: pixel %d = %d", sz[0], sz[1], i, v)
			}
		}
	}
}

func TestNativeDensityShrinksUpscaledRaster(t *testing.T) {
	// 100x150 page in a 400x300 pane would be enlarged 2x. Native density
	// rasters at 1x (the 0.5 floor): 200x150, same aspect as the pane.
	res, err := Render(Job{Pages: []image.Image{white(100, 150)}, W: 400, H: 300, NativeDensity: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.W != 200 || res.H != 150 {
		t.Fatalf("native raster %dx%d, want 200x150", res.W, res.H)
	}
	if got := pageBounds(decodePNG(t, res.PNG)); got != image.Rect(50, 0, 150, 150) {
		t.Fatalf("page rect %v, want (50,0)-(150,150)", got)
	}
	// A page already denser than the pane is untouched.
	res, err = Render(Job{Pages: []image.Image{white(1000, 1500)}, W: 400, H: 300, NativeDensity: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.W != 400 || res.H != 300 {
		t.Fatalf("dense page raster %dx%d, want full 400x300", res.W, res.H)
	}
}

func TestEncodePNGRoundTrips(t *testing.T) {
	g := image.NewGray(image.Rect(0, 0, 37, 23))
	for i := range g.Pix {
		g.Pix[i] = uint8(i * 7)
	}
	c := image.NewRGBA(image.Rect(0, 0, 19, 11))
	for i := range c.Pix {
		c.Pix[i] = uint8(i * 13)
		if i%4 == 3 {
			c.Pix[i] = 255
		}
	}
	for _, img := range []image.Image{g, c} {
		var buf bytes.Buffer
		if err := encodePNG(&buf, img, false); err != nil {
			t.Fatal(err)
		}
		back := decodePNG(t, buf.Bytes())
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r1, g1, b1, a1 := img.At(x, y).RGBA()
				r2, g2, b2, a2 := back.At(x, y).RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					t.Fatalf("%T pixel (%d,%d) differs after round trip", img, x, y)
				}
			}
		}
	}
}

// BenchmarkFixture renders a real manga page at a retina half-pane:
// MANGAP_BENCH_PAGE=/path/to/page.png go test -bench Fixture ./reader/render
func BenchmarkFixture(b *testing.B) {
	path := os.Getenv("MANGAP_BENCH_PAGE")
	if path == "" {
		b.Skip("set MANGAP_BENCH_PAGE")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		img, err := Decode(data)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := Render(Job{Pages: []image.Image{img}, W: 3200, H: 1800, Total: 50,
			NativeDensity: os.Getenv("MANGAP_BENCH_NATIVE") != "0"}); err != nil {
			b.Fatal(err)
		}
	}
}
