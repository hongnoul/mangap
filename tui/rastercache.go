package tui

import (
	"container/list"
	"strconv"
	"sync"
)

// rasterCache keeps rendered thumbnail PNGs in memory, keyed by source key
// and pixel size, so a thumbnail that comes back into view (scrolling back,
// returning to a list, moving the preview cursor, after the reader) is
// re-transmitted at once instead of being read, decoded and resized again.
// Bounded by total bytes, least recently used out first.
type rasterCache struct {
	mu    sync.Mutex
	max   int
	size  int
	order *list.List // front = most recent
	items map[string]*list.Element
}

type rasterEntry struct {
	key  string
	png  []byte
	w, h int
}

func newRasterCache(maxBytes int) *rasterCache {
	return &rasterCache{max: maxBytes, order: list.New(), items: map[string]*list.Element{}}
}

func rasterKey(src string, w, h int, fill bool) string {
	k := src + "|" + strconv.Itoa(w) + "x" + strconv.Itoa(h)
	if fill {
		k += "|f"
	}
	return k
}

func (c *rasterCache) get(k string) (png []byte, w, h int, ok bool) {
	if c == nil {
		return nil, 0, 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		return nil, 0, 0, false
	}
	c.order.MoveToFront(el)
	e := el.Value.(*rasterEntry)
	return e.png, e.w, e.h, true
}

func (c *rasterCache) put(k string, png []byte, w, h int) {
	if c == nil || len(png) == 0 || len(png) > c.max {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		e := el.Value.(*rasterEntry)
		c.size += len(png) - len(e.png)
		e.png, e.w, e.h = png, w, h
		c.order.MoveToFront(el)
	} else {
		c.items[k] = c.order.PushFront(&rasterEntry{key: k, png: png, w: w, h: h})
		c.size += len(png)
	}
	for c.size > c.max {
		el := c.order.Back()
		e := el.Value.(*rasterEntry)
		c.order.Remove(el)
		delete(c.items, e.key)
		c.size -= len(e.png)
	}
}
