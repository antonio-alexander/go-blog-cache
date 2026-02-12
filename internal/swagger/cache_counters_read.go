package swagger

import "github.com/antonio-alexander/go-blog-cache/internal/data"

// swagger:route GET /cachecounters CacheCounter ReadCacheCounters
// Reads all cache counters.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: CacheCountersGetResponseOk

// swagger:response CacheCountersGetResponseOk
type CacheCountersGetResponseOk struct {
	// in:body
	CacheCounters data.CacheCounters `json:"cache_counters"`
}

// swagger:parameters ReadCacheCounters
type CacheCountersGetParams struct {
	// in:header
	CorrelationId string `json:"Correlation-Id"`
}
