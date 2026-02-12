package swagger

// swagger:route DELETE /cachecounters CacheCounter DeleteCacheCounters
// Deletes all cache counters.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   204: CacheCountersDeleteResponseNoContent

// swagger:response CacheCountersDeleteResponseNoContent
type CacheCountersDeleteResponseNoContent struct{}

// swagger:parameters DeleteCacheCounters
type CacheCountersDeleteParams struct {
	// in:header
	CorrelationId string `json:"Correlation-Id"`
}
