package data

type CacheCounters struct {
	CounterHits   map[string]int `json:"counter_hits,omitempty"`
	CounterMisses map[string]int `json:"counter_misses,omitempty"`
}
