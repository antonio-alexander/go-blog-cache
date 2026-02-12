package swagger

import (
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
	Sleep data.Sleep `json:"sleep"`
}

// swagger:parameters Sleep
type SleepPostParams struct {
	// in:header
	CorrelationId string `json:"Correlation-Id"`

	// in:body
	Sleep data.Sleep `json:"sleep"`
}
