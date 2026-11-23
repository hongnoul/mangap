package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/hongnoul/mangap/reader/pages"
)

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanAndRead(t *testing.T) {
	img, err := os.ReadFile("../assets/testdata/hey.jpeg")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// cbz manga with cover, chapters out of lexical order
	for _, n := range []string{"[0010] Chapter 10.cbz", "[0002] Chapter 2.cbz"} {
		f, _ := os.Create(filepath.Join(mk(t, dir, "Alpha"), n))
		zw := zip.NewWriter(f)
		w, _ := zw.Create("001.jpeg")
		_, _ = w.Write(img)
		_ = zw.Close()
		_ = f.Close()
	}
	write(t, filepath.Join(dir, "Alpha", "cover.jpg"), img)
	// plain-format manga: chapter dirs of images
	write(t, filepath.Join(dir, "Beta", "[0001] One", "1.jpeg"), img)
	// unrelated folders are ignored
	write(t, filepath.Join(dir, "notes", "a.txt"), []byte("x"))
	write(t, filepath.Join(dir, "loose", "1.jpeg"), img)

	s := New(dir)
	ms := s.Mangas()
	if len(ms) != 2 {
		t.Fatalf("got %d manga, want 2", len(ms))
	}
	var alpha = ms[0]
	if alpha.Name != "Alpha" {
		alpha = ms[1]
	}
	if got := alpha.Metadata.Cover.ExtraLarge; got != FileURL(filepath.Join(dir, "Alpha", "cover.jpg")) {
		t.Fatalf("cover %q", got)
	}
	chs, _ := s.ChaptersOf(alpha)
	if len(chs) != 2 || chs[0].Name != "Chapter 2" || chs[1].Name != "Chapter 10" {
		t.Fatalf("chapters %v", chs)
	}
	src, err := pages.Open(s.PathOf(chs[1]))
	if err != nil || src.Len() != 1 {
		t.Fatalf("open: %v", err)
	}
	_ = src.Close() // Windows cannot remove the temp dir while it is open
	if res, _ := s.Search("bet"); len(res) != 1 || res[0].Name != "Beta" {
		t.Fatalf("search %v", res)
	}
}

func mk(t *testing.T, dir, name string) string {
	p := filepath.Join(dir, name)
	_ = os.MkdirAll(p, 0o755)
	return p
}
