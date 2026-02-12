package utilities

import (
	"sync"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

type counter struct {
	hit  int
	miss int
}

type cacheCounter struct {
	sync.RWMutex
	counters map[string]*counter
}

type Counter interface {
	Read(key string) (hitCount, missCount int)
	ReadAll() *data.CacheCounters
	IncrementHit(key string) (hitCount int)
	IncrementMiss(key string) (missCount int)
	Reset()
}

func NewCounter(parameters ...any) Counter {
	return &cacheCounter{
		counters: make(map[string]*counter),
	}
}

func (c *cacheCounter) Clear() {
	c.Lock()
	defer c.Unlock()

	c.counters = nil
	c.counters = make(map[string]*counter)
}

func (c *cacheCounter) Read(key string) (int, int) {
	c.RLock()
	defer c.RUnlock()

	if counter, found := c.counters[key]; found {
		return counter.hit, counter.miss
	}
	return -1, -1
}

func (c *cacheCounter) ReadAll() *data.CacheCounters {
	c.RLock()
	defer c.RUnlock()

	counterHit := make(map[string]int)
	counterMiss := make(map[string]int)
	for key, value := range c.counters {
		counterHit[key] = value.hit
		counterMiss[key] = value.miss
	}
	return &data.CacheCounters{
		CounterHits:   counterHit,
		CounterMisses: counterMiss,
	}
}

func (c *cacheCounter) Reset() {
	c.Lock()
	defer c.Unlock()

	c.counters = nil
	c.counters = make(map[string]*counter)
}

func (c *cacheCounter) IncrementHit(key string) int {
	c.Lock()
	defer c.Unlock()

	cntr, found := c.counters[key]
	if !found {
		cntr = &counter{}
		c.counters[key] = cntr
	}
	cntr.hit++
	return cntr.hit
}

func (c *cacheCounter) IncrementMiss(key string) int {
	c.Lock()
	defer c.Unlock()

	cntr, found := c.counters[key]
	if !found {
		cntr = &counter{}
		c.counters[key] = cntr
	}
	cntr.miss++
	return cntr.miss
}
