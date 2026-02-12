package swagger

import "github.com/antonio-alexander/go-blog-cache/internal/data"

// swagger:route PUT /employees Employee CreateEmployee
// Creates an employee.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: EmployeePutResponseOk

// swagger:response EmployeePutResponseOk
type EmployeePutResponseOk struct {
	// in:body
	Employee data.Employee `json:"employee"`
}

// swagger:parameters CreateEmployee
type EmployeePutParams struct {
	// in:body
	EmployeePartial data.EmployeePartial `json:"employee_partial"`

	// in:header
	CorrelationId string `json:"Correlation-Id"`
}
