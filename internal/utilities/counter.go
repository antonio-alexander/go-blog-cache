package utilities

import "sync"

type counter struct {
	hit  int
	miss int
}

type cacheCounter struct {
	sync.RWMutex
	counters map[any]*counter
}

type CacheCounter interface {
	Read(key any) (hitCount, missCount int)
	ReadAll() (counterHit, counterMiss map[any]int)
	IncrementHit(key any) (hitCount int)
	IncrementMiss(key any) (missCount int)
	Reset(key any)
}

func NewCacheCounter(parameters ...any) CacheCounter {
	return &cacheCounter{
		counters: make(map[any]*counter),
	}
}

func (c *cacheCounter) Clear() {
	c.Lock()
	defer c.Unlock()

	c.counters = nil
	c.counters = make(map[any]*counter)
}

func (c *cacheCounter) Read(key any) (int, int) {
	c.RLock()
	defer c.RUnlock()

	if counter, found := c.counters[key]; found {
		return counter.hit, counter.miss
	}
	return -1, -1
}

func (c *cacheCounter) ReadAll() (map[any]int, map[any]int) {
	c.RLock()
	defer c.RUnlock()

	counterHit := make(map[any]int)
	counterMiss := make(map[any]int)
	for key, value := range c.counters {
		counterHit[key] = value.hit
		counterMiss[key] = value.miss
	}
	return counterHit, counterMiss
}

func (c *cacheCounter) Reset(key any) {
	c.Lock()
	defer c.Unlock()

	delete(c.counters, key)
}

func (c *cacheCounter) IncrementHit(key any) int {
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

func (c *cacheCounter) IncrementMiss(key any) int {
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
