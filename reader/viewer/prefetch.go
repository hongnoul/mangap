package viewer

import (
	"image"
	"runtime"
	"sync"
	"time"

	"github.com/hongnoul/mangaz/reader/pages"
	"github.com/hongnoul/mangaz/reader/render"
)

// Prefetcher renders frames on worker goroutines so the terminal-owning
// goroutine never blocks on decode/scale/encode. Ported from pdfpane's
// Prefetch.swift:
//   - Declarative want/demand sets bound memory to a few frames, and a job
//     re-checks wantedness when it starts, so a fast page walk abandons
//     renders for pages already left behind.
//   - Urgent jobs (the frame the reader is waiting on) go to a dedicated
//     queue drained before background prefetch.
//   - A finished wanted render signals Ready, so a held-key scroll can show
//     the nearest landed page instead of freezing.
//   - Never writes to the terminal.
type Prefetcher struct {
	src pages.Source

	mu       sync.Mutex
	results  map[string]*render.Result
	inFlight map[string]bool
	wanted   map[string]bool
	awaited  map[string]bool
	urgent   []Target
	bg       []Target
	slow     bool
	closed   bool
	cond     *sync.Cond

	// decoded caches decoded pages: a spread and its neighbors share
	// pages, and rotation changes re-render the same source.
	decoded *decodeCache

	// Ready receives (non-blocking, coalesced) whenever a wanted render
	// lands.
	Ready chan struct{}

	// NativeDensity is passed to every render job.
	NativeDensity bool

	// Trace, if set, receives one line per render (never the terminal).
	Trace func(format string, args ...any)
}

// NewPrefetcher starts workers lanes (default: CPU count, at least 3).
func NewPrefetcher(src pages.Source, workers int) *Prefetcher {
	return NewPrefetcherReady(src, workers, make(chan struct{}, 1))
}

// NewPrefetcherReady is NewPrefetcher signaling on a caller-owned Ready
// channel, so one loop can wait on several prefetchers (the reader keeps
// neighbor chapters warm).
func NewPrefetcherReady(src pages.Source, workers int, ready chan struct{}) *Prefetcher {
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > 6 {
			workers = 6
		}
	}
	if workers < 3 {
		workers = 3
	}
	p := &Prefetcher{
		src:           src,
		results:       map[string]*render.Result{},
		inFlight:      map[string]bool{},
		wanted:        map[string]bool{},
		awaited:       map[string]bool{},
		decoded:       newDecodeCache(16),
		Ready:         ready,
		NativeDensity: true,
	}
	p.cond = sync.NewCond(&p.mu)
	for i := 0; i < workers; i++ {
		go p.worker()
	}
	return p
}

// Close stops the workers.
func (p *Prefetcher) Close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.cond.Broadcast()
}

// SawSlowRender latches once any render took >150ms (pdfpane's threshold
// for engaging the held-scroll clamp).
func (p *Prefetcher) SawSlowRender() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.slow
}

// Take returns a finished render for t, if any.
func (p *Prefetcher) Take(t Target) *render.Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.results[t.Key()]
}

// Want declares the frames worth having ready: results outside the set
// are dropped, missing ones are queued for background lanes.
func (p *Prefetcher) Want(ts []Target) {
	p.mu.Lock()
	p.wanted = keySet(ts)
	for k := range p.awaited {
		p.wanted[k] = true
	}
	p.prune()
	for _, t := range ts {
		if p.admit(t.Key()) {
			p.bg = append(p.bg, t)
		}
	}
	p.mu.Unlock()
	p.cond.Broadcast()
}

// Demand marks urgent frames the reader is waiting on, plus the
// background set to keep warming.
func (p *Prefetcher) Demand(urgent []Target, also []Target) {
	p.mu.Lock()
	p.awaited = keySet(urgent)
	p.wanted = keySet(also)
	for k := range p.awaited {
		p.wanted[k] = true
	}
	p.prune()
	ready := false
	for _, t := range urgent {
		if p.results[t.Key()] != nil {
			ready = true
		}
		if p.admit(t.Key()) {
			p.urgent = append(p.urgent, t)
		}
	}
	for _, t := range also {
		if p.admit(t.Key()) {
			p.bg = append(p.bg, t)
		}
	}
	p.mu.Unlock()
	p.cond.Broadcast()
	if ready {
		p.signal()
	}
}

// ClearAwait stops waking for the awaited frames (shown or superseded).
func (p *Prefetcher) ClearAwait() {
	p.mu.Lock()
	p.awaited = map[string]bool{}
	p.mu.Unlock()
}

func keySet(ts []Target) map[string]bool {
	m := make(map[string]bool, len(ts))
	for _, t := range ts {
		m[t.Key()] = true
	}
	return m
}

