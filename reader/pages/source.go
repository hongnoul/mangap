// Package pages enumerates the page images of one chapter: a directory of
// image files or a CBZ/ZIP archive. Reads are safe for concurrent use, so
// render lanes can load pages in parallel.
package pages

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source is an ordered list of page images.
type Source interface {
	// Len is the number of pages.
	Len() int
	// Name is the page's file name (for errors and tracing).
	Name(i int) string
	// Read returns the encoded bytes of page i (0-based).
	Read(i int) ([]byte, error)
	// Close releases the underlying file, if any.
	Close() error
}

// ErrNoPages is returned when the path holds no supported images.
var ErrNoPages = errors.New("no page images found")

var imageExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

// IsImage reports whether name has a supported page image extension.
func IsImage(name string) bool {
	return imageExt[strings.ToLower(filepath.Ext(name))]
}

// Open opens a directory, or a .cbz/.zip archive.
func Open(path string) (Source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return OpenDir(path)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cbz", ".zip":
		return OpenZip(path)
	}
	return nil, fmt.Errorf("%s: not a directory or .cbz/.zip archive", path)
}

// Dir is a directory of page images in natural name order.
type Dir struct {
	root  string
	files []string
}

// OpenDir lists the images directly inside root.
func OpenDir(root string) (*Dir, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() && IsImage(e.Name()) {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s: %w", root, ErrNoPages)
	}
	sort.Slice(files, func(a, b int) bool { return NaturalLess(files[a], files[b]) })
	return &Dir{root: root, files: files}, nil
}

func (d *Dir) Len() int          { return len(d.files) }
func (d *Dir) Name(i int) string { return d.files[i] }
func (d *Dir) Close() error      { return nil }
func (d *Dir) Read(i int) ([]byte, error) {
	return os.ReadFile(filepath.Join(d.root, d.files[i]))
}

// Zip is a CBZ/ZIP archive's page images in natural name order. Nested
// folders are flattened by full path, which keeps volume/chapter
// subfolders in order.
type Zip struct {
	rc    *zip.ReadCloser
	files []*zip.File
}

// OpenZip opens an archive and lists its images.
func OpenZip(path string) (*Zip, error) {
	rc, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	var files []*zip.File
	for _, f := range rc.File {
		if f.FileInfo().IsDir() || !IsImage(f.Name) {
			continue
		}
		// macOS archivers add resource-fork shadows of every file.
		if strings.HasPrefix(f.Name, "__MACOSX/") || strings.HasPrefix(filepath.Base(f.Name), "._") {
			continue
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		_ = rc.Close()
		return nil, fmt.Errorf("%s: %w", path, ErrNoPages)
	}
	sort.Slice(files, func(a, b int) bool { return NaturalLess(files[a].Name, files[b].Name) })
	return &Zip{rc: rc, files: files}, nil
}

func (z *Zip) Len() int          { return len(z.files) }
func (z *Zip) Name(i int) string { return z.files[i].Name }
func (z *Zip) Close() error      { return z.rc.Close() }

// Read decompresses page i. zip.File.Open reads through an
// io.SectionReader over the archive's ReaderAt, so concurrent calls are
// safe.
func (z *Zip) Read(i int) ([]byte, error) {
	r, err := z.files[i].Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// NaturalLess orders names so embedded numbers compare by value:
// "2.jpg" < "10.jpg". Ties on value fall back to digit-run length
// ("01" vs "1") and then to plain byte order, so the order is total.
func NaturalLess(a, b string) bool {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		ca, cb := a[ia], b[ib]
		if isDigit(ca) && isDigit(cb) {
			ja, jb := ia, ib
			for ja < len(a) && isDigit(a[ja]) {
				ja++
			}
			for jb < len(b) && isDigit(b[jb]) {
				jb++
			}
			na := strings.TrimLeft(a[ia:ja], "0")
			nb := strings.TrimLeft(b[ib:jb], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			if ja-ia != jb-ib {
				return ja-ia < jb-ib
			}
			ia, ib = ja, jb
			continue
		}
		la, lb := lower(ca), lower(cb)
		if la != lb {
			return la < lb
		}
		ia++
		ib++
	}
	if len(a)-ia != len(b)-ib {
		return len(a)-ia < len(b)-ib
	}
	return a < b
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}
