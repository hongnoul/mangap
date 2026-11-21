package pages

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNaturalLess(t *testing.T) {
	in := []string{"10.jpg", "2.jpg", "1.jpg", "page 02.png", "page 1.png", "01.jpg", "B.png", "a.png"}
	sort.Slice(in, func(a, b int) bool { return NaturalLess(in[a], in[b]) })
	want := []string{"1.jpg", "01.jpg", "2.jpg", "10.jpg", "a.png", "B.png", "page 1.png", "page 02.png"}
	for i := range want {
		if in[i] != want[i] {
			t.Fatalf("order = %v, want %v", in, want)
		}
	}
}

func TestDirAndZip(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"10.jpg", "2.png", "1.webp", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if src.Len() != 3 || src.Name(0) != "1.webp" || src.Name(2) != "10.jpg" {
		t.Fatalf("dir pages = %d [%s .. %s]", src.Len(), src.Name(0), src.Name(src.Len()-1))
	}
	b, err := src.Read(1)
	if err != nil || string(b) != "2.png" {
		t.Fatalf("Read(1) = %q, %v", b, err)
	}

	cbz := filepath.Join(t.TempDir(), "ch.cbz")
	f, err := os.Create(cbz)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, n := range []string{"ch/3.png", "ch/1.png", "__MACOSX/ch/._1.png", "ComicInfo.xml"} {
		w, _ := zw.Create(n)
		_, _ = w.Write([]byte(n))
	}
	_ = zw.Close()
	_ = f.Close()

	z, err := Open(cbz)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if z.Len() != 2 || z.Name(0) != "ch/1.png" {
		t.Fatalf("zip pages = %d first %s", z.Len(), z.Name(0))
	}
	b, err = z.Read(1)
	if err != nil || string(b) != "ch/3.png" {
		t.Fatalf("zip Read(1) = %q, %v", b, err)
	}
}

func TestEmpty(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("empty dir should fail")
	}
}
