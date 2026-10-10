package api

import (
	"container/list"

	"megabrain/internal/chunks"
)

// chunkCache é um cache LRU de chunks: base (z:level:cx:cy) -> caveID -> chunk.
// Cada base agrupa as variantes por cave, para que invalidar um chunk remova
// todas de uma vez.
type chunkCache struct {
	max   int
	ll    *list.List
	items map[string]*list.Element
}

type chunkEntry struct {
	base   string
	chunks map[int]*chunks.Chunk
}

func newChunkCache(max int) *chunkCache {
	return &chunkCache{max: max, ll: list.New(), items: make(map[string]*list.Element)}
}

func (c *chunkCache) get(base string, caveID int) (*chunks.Chunk, bool) {
	el, ok := c.items[base]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	ch, ok := el.Value.(*chunkEntry).chunks[caveID]
	return ch, ok
}

func (c *chunkCache) put(base string, caveID int, chunk *chunks.Chunk) {
	if el, ok := c.items[base]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*chunkEntry).chunks[caveID] = chunk
		return
	}
	el := c.ll.PushFront(&chunkEntry{base: base, chunks: map[int]*chunks.Chunk{caveID: chunk}})
	c.items[base] = el
	if c.ll.Len() > c.max {
		c.evict()
	}
}

func (c *chunkCache) remove(base string) {
	if el, ok := c.items[base]; ok {
		c.ll.Remove(el)
		delete(c.items, base)
	}
}

func (c *chunkCache) evict() {
	el := c.ll.Back()
	if el == nil {
		return
	}
	c.ll.Remove(el)
	delete(c.items, el.Value.(*chunkEntry).base)
}

func (c *chunkCache) clear() {
	c.ll.Init()
	c.items = make(map[string]*list.Element)
}
