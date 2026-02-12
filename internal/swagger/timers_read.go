package swagger

import "github.com/antonio-alexander/go-blog-cache/internal/data"

// swagger:route GET /timers Timers ReadTimers
// Reads all timers.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: TimersGetResponseOk

// swagger:response TimersGetResponseOk
type TimersGetResponseOk struct {
	// in:body
	Timers data.Timers `json:"timers"`
}

// swagger:parameters ReadTimers
type TimersGetParams struct {
	// in:header
	CorrelationId string `json:"Correlation-Id"`
}
