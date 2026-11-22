package pages

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

// PDF is a PDF whose pages each hold one image, the way mangap exports
// chapters. The largest image on a page is taken as the page. Images are
// extracted lazily, under a lock (pdfcpu decodes streams in place).
type PDF struct {
	mu    sync.Mutex
	ctx   *pdfcpu.Context
	pages []int // pdf page numbers that have an image
}

// OpenPDF parses path and finds the pages that carry an image.
func OpenPDF(path string) (*PDF, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	conf := pdfcpu.NewDefaultConfiguration()
	conf.ValidationMode = pdfcpu.ValidationRelaxed
	ctx, err := api.ReadContext(bytes.NewReader(data), conf)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := api.OptimizeContext(ctx); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := ctx.EnsurePageCount(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	p := &PDF{ctx: ctx}
	for i := 1; i <= ctx.PageCount; i++ {
		if len(ctx.ImageObjNrs(i)) > 0 {
			p.pages = append(p.pages, i)
		}
	}
	if len(p.pages) == 0 {
		return nil, fmt.Errorf("%s: %w", path, ErrNoPages)
	}
	return p, nil
}

func (p *PDF) Len() int          { return len(p.pages) }
func (p *PDF) Name(i int) string { return fmt.Sprintf("page %d", p.pages[i]) }
func (p *PDF) Close() error      { return nil }

// Read extracts the biggest image on page i.
func (p *PDF) Read(i int) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	imgs, err := p.ctx.ExtractPageImages(p.pages[i], false)
	if err != nil {
		return nil, err
	}
	var best []byte
	for _, img := range imgs {
		if img.Reader == nil {
			continue
		}
		b, err := io.ReadAll(img.Reader)
		if err != nil {
			return nil, err
		}
		if len(b) > len(best) {
			best = b
		}
	}
	if best == nil {
		return nil, fmt.Errorf("page %d: %w", p.pages[i], ErrNoPages)
	}
	return best, nil
}
