package swagger

// swagger:route DELETE /employees/{emp_no} Employee DeleteEmployee
// Deletes an employee using its id.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// responses:
//   200: EmployeeDeleteResponseOk

// swagger:response EmployeeDeleteResponseOk
type EmployeeDeleteResponseOk struct {
	//
}

// swagger:parameters DeleteEmployee
type EmployeeDeleteParams struct {
	// in:path
	EmpNo string `json:"emp_no"`
}
