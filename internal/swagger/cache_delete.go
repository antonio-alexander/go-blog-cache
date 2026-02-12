package swagger

// swagger:route DELETE /cache Cache DeleteCache
// Deletes all items in the cache.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   204: CacheDeleteResponseNoContent

// swagger:response CacheDeleteResponseNoContent
type CacheDeleteResponseNoContent struct{}

// swagger:parameters DeleteCache
type CacheDeleteParams struct {
	// in:header
	CorrelationId string `json:"Correlation-Id"`
}