// prune drops results and queued jobs nobody wants. Caller holds mu.
func (p *Prefetcher) prune() {
	for k := range p.results {
		if !p.wanted[k] {
			delete(p.results, k)
		}
	}
	keep := func(q []Target) []Target {
		out := q[:0]
		for _, t := range q {
			if k := t.Key(); p.wanted[k] {
				out = append(out, t)
			} else {
				delete(p.inFlight, k)
			}
		}
		return out
	}
	p.urgent = keep(p.urgent)
	p.bg = keep(p.bg)
}

// admit reports whether key needs a new job. Caller holds mu.
func (p *Prefetcher) admit(k string) bool {
	if p.results[k] != nil || p.inFlight[k] {
		return false
	}
	p.inFlight[k] = true
	return true
}

func (p *Prefetcher) signal() {
	select {
	case p.Ready <- struct{}{}:
	default:
	}
}

func (p *Prefetcher) next() (Target, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		if p.closed {
			return Target{}, false
		}
		if len(p.urgent) > 0 {
			t := p.urgent[0]
			p.urgent = p.urgent[1:]
			return t, true
		}
		if len(p.bg) > 0 {
			t := p.bg[0]
			p.bg = p.bg[1:]
			return t, true
		}
		p.cond.Wait()
	}
}

func (p *Prefetcher) worker() {
	for {
		t, ok := p.next()
		if !ok {
			return
		}
		k := t.Key()
		p.mu.Lock()
		still := p.wanted[k] || p.awaited[k]
		p.mu.Unlock()

		var res *render.Result
		var err error
		if still {
			res, err = p.render(t)
			if p.Trace != nil {
				if err != nil {
					p.Trace("render %s failed: %v", k, err)
				} else {
					p.Trace("render %s %dx%d gray=%v scale=%.1fms encode=%.1fms total=%.1fms png=%dB",
						k, res.W, res.H, res.Gray, res.ScaleMs, res.EncodeMs, res.TotalMs, len(res.PNG))
				}
			}
		}

		p.mu.Lock()
		delete(p.inFlight, k)
		fire := false
		if res != nil {
			if res.TotalMs > 150 {
				p.slow = true
			}
			if p.wanted[k] || p.awaited[k] {
				p.results[k] = res
				fire = true
			}
		}
		p.mu.Unlock()
		if fire {
			p.signal()
		}
	}
}

func (p *Prefetcher) render(t Target) (*render.Result, error) {
	st := State{Pages: p.src.Len(), Spread: t.Spread, RTL: t.RTL}
	idx := st.SheetPages(t.Page)
	imgs := make([]image.Image, 0, len(idx))
	t0 := time.Now()
	for _, i := range idx {
		img, err := p.decoded.get(i, func() (image.Image, error) {
			b, err := p.src.Read(i)
			if err != nil {
				return nil, err
			}
			return render.Decode(b)
		})
		if err != nil {
			return nil, err
		}
		imgs = append(imgs, img)
	}
	decodeMs := float64(time.Since(t0).Microseconds()) / 1000
	res, err := render.Render(render.Job{
		Pages: imgs, W: t.Grid.PxW, H: t.Grid.PxH,
		Rotation: t.Rotation,
		Index:    t.Page, Total: p.src.Len(), RTL: t.RTL,
		NativeDensity: p.NativeDensity,
	})
	if res != nil {
		res.TotalMs += decodeMs
	}
	return res, err
}

// decodeCache is a small LRU of decoded pages with single-flight loads.
type decodeCache struct {
	mu    sync.Mutex
	cap   int
	order []int
	imgs  map[int]image.Image
	wait  map[int]*sync.WaitGroup
	errs  map[int]error
}

func newDecodeCache(n int) *decodeCache {
	return &decodeCache{cap: n, imgs: map[int]image.Image{}, wait: map[int]*sync.WaitGroup{}, errs: map[int]error{}}
}

func (c *decodeCache) get(i int, load func() (image.Image, error)) (image.Image, error) {
	c.mu.Lock()
	if img, ok := c.imgs[i]; ok {
		c.bump(i)
		c.mu.Unlock()
		return img, nil
	}
	if wg, ok := c.wait[i]; ok {
		c.mu.Unlock()
		wg.Wait()
		c.mu.Lock()
		img, err := c.imgs[i], c.errs[i]
		c.mu.Unlock()
		if img == nil && err == nil {
			return load() // evicted between load and read: just reload
		}
		return img, err
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	c.wait[i] = wg
	c.mu.Unlock()

	img, err := load()

	c.mu.Lock()
	delete(c.wait, i)
	if err != nil {
		c.errs[i] = err
	} else {
		delete(c.errs, i)
		c.imgs[i] = img
		c.bump(i)
		for len(c.order) > c.cap {
			delete(c.imgs, c.order[0])
			c.order = c.order[1:]
		}
	}
	c.mu.Unlock()
	wg.Done()
	return img, err
}

func (c *decodeCache) bump(i int) {
	for j, v := range c.order {
		if v == i {
			c.order = append(c.order[:j], c.order[j+1:]...)
			break
		}
	}
	c.order = append(c.order, i)
}
