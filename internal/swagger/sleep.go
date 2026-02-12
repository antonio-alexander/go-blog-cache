package swagger

import (
	"time"

	"github.com/antonio-alexander/go-blog-cache/internal/data"
)

// swagger:route POST /sleep Sleep Sleep
// Sleeps for a configured period of time.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: SleepPostResponseOK

// swagger:response SleepPostResponseOK
type SleepPostResponseOK struct {
	// in:body
	Timers data.Timers `json:"timers"`
}

// swagger:parameters Sleep
type SleepPostParams struct {
	// in:header
	CorrelationId string `json:"Correlation-Id"`

	// in:body
	SleepDuration time.Duration `json:"sleep_duration"`
}
