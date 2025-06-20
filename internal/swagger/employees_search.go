package swagger

import "github.com/antonio-alexander/go-blog-big-data/internal/data"

// swagger:route GET /employees/search Employee SearchEmployee
// Searches employees using search criteria.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: EmployeeSearchResponseOk

// swagger:response EmployeeSearchResponseOk
type EmployeeSearchGetResponseOk struct {
	// in:body
	Employees []data.Employee `json:"employees"`
}

// swagger:parameters SearchEmployee
type EmployeeSearchGetParams struct {
	// in:header
	CorrelationId string `json:"Correlation-Id"`

	// in:query
	data.EmployeeSearch
}
