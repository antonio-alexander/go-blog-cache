package swagger

import "github.com/antonio-alexander/go-blog-cache/internal/data"

// swagger:route GET /employees/{emp_no} Employee ReadEmployee
// Reads an employee using its id.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: EmployeeGetResponseOk

// swagger:response EmployeeGetResponseOk
type EmployeeGetResponseOk struct {
	// in:body
	Employee data.Employee `json:"employee"`
}

// swagger:parameters ReadEmployee
type EmployeeGetParams struct {
	// in:path
	EmpNo string `json:"emp_no"`

	// in:header
	CorrelationId string `json:"Correlation-Id"`
}
