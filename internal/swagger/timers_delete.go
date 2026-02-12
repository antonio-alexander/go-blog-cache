package swagger

// swagger:route DELETE /timers Timers DeleteTimers
// Deletes all timers.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: TimersDeleteResponseOk

// swagger:response TimersDeleteResponseOk
type TimersDeleteResponseOk struct{}

// swagger:parameters DeleteTimers
type TimersDeleteParams struct {
	// in:header
	CorrelationId string `json:"Correlation-Id"`
}
