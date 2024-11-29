package swagger

import "github.com/antonio-alexander/go-blog-cache/internal/data"

// swagger:route POST /employees/{emp_no} Employee UpdateEmployee
// Updates an employee using its id.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: EmployeePostResponseOk

// swagger:response EmployeePostResponseOk
type EmployeePostResponseOk struct {
	// in:body
	Employee data.Employee `json:"employee"`
}

// swagger:parameters UpdateEmployee
type EmployeePostParams struct {
	// in:path
	EmpNo string `json:"emp_no"`

	// in:body
	EmployeePartial data.EmployeePartial `json:"employee_partial"`
}
